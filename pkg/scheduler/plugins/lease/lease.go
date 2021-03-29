package lease

import (
	"context"
	"math"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
	"volcano.sh/volcano/pkg/scheduler/api"
	"volcano.sh/volcano/pkg/scheduler/framework"
	"volcano.sh/volcano/pkg/scheduler/plugins/util"
	"volcano.sh/volcano/pkg/scheduler/plugins/util/consolidate"
	"volcano.sh/volcano/pkg/scheduler/plugins/util/k8s"
	compareutil "volcano.sh/volcano/pkg/scheduler/util"
)

// PluginName indicates name of volcano scheduler plugin.
const PluginName = "lease"

const leaseTerm = 900
const totalResourceNum = 80
const TolerateRatio = 4

type leasePlugin struct {
	// Arguments given for the plugin
	pluginArguments framework.Arguments
	// Key is Job ID
	jobAttrs map[api.JobID]*jobAttr

	queueOpts     map[api.QueueID]*queueAttr
	totalResource *api.Resource
	solver        MIPSolver
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

const (
	acceptedSLO   = 1
	unacceptedSLO = 3
	bestEffort    = 2
)

var curLeaseIndex = 0
var GuaranteeResource = 0
var SpotResource = totalResourceNum

type jobAttr struct {
	cacheSolution          []int
	emergence              int
	sloPrioirity           int
	occupyLease            int
	requireMipOptimization int
	forceGuarantee         int
	stillRequireLease      int
	remainingTime          int
	ddlTime                int
	submitTime             int
	reqGPU       int
	duration     int
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


func (lp *leasePlugin) SummaryCluster(ssn *framework.Session) {
	all_running_resource, all_running_numer := 0, 0
	all_pending_resource, all_pending_number := 0, 0
	
	for _, job := range ssn.Jobs {
		if !isFinished(job) {
			if isRunning(job) {
				all_running_resource += int(getJobGPUReq(job))
			} else {
				all_pending_resoure += int(getJobGPUReq(job))
			}
		}
	}
}


func (lp *leasePlugin) Name() string {
	return PluginName
}

func max(arr []int) int {
	maxval := arr[0]
	for _, value := range arr {
		if value > maxval {
			maxval = value
		}
	}
	return maxval
}

func (lp *leasePlugin) ProcessUnacceptedJobs(job *api.JobInfo, requireResourceList, requireLeaseList, maximumLeaseList, globalCacheSolution []int) bool {
	jAttr := lp.jobAttrs[job.UID]
	maxLen := len(requireLeaseList)
	infeasibleLease := requireLeaseList[maxLen-1]

	if requireLeaseList[maxLen-1] > maximumLeaseList[maxLen-1] {
		maximumLeaseList[maxLen-1] = requireLeaseList[maxLen-1]
	}
	left := maximumLeaseList[maxLen-1]
	right := len(globalCacheSolution) + jAttr.stillRequireLease + 1
	// feasible, _ := lp.solver.BatchFastCheckIfPackable(requireResourceList,
	// 	requireLeaseList,
	// 	maximumLeaseList,
	// 	globalCacheSolution,
	// 	totalResourceNum,
	// )
	for {
		if left >= right {
			break
		}
		maximumLeaseList[maxLen-1] = left + (right-left)/2
		resourceNumList := []int{totalResourceNum}
		maxInLease := max(maximumLeaseList)
		for j := 0; j < maxInLease; j++ {
			resourceNumList = append(resourceNumList, totalResourceNum)
		}
		feasible, _ := lp.solver.BatchFastCheckIfPackable(requireResourceList,
			requireLeaseList,
			maximumLeaseList,
			globalCacheSolution,
			resourceNumList,
		)
		if feasible {
			right = maximumLeaseList[maxLen-1]
		} else {
			left = maximumLeaseList[maxLen-1] + 1
		}
		maximumLeaseList[maxLen-1] = right
	}
	if jAttr.sloPrioirity == bestEffort {
		jAttr.requireMipOptimization = 0
		// TODO
	} else {
		if maximumLeaseList[-1] < infeasible_lease * TolerateRatio {
			jAttr.ddlTime = leaseTerm * (maximum_lease_list[-1] + curLeaseIndex) - jAttr.submitTime
			return true
		}
		jAttr.sloPrioirity = unacceptedSLO
		jAttr.requireMipOptimization = 0
	} // specified deadline time
	requireResourceList = requireResourceList[:maxLen-1]
	requireLeaseList = requireLeaseList[:maxLen-1]
	maximumLeaseList = maximumLeaseList[:maxLen-1]
	return false
}

func (lp *leasePlugin) OnSessionOpen(ssn *framework.Session) {
	if true {
		lp.FlushLeaseJobs(ssn)
		curLeaseIndex += 1
	}
	lp.FlushSpotGuaranteeResource(ssn)
	lp.FlushRunningJobs(ssn)
	lp.FlushEventJobs(ssn)
	lp.FlushPendingJobs(ssn)


	// use lp.jobAttrs[lv.UID].fairness for job selection
	jobOrderFn := func(l, r interface{}) int {
		lv := l.(*api.JobInfo)
		rv := r.(*api.JobInfo)

		klog.V(4).Infof("Lease JobOrderFn: <%v/%v> priority: %f, <%v/%v> priority: %f",
			lv.Namespace, lv.Name, lp.jobAttrs[lv.UID].emergence, rv.Namespace, rv.Name, lp.jobAttrs[rv.UID].emergence)
		lattr, rattr := lp.jobAttrs[lv.UID], lp.jobAttrs[rv.UID]
		lforceOccupy := lattr.occupyLease + lattr.forceGuarantee
		rforceOccupy := rattr.occupyLease + rattr.forceGuarantee
		if lforceOccupy == lforceOccupy {
			if lforceOccupy == 0 {
				if lattr.sloPrioirity == rattr.sloPrioirity {
					return compareutil.CompareInt(lattr.emergence, rattr.emergence)
				} else {
					return compareutil.CompareInt(lattr.sloPrioirity, rattr.sloPrioirity)
				}
			} else {
				return compareutil.CompareInt(int(getJobGPUReq(lv)), int(getJobGPUReq(rv)))
			}
		}
		return compareutil.CompareInt(lforceOccupy, rforceOccupy)
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

			jAttr := lp.jobAttrs[job.UID]
			jAttr.remainingTime -= leaseTerm
			if jAttr.occupyLease == 1 && jAttr.sloPrioirity == acceptedSLO {
				jAttr.cacheSolution = jAttr.cacheSolution[1:]
			}
			jAttr.emergence = computeEmergence(jAttr.remainingTime, jAttr.reqGPU)
		},
		DeallocateJobFunc: func(event *framework.Event) {
			job := ssn.Jobs[event.Task.Job]
			jAttr := lp.jobAttrs[job.UID]
			jAttr.emergence = computeEmergence(jAttr.remainingTime, jAttr.reqGPU)
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
	}

	klog.V(4).Infof("The total resource is <%v>", lp.totalResource)

	// build up queues
	for _, queue := range ssn.Queues {
		// if not cached
		if _, found := lp.queueOpts[queue.UID]; !found {
			attr := &queueAttr{
				queueID: queue.UID,
				name:    queue.Name,
				// for overused
				allocated: api.EmptyResource(),
				// TODO: store & calculate user fairness related historical information

			}
			lp.queueOpts[queue.UID] = attr
		}
	}

	// build up scheduling information:

	// Finished: decide queue quota
	// We need simulate job scheduling and divide free capacity to different queue
	// Use ssn.AddOverusedFn to support quota settings of each queue
	// Should be similar with actual scheduling

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
	for _, job := range ssn.Jobs {
		if isFinishedJob(job) {
			_, found := lp.queueOpts[queue.UID]
			if found {
				delete(lp.jobAttrs, job.UID)
			}
		}
	}
}

func isRunningJob(job *api.JobInfo) bool {
	for _, status := range job.TaskStatusIndex {
		if status != api.RUNNING {
			return false
	}
	return false
}

// check if job need scheduling
func isPendingJob(job *api.JobInfo) bool {
	for _, status := range job.TaskStatusIndex {
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

func computeEmergence(remainingTime int, reqResource int) int {
	return -1 * remainingTime * reqResource
}

func computeMaximumLease(expectMaximumEndTime int, leaseTerm int, curLeaseIndex int) int {
	return int(math.Floor(float64(expectMaximumEndTime)/float64(leaseTerm))) - curLeaseIndex
}

