package lease

import (
	"context"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/util"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/util/consolidate"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/util/k8s"
	schedulerutil "pkg.yezhisheng.me/volcano/pkg/scheduler/util"
	"sync"
	"time"

	"pkg.yezhisheng.me/volcano/pkg/scheduler/api"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/framework"
)

var once sync.Once
var leaseInstance framework.Plugin

// PluginName indicates name of volcano scheduler plugin.
const PluginName = "lease"

const leaseTerm = 900

func jobKey(job *api.JobInfo) string {
	return job.PodGroup.Spec.JobGroupName
}

type leasePlugin struct {
	// Arguments given for the plugin
	pluginArguments framework.Arguments
	// Key is Job ID
	jobAttrs map[string]*jobAttr

	queueOpts         map[api.QueueID]*queueAttr
	totalFreeResource *api.Resource
	totalResource     *api.Resource

	firstRun bool
	mc       *MetricsCollector
}

type queueAttr struct {
	queueID    api.QueueID
	name       string
	quotaRatio float64

	uDivideW             float64
	utilizedQuotaPercent float64

	userQuota float64
	newQuota  float64
	utilized  float64
	weighted  float64
	deserved  float64
	fairness  float64

	// only use allocated for ssn.overused
	allocated *api.Resource
}

type jobAttr struct {
	utilized float64
	deserved float64
	fairness float64
}

// New return priority plugin
func New(arguments framework.Arguments) framework.Plugin {
	once.Do(func() {
		leaseInstance = &leasePlugin{
			pluginArguments:   arguments,
			jobAttrs:          map[string]*jobAttr{},
			queueOpts:         map[api.QueueID]*queueAttr{},
			totalFreeResource: api.EmptyResource(),
			totalResource:     api.EmptyResource(),

			firstRun: true,
		}
	})
	return leaseInstance
}

func (lp *leasePlugin) Name() string {
	return PluginName
}

func (lp *leasePlugin) OnSessionOpen(ssn *framework.Session) {
	// if first run, init metrics collector
	if lp.firstRun {
		// mc.RunOnce is called according to session, should be set to scheduler.period
		lp.mc = NewMetricsCollector(time.Second, ssn, lp)
	}

	for _, n := range ssn.Nodes {
		lp.totalFreeResource.Add(n.Allocatable)
		lp.totalResource.Add(n.Capability)
	}

	klog.V(4).Infof("The total resource is <%v>, total free resource is <%v>", lp.totalResource, lp.totalFreeResource)

	// build up scheduling information:
	totalFreeGPU := lp.totalFreeResource.ScalarResources[api.GPUResourceName]
	totalGPU := lp.totalResource.ScalarResources[api.GPUResourceName]
	// all pending job map
	pendingJobs := map[api.JobID]struct{}{}
	// user->pending_job
	userJobMap := map[api.QueueID]*schedulerutil.PriorityQueue{}

	// set cluster size in mc
	lp.mc.SetClusterSize(totalGPU)

	// call metrics collector
	lp.mc.RunOnce()

	// Job selection func
	for _, job := range ssn.Jobs {
		if !isFinishedJob(job) {
			// get job utilized and deserved from mc
			jobStat, err := lp.mc.GetJobStatistics(jobKey(job))
			if err != nil {
				// not found in mc, will never happen
				lp.jobAttrs[jobKey(job)] = &jobAttr{
					utilized: 0,
					deserved: 0,
					fairness: 1,
				}
				continue
			}
			// store in lp.jobAttrs
			attr := &jobAttr{
				utilized: jobStat.Utilized,
				deserved: jobStat.Deserved,
			}
			lp.updateJobFairness(attr)
			lp.jobAttrs[jobKey(job)] = attr
		} else {
			// delete job from active job
			lp.mc.Delete(job)
		}
	}

	// use lp.jobAttrs[lv.UID].fairness for job selection
	jobOrderFn := func(l, r interface{}) int {
		lv := l.(*api.JobInfo)
		rv := r.(*api.JobInfo)

		klog.V(4).Infof("Lease JobOrderFn: <%v/%v> priority: %f, <%v/%v> priority: %f",
			lv.Namespace, lv.Name, lp.jobAttrs[jobKey(lv)].fairness, rv.Namespace, rv.Name, lp.jobAttrs[jobKey(rv)].fairness)

		if lp.jobAttrs[jobKey(lv)].fairness > lp.jobAttrs[jobKey(rv)].fairness {
			return -1
		}

		if lp.jobAttrs[jobKey(lv)].fairness < lp.jobAttrs[jobKey(rv)].fairness {
			return 1
		}

		return 0
	}

	ssn.AddJobOrderFn(lp.Name(), jobOrderFn)

	pl := util.NewPodLister(ssn)
	pods, _ := pl.List(labels.NewSelector())
	nodeMap, nodeSlice := util.GenerateNodeMapAndSlice(ssn.Nodes)
	handle := k8s.NewFrameworkHandle(pods, nodeSlice)

	// Register event handlers to update task info in PodLister & nodeMap
	ssn.AddEventHandler(&framework.EventHandler{
		AllocateFunc: func(event *framework.Event) {
			pod := pl.UpdateTask(event.Task, event.Task.NodeName)

			nodeName := event.Task.NodeName
			node, found := nodeMap[nodeName]
			if !found {
				klog.Warningf("lease, update pod %s/%s allocate to NOT EXIST node [%s]", pod.Namespace, pod.Name, nodeName)
			} else {
				node.AddPod(pod)
				klog.V(4).Infof("lease, update pod %s/%s allocate to node [%s]", pod.Namespace, pod.Name, nodeName)
			}
			// update queue information
			job := ssn.Jobs[event.Task.Job]
			qAttr := lp.queueOpts[job.Queue]
			qAttr.allocated.Add(event.Task.Resreq)
		},
		DeallocateFunc: func(event *framework.Event) {
			pod := pl.UpdateTask(event.Task, "")

			nodeName := event.Task.NodeName
			node, found := nodeMap[nodeName]
			if !found {
				klog.Warningf("lease, update pod %s/%s allocate from NOT EXIST node [%s]", pod.Namespace, pod.Name, nodeName)
			} else {
				err := node.RemovePod(pod)
				if err != nil {
					klog.Errorf("Failed to update pod %s/%s and deallocate from node [%s]: %s", pod.Namespace, pod.Name, nodeName, err.Error())
				} else {
					klog.V(4).Infof("node order, update pod %s/%s deallocate from node [%s]", pod.Namespace, pod.Name, nodeName)
				}
			}
			// We should only consider user quota usage in every task deallocate
			// Task de-allocation should only happen under session discard
			// Job fairness should never be updated because there is no job allocation
			job := ssn.Jobs[event.Task.Job]
			qAttr := lp.queueOpts[job.Queue]
			qAttr.allocated.Sub(event.Task.Resreq)
		},
		// We need AllocateJobFunc and do the following update to queueAttr and jobAttr in plugin
		// For simplicity, check if the task allocation is the last allocation
		// In other words, check if the job is allocated
		// if true, the job is allocated, update job fairness and queue fairness (like dispatchJob)
		AllocateJobFunc: func(event *framework.Event) {
			job := ssn.Jobs[event.Task.Job]
			jAttr, qAttr := lp.jobAttrs[jobKey(job)], lp.queueOpts[job.Queue]
			qAttr.utilized += getJobGPUReq(job) * leaseTerm
			jAttr.utilized += getJobGPUReq(job) * leaseTerm
			lp.updateUserFairness(qAttr)
			lp.updateJobFairness(jAttr)
		},
		DeallocateJobFunc: func(event *framework.Event) {
			job := ssn.Jobs[event.Task.Job]
			jAttr, qAttr := lp.jobAttrs[jobKey(job)], lp.queueOpts[job.Queue]
			qAttr.utilized -= getJobGPUReq(job) * leaseTerm
			jAttr.utilized -= getJobGPUReq(job) * leaseTerm
			lp.updateUserFairness(qAttr)
			lp.updateJobFairness(jAttr)
		},
	})

	nodeOrderFn := func(task *api.TaskInfo, node *api.NodeInfo) (float64, error) {
		p, _ := consolidate.New(nil, handle, ssn)
		cp := p.(*consolidate.Consolidate)
		score, status := cp.Score(context.TODO(), nil, task.Pod, node.Name)
		if !status.IsSuccess() {
			klog.Warningf("Calculate Node Affinity Priority Failed because of Error: %v", status.AsError())
			return 0, status.AsError()
		}
		nodeScore := float64(score)
		klog.V(4).Infof("Total Score for task %s/%s on node %s is: %f", task.Namespace, task.Name, node.Name, nodeScore)
		return nodeScore, nil
	}

	ssn.AddNodeOrderFn(lp.Name(), nodeOrderFn)

	// Finished: build up queue attr like plugin proportion
	// We need calculate deserved/allocated of every queue to utilize user-level fairness
	// Get job information and resource information directly from ssn.Jobs and ssn.Nodes to build queue
	// After building up queue, use ssn.AddQueueOrderFn to utilize user-level fairness

	// build up queues
	for _, queue := range ssn.Queues {
		// get queue metrics from mc
		userStat, err := lp.mc.GetUserStatistics(queue.UID)
		if err != nil {
			// should never happen
			continue
		}
		attr := &queueAttr{
			queueID:    queue.UID,
			name:       queue.Name,
			quotaRatio: queue.QuotaRatio,
			// for overused
			allocated: api.EmptyResource(),
			// FIXED: store & calculate user fairness related historical information
			// get utilized/weighted/deserved from metrics
			utilized: userStat.utilized,
			weighted: userStat.weighted,
			deserved: userStat.deserved,
		}
		lp.updateUserFairness(attr)
		lp.queueOpts[queue.UID] = attr
	}

	for _, job := range ssn.Jobs {
		// add all pending job
		if isPendingJob(job) {
			pendingJobs[job.UID] = struct{}{}
			userJobMap[job.Queue].Push(job)
		}

		// update queue information: allocated & request
		attr := lp.queueOpts[job.Queue]
		for status, tasks := range job.TaskStatusIndex {
			if api.AllocatedStatus(status) {
				for _, t := range tasks {
					attr.allocated.Add(t.Resreq)
				}
			}
		}
	}

	// Finished: decide queue quota
	// We need simulate job scheduling and divide free capacity to different queue
	// Use ssn.AddOverusedFn to support quota settings of each queue
	// Should be similar with actual scheduling

	// simulate scheduling
	for {
		// if no capacity || no pending jobs || no user
		if totalFreeGPU == 0 || len(pendingJobs) == 0 || len(userJobMap) == 0 {
			break
		}
		// select user
		user := lp.selectUser()
		// check if user has pending job
		for {
			if userJobMap[user].Empty() {
				break
			}
			// select job from user pending job pq
			job := userJobMap[user].Pop().(*api.JobInfo)
			jobGPUReq := getJobGPUReq(job)
			if totalFreeGPU < jobGPUReq {
				continue
			}
			lp.queueOpts[user].newQuota += jobGPUReq
			totalFreeGPU -= jobGPUReq
			// update user related information
			lp.queueOpts[user].utilized += jobGPUReq * leaseTerm
			lp.queueOpts[user].uDivideW = lp.queueOpts[user].utilized / lp.queueOpts[user].weighted
			lp.queueOpts[user].utilizedQuotaPercent = lp.queueOpts[user].newQuota / lp.queueOpts[user].userQuota
			break
		}
		// del user if no pending job
		if userJobMap[user].Empty() {
			delete(userJobMap, user)
		}
	}

	ssn.AddOverusedFn(lp.Name(), func(obj interface{}) bool {
		queue := obj.(*api.QueueInfo)
		attr := lp.queueOpts[queue.UID]

		var allocatedGPU float64
		if c, exist := attr.allocated.ScalarResources[api.GPUResourceName]; exist {
			allocatedGPU = c
		}

		overused := allocatedGPU > attr.newQuota

		if overused {
			klog.V(3).Infof("Queue <%v>: newQuota <%v>, allocated <%v>",
				queue.Name, attr.newQuota, attr.allocated)
		}

		return overused
	})

}

func (lp *leasePlugin) OnSessionClose(ssn *framework.Session) {
	// clear jobAttr
	lp.jobAttrs = map[string]*jobAttr{}
	// clear queueOpts
	lp.queueOpts = map[api.QueueID]*queueAttr{}
	lp.totalFreeResource = api.EmptyResource()
	lp.totalResource = api.EmptyResource()

	lp.firstRun = false
}

// check if job need scheduling
func isPendingJob(job *api.JobInfo) bool {
	for status := range job.TaskStatusIndex {
		if status != api.Pending {
			return false
		}
	}
	return true
}

// to skip finished job in ssn.Jobs
// TODO: check
func isFinishedJob(job *api.JobInfo) bool {
	for status := range job.TaskStatusIndex {
		if !(status == api.Succeeded || status == api.Failed) {
			return false
		}
	}
	return true
}

func getJobGPUReq(job *api.JobInfo) float64 {
	var cnt float64
	for _, task := range job.Tasks {
		if c, exist := task.Resreq.ScalarResources[api.GPUResourceName]; exist {
			cnt += c
		}
	}
	return cnt
}

func (lp *leasePlugin) selectUser() api.QueueID {
	var user api.QueueID
	for user = range lp.queueOpts {
		break
	}
	// least utilized quota percent & least U_divide_W
	leastUtilizedQuotaPercentUser, leastUDivideWUser := user, user
	for _, u := range lp.queueOpts {
		if u.utilizedQuotaPercent < lp.queueOpts[leastUtilizedQuotaPercentUser].utilizedQuotaPercent {
			leastUtilizedQuotaPercentUser = u.queueID
		}
		if u.uDivideW < lp.queueOpts[leastUDivideWUser].uDivideW {
			leastUDivideWUser = u.queueID
		}
	}
	if lp.queueOpts[leastUtilizedQuotaPercentUser].utilizedQuotaPercent < 0.2 {
		return leastUtilizedQuotaPercentUser
	}
	return leastUDivideWUser
}

func (lp *leasePlugin) updateUserFairness(attr *queueAttr) {
	if attr.weighted == 0 {
		attr.uDivideW = 1
	} else {
		attr.uDivideW = attr.utilized / attr.weighted
	}

	if attr.deserved == 0 {
		attr.fairness = 1
	} else {
		attr.fairness = attr.utilized / attr.deserved
	}
}

func (lp *leasePlugin) updateJobFairness(attr *jobAttr) {
	if attr.deserved == 0 {
		attr.fairness = 1
	} else {
		attr.fairness = attr.utilized / attr.deserved
	}
}
