package lease

import (
	"context"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
	"volcano.sh/volcano/pkg/scheduler/plugins/util"
	"volcano.sh/volcano/pkg/scheduler/plugins/util/consolidate"
	"volcano.sh/volcano/pkg/scheduler/plugins/util/k8s"
	schedulerutil "volcano.sh/volcano/pkg/scheduler/util"

	"volcano.sh/volcano/pkg/scheduler/api"
	"volcano.sh/volcano/pkg/scheduler/framework"
)

// PluginName indicates name of volcano scheduler plugin.
const PluginName = "lease"

const leastTerm = 900

type leasePlugin struct {
	// Arguments given for the plugin
	pluginArguments framework.Arguments
	// Key is Job ID
	jobAttrs map[api.JobID]*leaseAttr

	queueOpts     map[api.QueueID]*queueAttr
	totalResource *api.Resource
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

type leaseAttr struct {
	utilized float64
	deserved float64
	fairness float64
}

// New return priority plugin
func New(arguments framework.Arguments) framework.Plugin {
	return &leasePlugin{
		pluginArguments: arguments,
		jobAttrs:        map[api.JobID]*leaseAttr{},
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
			attr := lp.queueOpts[job.Queue]
			attr.allocated.Add(event.Task.Resreq)
			// TODO: should update user fairness?
			attr.utilized += getJobGPUReq(job) * leastTerm
			lp.updateUserFairness(attr)
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
			job := ssn.Jobs[event.Task.Job]
			attr := lp.queueOpts[job.Queue]
			attr.allocated.Sub(event.Task.Resreq)
			// TODO: should update user fairness?
			attr.utilized -= getJobGPUReq(job) * leastTerm
			lp.updateUserFairness(attr)
		},
	})

	nodeOrderFn := func(task *api.TaskInfo, node *api.NodeInfo) (float64, error) {
		p, _ := consolidate.New(nil, handle)
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

	// TODO: build up queue attr like plugin proportion
	// We need calculate deserved/allocated of every queue to utilize user-level fairness
	// Get job information and resource information directly from ssn.Jobs and ssn.Nodes to build queue
	// After building up queue, use ssn.AddQueueOrderFn to utilize user-level fairness

	for _, n := range ssn.Nodes {
		lp.totalResource.Add(n.Allocatable)
	}

	klog.V(4).Infof("The total resource is <%v>", lp.totalResource)

	// build up queues
	for _, queue := range ssn.Queues {
		// if not cached
		if _, found := lp.queueOpts[queue.UID]; !found {
			attr := &queueAttr{
				queueID:    queue.UID,
				name:       queue.Name,
				quotaRatio: queue.QuotaRatio,
				// for overused
				allocated: api.EmptyResource(),
				// TODO: store & calculate user fairness related historical information

			}
			lp.queueOpts[queue.UID] = attr
		}
	}

	// build up scheduling information:
	totalFreeGPU := lp.totalResource.ScalarResources[api.GPUResourceName]
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

	// TODO: decide queue quota
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
			lp.queueOpts[user].utilized += jobGPUReq * leastTerm
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
	lp.jobAttrs = map[api.JobID]*leaseAttr{}
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
