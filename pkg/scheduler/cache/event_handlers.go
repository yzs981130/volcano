/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/api/scheduling/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"pkg.yezhisheng.me/volcano/pkg/apis/scheduling"
	"pkg.yezhisheng.me/volcano/pkg/apis/scheduling/scheme"
	schedulingv1 "pkg.yezhisheng.me/volcano/pkg/apis/scheduling/v1beta1"
	"pkg.yezhisheng.me/volcano/pkg/apis/utils"
	schedulingapi "pkg.yezhisheng.me/volcano/pkg/scheduler/api"
)

func isTerminated(status schedulingapi.TaskStatus) bool {
	return status == schedulingapi.Succeeded || status == schedulingapi.Failed
}

// getOrCreateJob will return corresponding Job for pi if it exists, or it will create a Job and return it if
// pi.Pod.Spec.SchedulerName is same as volcano scheduler's name, otherwise it will return nil.
func (sc *SchedulerCache) getOrCreateJob(pi *schedulingapi.TaskInfo) *schedulingapi.JobInfo {
	if len(pi.Job) == 0 {
		if pi.Pod.Spec.SchedulerName != sc.schedulerName {
			klog.V(4).Infof("Pod %s/%s will not not scheduled by %s, skip creating PodGroup and Job for it",
				pi.Pod.Namespace, pi.Pod.Name, sc.schedulerName)
		}
		return nil
	}

	if _, found := sc.Jobs[pi.Job]; !found {
		sc.Jobs[pi.Job] = schedulingapi.NewJobInfo(pi.Job)
	}

	return sc.Jobs[pi.Job]
}

func (sc *SchedulerCache) addTask(pi *schedulingapi.TaskInfo) error {
	job := sc.getOrCreateJob(pi)
	if job != nil {
		job.AddTaskInfo(pi)
	}

	if len(pi.NodeName) != 0 {
		if _, found := sc.Nodes[pi.NodeName]; !found {
			sc.Nodes[pi.NodeName] = schedulingapi.NewNodeInfo(nil)
		}

		node := sc.Nodes[pi.NodeName]
		if !isTerminated(pi.Status) {
			return node.AddTask(pi)
		}
	}

	return nil
}

// Assumes that lock is already acquired.
func (sc *SchedulerCache) addPod(pod *v1.Pod) error {
	pi := schedulingapi.NewTaskInfo(pod)

	return sc.addTask(pi)
}

func (sc *SchedulerCache) syncTask(oldTask *schedulingapi.TaskInfo) error {
	newPod, err := sc.kubeClient.CoreV1().Pods(oldTask.Namespace).Get(context.TODO(), oldTask.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			err := sc.deleteTask(oldTask)
			if err != nil {
				klog.Errorf("Failed to delete Pod <%v/%v> and remove from cache: %s", oldTask.Namespace, oldTask.Name, err.Error())
				return err
			}
			klog.V(3).Infof("Pod <%v/%v> was deleted, removed from cache.", oldTask.Namespace, oldTask.Name)

			return nil
		}
		return fmt.Errorf("failed to get Pod <%v/%v>: err %v", oldTask.Namespace, oldTask.Name, err)
	}

	newTask := schedulingapi.NewTaskInfo(newPod)

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()
	return sc.updateTask(oldTask, newTask)
}

func (sc *SchedulerCache) updateTask(oldTask, newTask *schedulingapi.TaskInfo) error {
	if err := sc.deleteTask(oldTask); err != nil {
		klog.Warningf("Failed to delete task: %v", err)
	}

	return sc.addTask(newTask)
}

// Assumes that lock is already acquired.
func (sc *SchedulerCache) updatePod(oldPod, newPod *v1.Pod) error {
	if err := sc.deletePod(oldPod); err != nil {
		return err
	}
	//when delete pod, the ownerreference of pod will be set nil,just as orphan pod
	if len(utils.GetController(newPod)) == 0 {
		newPod.OwnerReferences = oldPod.OwnerReferences
	}
	return sc.addPod(newPod)
}

func (sc *SchedulerCache) deleteTask(pi *schedulingapi.TaskInfo) error {
	var jobErr, nodeErr error

	if len(pi.Job) != 0 {
		if job, found := sc.Jobs[pi.Job]; found {
			jobErr = job.DeleteTaskInfo(pi)
		} else {
			jobErr = fmt.Errorf("failed to find Job <%v> for Task %v/%v",
				pi.Job, pi.Namespace, pi.Name)
		}
	}

	if len(pi.NodeName) != 0 {
		node := sc.Nodes[pi.NodeName]
		if node != nil {
			nodeErr = node.RemoveTask(pi)
		}
	}

	if jobErr != nil || nodeErr != nil {
		return schedulingapi.MergeErrors(jobErr, nodeErr)
	}

	return nil
}

// Assumes that lock is already acquired.
func (sc *SchedulerCache) deletePod(pod *v1.Pod) error {
	pi := schedulingapi.NewTaskInfo(pod)

	// Delete the Task in cache to handle Binding status.
	task := pi
	if job, found := sc.Jobs[pi.Job]; found {
		if t, found := job.Tasks[pi.UID]; found {
			task = t
		}
	}
	if err := sc.deleteTask(task); err != nil {
		klog.Warningf("Failed to delete task: %v", err)
	}

	// If job was terminated, delete it.
	if job, found := sc.Jobs[pi.Job]; found && schedulingapi.JobTerminated(job) {
		sc.deleteJob(job)
	}

	return nil
}

// AddPod add pod to scheduler cache
func (sc *SchedulerCache) AddPod(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		klog.Errorf("Cannot convert to *v1.Pod: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.addPod(pod)
	if err != nil {
		klog.Errorf("Failed to add pod <%s/%s> into cache: %v",
			pod.Namespace, pod.Name, err)
		return
	}
	klog.V(3).Infof("Added pod <%s/%v> into cache.", pod.Namespace, pod.Name)
}

// UpdatePod update pod to scheduler cache
func (sc *SchedulerCache) UpdatePod(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*v1.Pod)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *v1.Pod: %v", oldObj)
		return
	}
	newPod, ok := newObj.(*v1.Pod)
	if !ok {
		klog.Errorf("Cannot convert newObj to *v1.Pod: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.updatePod(oldPod, newPod)
	if err != nil {
		klog.Errorf("Failed to update pod %v in cache: %v", oldPod.Name, err)
		return
	}

	klog.V(4).Infof("Updated pod <%s/%v> in cache.", oldPod.Namespace, oldPod.Name)
}

// DeletePod delete pod from scheduler cache
func (sc *SchedulerCache) DeletePod(obj interface{}) {
	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*v1.Pod)
		if !ok {
			klog.Errorf("Cannot convert to *v1.Pod: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *v1.Pod: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.deletePod(pod)
	if err != nil {
		klog.Errorf("Failed to delete pod %v from cache: %v", pod.Name, err)
		return
	}

	klog.V(3).Infof("Deleted pod <%s/%v> from cache.", pod.Namespace, pod.Name)
}

// Assumes that lock is already acquired.
func (sc *SchedulerCache) addNode(node *v1.Node) error {
	if sc.Nodes[node.Name] != nil {
		sc.Nodes[node.Name].SetNode(node)
	} else {
		sc.Nodes[node.Name] = schedulingapi.NewNodeInfo(node)
	}
	return nil
}

// Assumes that lock is already acquired.
func (sc *SchedulerCache) updateNode(oldNode, newNode *v1.Node) error {
	if sc.Nodes[newNode.Name] != nil {
		sc.Nodes[newNode.Name].SetNode(newNode)
		return nil
	}

	return fmt.Errorf("node <%s> does not exist", newNode.Name)
}

// Assumes that lock is already acquired.
func (sc *SchedulerCache) deleteNode(node *v1.Node) error {
	if _, ok := sc.Nodes[node.Name]; !ok {
		return fmt.Errorf("node <%s> does not exist", node.Name)
	}
	delete(sc.Nodes, node.Name)
	return nil
}

// AddNode add node to scheduler cache
func (sc *SchedulerCache) AddNode(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if !ok {
		klog.Errorf("Cannot convert to *v1.Node: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.addNode(node)
	if err != nil {
		klog.Errorf("Failed to add node %s into cache: %v", node.Name, err)
		return
	}
}

// UpdateNode update node to scheduler cache
func (sc *SchedulerCache) UpdateNode(oldObj, newObj interface{}) {
	oldNode, ok := oldObj.(*v1.Node)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *v1.Node: %v", oldObj)
		return
	}
	newNode, ok := newObj.(*v1.Node)
	if !ok {
		klog.Errorf("Cannot convert newObj to *v1.Node: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.updateNode(oldNode, newNode)
	if err != nil {
		klog.Errorf("Failed to update node %v in cache: %v", oldNode.Name, err)
		return
	}
}

// DeleteNode delete node from scheduler cache
func (sc *SchedulerCache) DeleteNode(obj interface{}) {
	var node *v1.Node
	switch t := obj.(type) {
	case *v1.Node:
		node = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		node, ok = t.Obj.(*v1.Node)
		if !ok {
			klog.Errorf("Cannot convert to *v1.Node: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *v1.Node: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.deleteNode(node)
	if err != nil {
		klog.Errorf("Failed to delete node %s from cache: %v", node.Name, err)
		return
	}
}

func getJobID(pg *schedulingapi.PodGroup) schedulingapi.JobID {
	return schedulingapi.JobID(fmt.Sprintf("%s/%s", pg.Namespace, pg.Name))
}

// Assumes that lock is already acquired.
func (sc *SchedulerCache) setPodGroup(ss *schedulingapi.PodGroup) error {
	job := getJobID(ss)
	if _, found := sc.Jobs[job]; !found {
		sc.Jobs[job] = schedulingapi.NewJobInfo(job)
	}

	sc.Jobs[job].SetPodGroup(ss)

	// TODO(k82cn): set default queue in admission.
	if len(ss.Spec.Queue) == 0 {
		sc.Jobs[job].Queue = schedulingapi.QueueID(sc.defaultQueue)
	}

	return nil
}

// Assumes that lock is already acquired.
func (sc *SchedulerCache) updatePodGroup(newPodGroup *schedulingapi.PodGroup) error {
	return sc.setPodGroup(newPodGroup)
}

// Assumes that lock is already acquired.
func (sc *SchedulerCache) deletePodGroup(id schedulingapi.JobID) error {
	job, found := sc.Jobs[id]
	if !found {
		return fmt.Errorf("can not found job %v", id)
	}

	// Unset SchedulingSpec
	job.UnsetPodGroup()

	sc.deleteJob(job)

	return nil
}

// AddPodGroupV1beta1 add podgroup to scheduler cache
func (sc *SchedulerCache) AddPodGroupV1beta1(obj interface{}) {
	ss, ok := obj.(*schedulingv1.PodGroup)
	if !ok {
		klog.Errorf("Cannot convert to *schedulingv1.PodGroup: %v", obj)
		return
	}

	podgroup := scheduling.PodGroup{}
	if err := scheme.Scheme.Convert(ss, &podgroup, nil); err != nil {
		klog.Errorf("Failed to convert podgroup from %T to %T", ss, podgroup)
		return
	}

	pg := &schedulingapi.PodGroup{PodGroup: podgroup, Version: schedulingapi.PodGroupVersionV1Beta1}
	klog.V(4).Infof("Add PodGroup(%s) into cache, spec(%#v)", ss.Name, ss.Spec)

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if err := sc.setPodGroup(pg); err != nil {
		klog.Errorf("Failed to add PodGroup %s into cache: %v", ss.Name, err)
		return
	}
}

// UpdatePodGroupV1beta1 add podgroup to scheduler cache
func (sc *SchedulerCache) UpdatePodGroupV1beta1(oldObj, newObj interface{}) {
	oldSS, ok := oldObj.(*schedulingv1.PodGroup)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *schedulingv1.SchedulingSpec: %v", oldObj)
		return
	}
	newSS, ok := newObj.(*schedulingv1.PodGroup)
	if !ok {
		klog.Errorf("Cannot convert newObj to *schedulingv1.SchedulingSpec: %v", newObj)
		return
	}

	if oldSS.ResourceVersion == newSS.ResourceVersion {
		return
	}

	podgroup := scheduling.PodGroup{}
	if err := scheme.Scheme.Convert(newSS, &podgroup, nil); err != nil {
		klog.Errorf("Failed to convert podgroup from %T to %T", newSS, podgroup)
		return
	}

	pg := &schedulingapi.PodGroup{PodGroup: podgroup, Version: schedulingapi.PodGroupVersionV1Beta1}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if err := sc.updatePodGroup(pg); err != nil {
		klog.Errorf("Failed to update SchedulingSpec %s into cache: %v", pg.Name, err)
		return
	}
}

// DeletePodGroupV1beta1 delete podgroup from scheduler cache
func (sc *SchedulerCache) DeletePodGroupV1beta1(obj interface{}) {
	var ss *schedulingv1.PodGroup
	switch t := obj.(type) {
	case *schedulingv1.PodGroup:
		ss = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		ss, ok = t.Obj.(*schedulingv1.PodGroup)
		if !ok {
			klog.Errorf("Cannot convert to podgroup: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to podgroup: %v", t)
		return
	}

	jobID := schedulingapi.JobID(fmt.Sprintf("%s/%s", ss.Namespace, ss.Name))

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if err := sc.deletePodGroup(jobID); err != nil {
		klog.Errorf("Failed to delete podgroup %s from cache: %v", ss.Name, err)
		return
	}
}

// AddQueueV1beta1 add queue to scheduler cache
func (sc *SchedulerCache) AddQueueV1beta1(obj interface{}) {
	ss, ok := obj.(*schedulingv1.Queue)
	if !ok {
		klog.Errorf("Cannot convert to *schedulingv1.Queue: %v", obj)
		return
	}

	queue := &scheduling.Queue{}
	if err := scheme.Scheme.Convert(ss, queue, nil); err != nil {
		klog.Errorf("Failed to convert queue from %T to %T", ss, queue)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	klog.V(4).Infof("Add Queue(%s) into cache, spec(%#v)", ss.Name, ss.Spec)
	sc.addQueue(queue)
}

// UpdateQueueV1beta1 update queue to scheduler cache
func (sc *SchedulerCache) UpdateQueueV1beta1(oldObj, newObj interface{}) {
	oldSS, ok := oldObj.(*schedulingv1.Queue)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *schedulingv1.Queue: %v", oldObj)
		return
	}
	newSS, ok := newObj.(*schedulingv1.Queue)
	if !ok {
		klog.Errorf("Cannot convert newObj to *schedulingv1.Queue: %v", newObj)
		return
	}

	if oldSS.ResourceVersion == newSS.ResourceVersion {
		return
	}

	newQueue := &scheduling.Queue{}
	if err := scheme.Scheme.Convert(newSS, newQueue, nil); err != nil {
		klog.Errorf("Failed to convert queue from %T to %T", newSS, newQueue)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()
	sc.updateQueue(newQueue)
}

// DeleteQueueV1beta1 delete queue from the scheduler cache
func (sc *SchedulerCache) DeleteQueueV1beta1(obj interface{}) {
	var ss *schedulingv1.Queue
	switch t := obj.(type) {
	case *schedulingv1.Queue:
		ss = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		ss, ok = t.Obj.(*schedulingv1.Queue)
		if !ok {
			klog.Errorf("Cannot convert to *schedulingv1.Queue: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *schedulingv1.Queue: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()
	sc.deleteQueue(schedulingapi.QueueID(ss.Name))
}

func (sc *SchedulerCache) addQueue(queue *scheduling.Queue) {
	qi := schedulingapi.NewQueueInfo(queue)
	sc.Queues[qi.UID] = qi
}

func (sc *SchedulerCache) updateQueue(queue *scheduling.Queue) {
	sc.addQueue(queue)
}

func (sc *SchedulerCache) deleteQueue(id schedulingapi.QueueID) {
	delete(sc.Queues, id)
}

//DeletePriorityClass delete priorityclass from the scheduler cache
func (sc *SchedulerCache) DeletePriorityClass(obj interface{}) {
	var ss *v1beta1.PriorityClass
	switch t := obj.(type) {
	case *v1beta1.PriorityClass:
		ss = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		ss, ok = t.Obj.(*v1beta1.PriorityClass)
		if !ok {
			klog.Errorf("Cannot convert to *v1beta1.PriorityClass: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *v1beta1.PriorityClass: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	sc.deletePriorityClass(ss)
}

//UpdatePriorityClass update priorityclass to scheduler cache
func (sc *SchedulerCache) UpdatePriorityClass(oldObj, newObj interface{}) {
	oldSS, ok := oldObj.(*v1beta1.PriorityClass)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *v1beta1.PriorityClass: %v", oldObj)

		return

	}

	newSS, ok := newObj.(*v1beta1.PriorityClass)
	if !ok {
		klog.Errorf("Cannot convert newObj to *v1beta1.PriorityClass: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	sc.deletePriorityClass(oldSS)
	sc.addPriorityClass(newSS)
}

//AddPriorityClass add priorityclass to scheduler cache
func (sc *SchedulerCache) AddPriorityClass(obj interface{}) {
	var ss *v1beta1.PriorityClass
	switch t := obj.(type) {
	case *v1beta1.PriorityClass:
		ss = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		ss, ok = t.Obj.(*v1beta1.PriorityClass)
		if !ok {
			klog.Errorf("Cannot convert to *v1beta1.PriorityClass: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *v1beta1.PriorityClass: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	sc.addPriorityClass(ss)
}

func (sc *SchedulerCache) deletePriorityClass(pc *v1beta1.PriorityClass) {
	if pc.GlobalDefault {
		sc.defaultPriorityClass = nil
		sc.defaultPriority = 0

	}

	delete(sc.PriorityClasses, pc.Name)
}

func (sc *SchedulerCache) addPriorityClass(pc *v1beta1.PriorityClass) {
	if pc.GlobalDefault {
		if sc.defaultPriorityClass != nil {
			klog.Errorf("Updated default priority class from <%s> to <%s> forcefully.",
				sc.defaultPriorityClass.Name, pc.Name)

		}
		sc.defaultPriorityClass = pc
		sc.defaultPriority = pc.Value
	}

	sc.PriorityClasses[pc.Name] = pc
}

func (sc *SchedulerCache) updateResourceQuota(quota *v1.ResourceQuota) {
	collection, ok := sc.NamespaceCollection[quota.Namespace]
	if !ok {
		collection = schedulingapi.NewNamespaceCollection(quota.Namespace)
		sc.NamespaceCollection[quota.Namespace] = collection
	}

	collection.Update(quota)
}

func (sc *SchedulerCache) deleteResourceQuota(quota *v1.ResourceQuota) {
	collection, ok := sc.NamespaceCollection[quota.Namespace]
	if !ok {
		return
	}

	collection.Delete(quota)
}

// DeleteResourceQuota delete ResourceQuota from the scheduler cache
func (sc *SchedulerCache) DeleteResourceQuota(obj interface{}) {
	var r *v1.ResourceQuota
	switch t := obj.(type) {
	case *v1.ResourceQuota:
		r = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		r, ok = t.Obj.(*v1.ResourceQuota)
		if !ok {
			klog.Errorf("Cannot convert to *v1.ResourceQuota: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *v1.ResourceQuota: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	klog.V(3).Infof("Delete ResourceQuota <%s/%v> in cache", r.Namespace, r.Name)
	sc.deleteResourceQuota(r)
}

// UpdateResourceQuota update ResourceQuota to scheduler cache
func (sc *SchedulerCache) UpdateResourceQuota(oldObj, newObj interface{}) {
	newR, ok := newObj.(*v1.ResourceQuota)
	if !ok {
		klog.Errorf("Cannot convert newObj to *v1.ResourceQuota: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	klog.V(3).Infof("Update ResourceQuota <%s/%v> in cache, with spec: %v.", newR.Namespace, newR.Name, newR.Spec.Hard)
	sc.updateResourceQuota(newR)
}

// AddResourceQuota add ResourceQuota to scheduler cache
func (sc *SchedulerCache) AddResourceQuota(obj interface{}) {
	var r *v1.ResourceQuota
	switch t := obj.(type) {
	case *v1.ResourceQuota:
		r = t
	default:
		klog.Errorf("Cannot convert to *v1.ResourceQuota: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	klog.V(3).Infof("Add ResourceQuota <%s/%v> in cache, with spec: %v.", r.Namespace, r.Name, r.Spec.Hard)
	sc.updateResourceQuota(r)
}
