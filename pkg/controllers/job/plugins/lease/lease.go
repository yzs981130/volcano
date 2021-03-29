package lease

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pkg.yezhisheng.me/volcano/pkg/apis/batch/v1alpha1"
	pluginsinterface "pkg.yezhisheng.me/volcano/pkg/controllers/job/plugins/interface"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/metrics"
	"strings"
	"sync"
	"time"
)

var leaseInstance *leasePlugin
var once sync.Once

type leasePlugin struct {
	// Arguments given for the plugin
	pluginArguments []string

	client pluginsinterface.PluginClientset
}

func (lp *leasePlugin) Name() string {
	return "lease"
}

func (lp *leasePlugin) OnPodCreate(pod *v1.Pod, job *v1alpha1.Job) error {
	return nil
}

func (lp *leasePlugin) OnJobAdd(job *v1alpha1.Job) error {
	// filter normal job
	if job.Spec.TotalLeaseJobCnt == 0 {
		return nil
	}

	// every job name should be "whateverJobName-1"
	job.Spec.JobGroupName = job.Name[:strings.LastIndex(job.Name, "-")]

	// if first sub job
	if job.Spec.CurrentLeaseJobCnt == 1 {
		job.Spec.JobGroupCreationTimeStamp = &job.CreationTimestamp
	}

	return nil
}

func (lp *leasePlugin) OnJobDelete(job *v1alpha1.Job) error {
	// TODO: set ttlSeconds in a proper time (checkpoint time)
	// when job finishes, get job index and total cnt

	// filter normal job
	if job.Spec.TotalLeaseJobCnt == 0 {
		return nil
	}
	// if last sub job
	if job.Spec.TotalLeaseJobCnt == job.Spec.CurrentLeaseJobCnt {
		metrics.UpdateJobTotalDuration(job.Name, metrics.Duration(job.Spec.JobGroupCreationTimeStamp.Time))
		return nil
	}

	newJob := getSuccessorJob(job)
	_, err := lp.client.VcClients.BatchV1alpha1().Jobs(newJob.Namespace).Create(context.TODO(), newJob, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		return nil
	}

	return err
}

func (lp *leasePlugin) OnJobUpdate(job *v1alpha1.Job) error {
	return nil
}

// New creates lease plugin
func New(client pluginsinterface.PluginClientset, arguments []string) pluginsinterface.PluginInterface {
	once.Do(func() {
		leaseInstance = &leasePlugin{
			pluginArguments: arguments,
			client:          client,
		}
	})
	return leaseInstance
}

func generateSuccessorJobNameWithIndex(job *v1alpha1.Job, index int32) string {
	return job.Spec.JobGroupName + "-" + string(index)
}

// should create a complete different new job, because by now the old job has not been deleted
// the old job should be deleted by other component by setting ttl
func getSuccessorJob(originJob *v1alpha1.Job) *v1alpha1.Job {
	// create job with currentLeaseCnt++ and resume command
	newJob := &v1alpha1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateSuccessorJobNameWithIndex(originJob, originJob.Spec.CurrentLeaseJobCnt+1),
			Namespace: originJob.Namespace,
			// should set OwnerReferences/Annotations/Labels?
			OwnerReferences: originJob.OwnerReferences,
		},
		Spec: *originJob.Spec.DeepCopy(),
	}
	newJob.Spec.CurrentLeaseJobCnt++
	newJob.Spec.CurrentJobScheduledTimeStamp = nil
	newJob.Spec.FormerJobDeletionTimeStamp = &metav1.Time{Time: time.Now()}
	return newJob
}
