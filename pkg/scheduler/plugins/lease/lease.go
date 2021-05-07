package lease

import (
	"context"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/util"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/util/consolidate"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/util/k8s"
	schedulerutil "pkg.yezhisheng.me/volcano/pkg/scheduler/util"

	"pkg.yezhisheng.me/volcano/pkg/scheduler/api"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/framework"
)

// PluginName indicates name of volcano scheduler plugin.
const PluginName = "lease"

const leaseTerm = 900

type leasePlugin struct {
	// Arguments given for the plugin
	pluginArguments framework.Arguments
	// Key is Job ID
	jobAttrs map[api.JobID]*jobAttr

	queueOpts     map[api.QueueID]*queueAttr
	totalResource *api.Resource
	idleResource *api.Resource
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

	allocated *api.Resource
}

type jobAttr struct {
	utilized float64
	deserved float64
	fairness float64
}

// New return priority plugin
func New(arguments framework.Arguments) framework.Plugin {
	return &leasePlugin{
		pluginArguments: arguments,
		jobAttrs:        map[api.JobID]*jobAttr{},
		queueOpts:       map[api.QueueID]*queueAttr{},
		totalResource:   api.EmptyResource(),
	}
}

func (lp *leasePlugin) Name() string {
	return PluginName
}

func (lp *leasePlugin) OnSessionOpen(ssn *framework.Session) {
	// Job selection func
	// TODO: update jobAttrs.fairness
	for _, job := range ssn.Jobs {
		if !isFinishedJob(job) {
			if _, found := lp.jobAttrs[job.UID]; !found {
				// TODO: get job utilized & deserved from job.PodGroup
				if job.PodGroup.Annotations == nil {
					klog.Errorf("job %s has no annotation in its podgroup, skipping", job.Name)
					continue
				}
				attr := &jobAttr{
					utilized: string2float(job.PodGroup.Annotations[PodGroupStatisticsAnnoKey.Utilized]),
					deserved: string2float(job.PodGroup.Annotations[PodGroupStatisticsAnnoKey.Deserved]),
				}
				lp.updateJobFairness(attr)
				lp.jobAttrs[job.UID] = attr
			}
		}
	}

	// use lp.jobAttrs[lv.UID].fairness for job selection
	jobOrderFn := func(l, r interface{}) int {
		lv := l.(*api.JobInfo)
		rv := r.(*api.JobInfo)

		klog.V(4).Infof("Lease JobOrderFn: <%v/%v> priority: %f, <%v/%v> priority: %f",
			lv.Namespace, lv.Name, lp.jobAttrs[lv.UID].fairness, rv.Namespace, rv.Name, lp.jobAttrs[rv.UID].fairness)

		if lp.jobAttrs[lv.UID].fairness > lp.jobAttrs[rv.UID].fairness {
			return -1
		}

		if lp.jobAttrs[lv.UID].fairness < lp.jobAttrs[rv.UID].fairness {
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
			jAttr, qAttr := lp.jobAttrs[job.UID], lp.queueOpts[job.Queue]
			qAttr.utilized += getJobGPUReq(job) * leaseTerm
			jAttr.utilized += getJobGPUReq(job) * leaseTerm
			lp.updateUserFairness(qAttr)
			lp.updateJobFairness(jAttr)
		},
		DeallocateJobFunc: func(event *framework.Event) {
			job := ssn.Jobs[event.Task.Job]
			jAttr, qAttr := lp.jobAttrs[job.UID], lp.queueOpts[job.Queue]
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

	for _, n := range ssn.Nodes {
		lp.totalResource.Add(n.Allocatable)
		lp.idleResource.Add(n.Idle)
	}

	klog.V(4).Infof("The total resource is <%v>", lp.totalResource)
	klog.V(4).Infof("The idle resource is <%v>", lp.idleResource)

	totalGPUcnt := lp.totalResource.ScalarResources[api.GPUResourceName]
	if totalGPUcnt == 0 {
		klog.V(2).Infof("Total GPU is 0, may cause panic later")
	}

	// build up queues
	for _, queue := range ssn.Queues {
		// if not cached
		if _, found := lp.queueOpts[queue.UID]; !found {
			if queue.Queue.Annotations == nil {
				klog.Errorf("user %s has no annotation in its queue, skipping", queue.Name)
				continue
			}
			attr := &queueAttr{
				queueID:    queue.UID,
				name:       queue.Name,
				quotaRatio: queue.QuotaRatio,
				// for overused
				allocated: api.EmptyResource(),
				// TODO: store & calculate user fairness related historical information
				utilized: string2float(queue.Queue.Annotations[UserStatisticsAnnoKey.Utilized]),
				weighted: string2float(queue.Queue.Annotations[UserStatisticsAnnoKey.Weighted]),
				// TODO: calculate userquota by quotaRatio or get from spec directly, use quotaRatio for now
				userQuota: lp.totalResource.ScalarResources[api.GPUResourceName] * queue.QuotaRatio,
			}
			lp.queueOpts[queue.UID] = attr
		}
	}

	// build up scheduling information:
	totalFreeGPU := lp.idleResource.ScalarResources[api.GPUResourceName]
	// all pending job map
	pendingJobs := map[api.JobID]struct{}{}
	// user->pending_job
	userJobMap := map[api.QueueID]*schedulerutil.PriorityQueue{}

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
	lp.jobAttrs = map[api.JobID]*jobAttr{}
	// clear queueOpts
	lp.queueOpts = map[api.QueueID]*queueAttr{}
	lp.totalResource = api.EmptyResource()
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
	attr.uDivideW = attr.utilized / attr.weighted
}

func (lp *leasePlugin) updateJobFairness(attr *jobAttr) {
	if attr.deserved == 0 {
		attr.fairness = 1
	} else {
		attr.fairness = attr.utilized / attr.deserved
	}
}
