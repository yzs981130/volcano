/*
Copyright 2018 The Kubernetes Authors.

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

package dlas

import (
	"k8s.io/klog"
	"volcano.sh/volcano/pkg/scheduler/api"
	"volcano.sh/volcano/pkg/scheduler/framework"
	"volcano.sh/volcano/pkg/scheduler/util"
)

// PluginName indicates name of volcano scheduler plugin.
const PluginName = "dlas"

var queueLimit [2]int = [2]int{3600, 10800}

type dlasPlugin struct {
	// Arguments given for the plugin
	pluginArguments framework.Arguments
	jobAttrs        map[api.JobID]*jobAttr
}
type jobAttr struct {
	submitTime        int
	totalExecutedTime int
	queueID           int
}

// New return priority plugin
func New(arguments framework.Arguments) framework.Plugin {
	return &dlasPlugin{pluginArguments: arguments}
}

func (pp *dlasPlugin) Name() string {
	return PluginName
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

func (pp *dlasPlugin) AssignQueue(service int) int {
	for i := 0; i < len(queueLimit); i++ {
		if service < queueLimit[i] {
			return i
		}
	}
	return len(queueLimit)
}

func (pp *dlasPlugin) OnSessionOpen(ssn *framework.Session) {

	for _, job := range ssn.Jobs {
		if _, found := pp.jobAttrs[job.UID]; !found {
			jAttr := &jobAttr{
				totalExecutedTime: 0,
				queueID:           0,
			}
			pp.jobAttrs[job.UID] = jAttr
		}
		jAttr := pp.jobAttrs[job.UID]
		jobService := int(getJobGPUReq(job)) * jAttr.totalExecutedTime
		jAttr.queueID = pp.AssignQueue(jobService)
	}
	jobOrderFn := func(l, r interface{}) int {
		lv := l.(*api.JobInfo)
		rv := r.(*api.JobInfo)

		klog.V(4).Infof("Priority JobOrderFn: <%v/%v> priority: %d, <%v/%v> priority: %d",
			lv.Namespace, lv.Name, lv.Priority, rv.Namespace, rv.Name, rv.Priority)

		lJobAttr := pp.jobAttrs[lv.UID]
		rJobAttr := pp.jobAttrs[rv.UID]

		if lJobAttr.queueID == rJobAttr.queueID {
			return util.CompareInt(lJobAttr.submitTime, rJobAttr.submitTime)
		}
		return util.CompareInt(lJobAttr.queueID, rJobAttr.queueID)
	}

	ssn.AddJobOrderFn(pp.Name(), jobOrderFn)

	preemptableFn := func(preemptor *api.TaskInfo, preemptees []*api.TaskInfo) []*api.TaskInfo {
		preemptorJob := ssn.Jobs[preemptor.Job]
		preemptorJobAttr := pp.jobAttrs[preemptorJob.UID]

		var victims []*api.TaskInfo
		for _, preemptee := range preemptees {
			preempteeJob := ssn.Jobs[preemptee.Job]
			preempteeJobAttr := pp.jobAttrs[preempteeJob.UID]

			if preempteeJobAttr.queueID <= preemptorJobAttr.queueID {
				klog.V(4).Infof("Can not preempt task <%v/%v> because "+
					"preemptee has greater or equal job priority (%d) than preemptor (%d)",
					preemptee.Namespace, preemptor.Name, preempteeJobAttr.queueID, preemptorJobAttr.queueID)
			} else {
				victims = append(victims, preemptee)
			}
		}

		klog.V(4).Infof("Victims from Priority plugins are %+v", victims)
		return victims
	}

	ssn.AddPreemptableFn(pp.Name(), preemptableFn)
}

func isFinishedJob(job *api.JobInfo) bool {
	for status := range job.TaskStatusIndex {
		if !(status == api.Succeeded || status == api.Failed) {
			return false
		}
	}
	return true
}

func (pp *dlasPlugin) OnSessionClose(ssn *framework.Session) {
	for _, job := range ssn.Jobs {
		if isFinishedJob(job) {
			delete(pp.jobAttrs, job.UID)
		}
	}
}
