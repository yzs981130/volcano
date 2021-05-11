package lease

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"pkg.yezhisheng.me/volcano/pkg/apis/scheduling"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/api"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/framework"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/util"
)

type Action struct {
	isBlock bool
}

const (
	blockScheduling        = "blockScheduling"
	defaultBlockScheduling = false
)

func New() *Action {
	return &Action{
		isBlock: defaultBlockScheduling,
	}
}

func (la *Action) Name() string {
	return "lease"
}

func (la *Action) Initialize() {}

func (la *Action) Execute(ssn *framework.Session) {
	klog.V(3).Infof("Enter Lease ...")
	defer klog.V(3).Infof("Leaving Lease ...")

	la.getIsBlock(ssn)

	// the conventional allocation for pod may have many stages
	// 1. pick a namespace named N (using ssn.NamespaceOrderFn)
	// 2. pick a queue named Q from N (using ssn.QueueOrderFn)
	// 3. pick a job named J from Q (using ssn.JobOrderFn)
	// 4. pick a task T from J (using ssn.TaskOrderFn)
	// 5. use predicateFn to filter out node that T can not be allocated on.
	// 6. use ssn.NodeOrderFn to judge the best node and assign it to T

	// We will have only one namespace and multi queues for multi VCs(users)
	// Every queue will have its deserved/allocated
	// Even though we know that there is always single namespace
	// We pretend that there might be in the future, as if someone will use it
	namespaces := util.NewPriorityQueue(ssn.NamespaceOrderFn)

	// jobsMap is map[api.NamespaceName]map[api.QueueID]PriorityQueue(*api.JobInfo)
	// used to find job with highest priority in given queue and namespace
	// add job into jobsMap & queueMap
	jobsMap := map[api.NamespaceName]map[api.QueueID]*util.PriorityQueue{}

	var renewingJobList []*api.JobInfo
	renewingJobMap := make(map[*api.JobInfo]struct{})

	for _, job := range ssn.Jobs {
		if job.PodGroup.Status.Phase == scheduling.PodGroupPending {
			continue
		}
		if vr := ssn.JobValid(job); vr != nil && !vr.Pass {
			klog.V(4).Infof("Job <%s/%s> Queue <%s> skip allocate, reason: %v, message %v", job.Namespace, job.Name, job.Queue, vr.Reason, vr.Message)
			continue
		}

		if _, found := ssn.Queues[job.Queue]; !found {
			klog.Warningf("Skip adding Job <%s/%s> because its queue %s is not found",
				job.Namespace, job.Name, job.Queue)
			continue
		}

		namespace := api.NamespaceName(job.Namespace)
		queueMap, found := jobsMap[namespace]
		if !found {
			namespaces.Push(namespace)

			queueMap = make(map[api.QueueID]*util.PriorityQueue)
			jobsMap[namespace] = queueMap
		}

		jobs, found := queueMap[job.Queue]
		if !found {
			jobs = util.NewPriorityQueue(ssn.JobOrderFn)
			queueMap[job.Queue] = jobs
		}

		klog.V(4).Infof("Added Job <%s/%s> into Queue <%s>", job.Namespace, job.Name, job.Queue)
		jobs.Push(job)

		// store all lease renewing job
		// use clone to avoid affecting latter status
		if isJobRenewing(job) {
			renewingJobList = append(renewingJobList, job.Clone())
			renewingJobMap[job] = struct{}{}
		}
	}

	klog.V(3).Infof("Try to allocate resource to %d Namespaces", len(jobsMap))

	allNodes := util.GetNodeList(ssn.Nodes)

	predicateFn := func(task *api.TaskInfo, node *api.NodeInfo) error {
		// Check for Resource Predicate
		if !task.InitResreq.LessEqual(node.FutureIdle()) {
			return api.NewFitError(task, node, api.NodeResourceFitFailed)
		}

		return ssn.PredicateFn(task, node)
	}
	// To pick <namespace, queue> tuple for job, we choose to pick namespace firstly.
	// Because we believe that number of queues would less than namespaces in most case.
	// And, this action would make the resource usage among namespace balanced.
	for {
		if namespaces.Empty() {
			break
		}

		// pick namespace from namespaces PriorityQueue
		namespace := namespaces.Pop().(api.NamespaceName)

		queueInNamespace := jobsMap[namespace]

		// pick queue for given namespace
		//
		// This block use a algorithm with time complex O(n).
		// But at least PriorityQueue could not be used here,
		// because the allocation of job would change the priority of queue among all namespaces,
		// and the PriorityQueue have no ability to update priority for a special queue.

		// Finished: Do scheduling on queue(pending jobs in queue v.s. queue.quota)
		// select job by jobOrderFn, with resource restriction of queue.quota

		// Finished: extra logic for lease renewal job
		// Do scheduling twice with renewal job and without job
		// different logic in committing scheduling results according to job type

		// first scheduling begins
		// job: pending + lease renewing
		// resource: free capacity + lease renewing job

		// snapshot current resource and job status
		ssn.Snapshot()
		// set resource occupied by renewing job to free
		for _, job := range renewingJobList {
			reclaimJobResource(ssn, job)
			// update queue quota usage (in deallocateJobFn)
			ssn.DeallocateJob(job)
		}
		allocatedJobList := la.scheduling(ssn, queueInNamespace, allNodes, predicateFn, renewalFilter)
		// restore former resource and job status
		// update queue quota usage (in deallocateJobFn)
		for _, job := range allocatedJobList {
			ssn.DeallocateJob(job)
		}
		for _, job := range renewingJobList {
			ssn.AllocateJob(job)
		}
		// RestoreLastSnapshot only considers ssn.Node/Jobs/Queues information
		ssn.RestoreLastSnapshot()
		// first scheduling ends

		// second scheduling begins
		// job: pending job
		// resource: free capacity
		allocatedJobList2 := la.scheduling(ssn, queueInNamespace, allNodes, predicateFn, pendingFilter)
		// second scheduling ends

		// collect scheduling result
		stmt := framework.NewStatement(ssn)
		// for all lease renewing job, if in allocatedJobList, return true
		for _, job := range allocatedJobList {
			if _, exist := renewingJobMap[job]; exist {
				renewalSucceedJob(ssn, job)
				delete(renewingJobMap, job)
			}
		}
		for job := range renewingJobMap {
			renewalFailedJob(ssn, job)
		}
		// for all job in allocatedJobList2, allocate job task
		for _, job := range allocatedJobList2 {
			for _, task := range job.Tasks {
				// all task should have task.NodeName after pseudo scheduling
				if task.Status == api.Pipelined {
					_ = stmt.Pipeline(task, task.NodeName)
				} else if task.Status == api.Allocated {
					_ = stmt.Allocate(task, task.NodeName)
				}
			}
		}

		stmt.Commit()

		// don't support multi namespace, skip
		// namespaces.Push(namespace)
	}
}

func (la *Action) UnInitialize() {}

// return if job is renewing in this scheduling round
func isJobRenewing(job *api.JobInfo) bool {
	if job.PodGroup.Annotations == nil {
		return false
	}
	// if found result, return false
	if _, exist := job.PodGroup.Annotations[PodGroupRenewingResultAnnoKey]; exist {
		return false
	}
	// if have renewing annotation, return true
	if result, exist := job.PodGroup.Annotations[PodGroupRenewingAnnoKey]; exist && result == PodGroupRenewingOngoing {
		return true
	}
	return false
}

// return if job is pending in this scheduling round
func isJobPending(job *api.JobInfo) bool {
	for _, task := range job.Tasks {
		if task.Status != api.Pending {
			return false
		}
	}
	return true
}

func renewalFilter(j interface{}) bool {
	job := j.(*api.JobInfo)
	if !(isJobRenewing(job) || isJobPending(job)) {
		return false
	}
	return true
}

func pendingFilter(j interface{}) bool {
	job := j.(*api.JobInfo)
	return isJobPending(job)
}

func jobList2UserJobPQ(jobs []*api.JobInfo, lessFn api.LessFn, filterFns []func(j *api.JobInfo) bool) (result map[api.QueueID]*util.PriorityQueue) {
	result = make(map[api.QueueID]*util.PriorityQueue)
	for _, job := range jobs {
		queueID := job.Queue
		if _, exist := result[queueID]; !exist {
			m := util.NewPriorityQueue(lessFn)
			result[queueID] = m
		}
		result[queueID].Push(job)
	}
	return
}

func (la *Action) scheduling(ssn *framework.Session, userJobPQ map[api.QueueID]*util.PriorityQueue, candidateNodes []*api.NodeInfo, predicatedFn api.PredicateFn, filterFns ...func(interface{}) bool) (allocatedJobList []*api.JobInfo) {
	// for every user, allocate user pending job in order, under the restriction of user quota
	for queueID := range userJobPQ {
		pendingJobs := userJobPQ[queueID].Filter(filterFns...)
		for !pendingJobs.Empty() {
			// select one job
			job := pendingJobs.Pop().(*api.JobInfo)
			// check quota
			if ssn.Overused(ssn.Queues[queueID]) {
				// quota exceeds, skip current user
				break
			}
			// try to allocate the job
			// allocateJob will affect job status and resource status
			canAllocate := false
			if canAllocate = allocateJob(ssn, job, candidateNodes, predicatedFn); canAllocate {
				allocatedJobList = append(allocatedJobList, job)
				// update queue quota usage (in allocateJobFn)
				ssn.AllocateJob(job)
			}
			// if enable block scheduling, skip to next user once allocation failed
			if !canAllocate && la.isBlock {
				break
			}
		}
	}
	return
}

func allocateJob(ssn *framework.Session, job *api.JobInfo, nodes []*api.NodeInfo, predicateFn api.PredicateFn) bool {
	// statement only handles single job resource allocate
	// resource will be restored only when job is not allocated due to gang
	stmt := framework.NewStatement(ssn)
	tasks := util.NewPriorityQueue(ssn.TaskOrderFn)
	for _, task := range job.Tasks {
		// use task clone to avoid modify original task status
		tasks.Push(task.Clone())
	}

	allocatedFailedTaskCnt := 0

	for !tasks.Empty() {
		task := tasks.Pop().(*api.TaskInfo)

		predicateNodes, fitErrors := util.PredicateNodes(task, nodes, predicateFn)
		if len(predicateNodes) == 0 {
			job.NodesFitErrors[task.UID] = fitErrors
			allocatedFailedTaskCnt++
			break
		}

		var candidateNodes []*api.NodeInfo
		for _, n := range predicateNodes {
			if task.InitResreq.LessEqual(n.Idle) || task.InitResreq.LessEqual(n.FutureIdle()) {
				candidateNodes = append(candidateNodes, n)
			}
		}

		// If not candidate nodes for this task, skip it.
		if len(candidateNodes) == 0 {
			allocatedFailedTaskCnt++
			break
		}

		nodeScores := util.PrioritizeNodes(task, candidateNodes, ssn.BatchNodeOrderFn, ssn.NodeOrderMapFn, ssn.NodeOrderReduceFn)

		node := ssn.BestNodeFn(task, nodeScores)
		if node == nil {
			node = util.SelectBestNode(nodeScores)
		}

		// Allocate idle resource to the task.
		if task.InitResreq.LessEqual(node.Idle) {
			klog.V(3).Infof("Binding Task <%v/%v> to node <%v>",
				task.Namespace, task.Name, node.Name)
			if err := stmt.Allocate(task, node.Name); err != nil {
				allocatedFailedTaskCnt++
				klog.Errorf("Failed to bind Task %v on %v in Session %v, err: %v",
					task.UID, node.Name, ssn.UID, err)
			}
		} else if task.InitResreq.LessEqual(node.FutureIdle()) {
			klog.V(3).Infof("Pipelining Task <%v/%v> to node <%v> for <%v> on <%v>",
				task.Namespace, task.Name, node.Name, task.InitResreq, node.Releasing)
			if err := stmt.Pipeline(task, node.Name); err != nil {
				allocatedFailedTaskCnt++
				klog.Errorf("Failed to pipeline Task %v on %v in Session %v for %v.",
					task.UID, node.Name, ssn.UID, err)
			}
		} else {
			// no enough resource, job allocation failed because of gang scheduling
			allocatedFailedTaskCnt++
			break
		}
	}
	// check if all tasks are allocated (gang)
	if allocatedFailedTaskCnt == 0 {
		return true
	} else {
		// discard allocated task impact on resource
		stmt.Discard()
	}
	return false
}

func reclaimJobResource(ssn *framework.Session, job *api.JobInfo) {
	for _, task := range job.Tasks {
		if node, found := ssn.Nodes[task.NodeName]; found {
			klog.V(3).Infof("Remove Task <%v> on node <%v>", task.Name, task.NodeName)
			err := node.RemoveTask(task)
			if err != nil {
				klog.Errorf("Failed to remove Task <%v> on node <%v>: %s", task.Name, task.NodeName, err.Error())
			}
		}
		task.NodeName = ""
	}
}

func renewalSucceedJob(ssn *framework.Session, job *api.JobInfo) {
	// get vc job
	vcJob, err := ssn.VcClient().BatchV1alpha1().Jobs(job.Namespace).Get(context.TODO(), job.Name, metav1.GetOptions{})
	if err != nil {
		klog.V(3).Infof("renewalSucceedJob: get job err %v", err)
	}
	// annotate vcJob
	if vcJob.Annotations == nil {
		vcJob.Annotations = make(map[string]string)
	}
	vcJob.Annotations[PodGroupRenewingResultAnnoKey] = PodGroupRenewingSucceeded
	// update vcJob
	_, err = ssn.VcClient().BatchV1alpha1().Jobs(job.Namespace).Update(context.TODO(), vcJob, metav1.UpdateOptions{})
	if err != nil {
		klog.V(3).Infof("renewalSucceedJob: update job err %v", err)
	}
}

func renewalFailedJob(ssn *framework.Session, job *api.JobInfo) {
	// get vc job
	vcJob, err := ssn.VcClient().BatchV1alpha1().Jobs(job.Namespace).Get(context.TODO(), job.Name, metav1.GetOptions{})
	if err != nil {
		klog.V(3).Infof("renewalFailedJob: get job err %v", err)
	}
	// annotate vcJob
	if vcJob.Annotations == nil {
		vcJob.Annotations = make(map[string]string)
	}
	vcJob.Annotations[PodGroupRenewingResultAnnoKey] = PodGroupRenewingFailed
	// update vcJob
	_, err = ssn.VcClient().BatchV1alpha1().Jobs(job.Namespace).Update(context.TODO(), vcJob, metav1.UpdateOptions{})
	if err != nil {
		klog.V(3).Infof("renewalFailedJob: update job err %v", err)
	}
}

func (la *Action) getIsBlock(ssn *framework.Session) {
	arg := framework.GetArgOfActionFromConf(ssn.Configurations, la.Name())
	if arg != nil {
		arg.GetBool(&la.isBlock, blockScheduling)
	}
}
