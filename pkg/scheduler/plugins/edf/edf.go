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

package edf

import (
	"k8s.io/klog"
	"volcano.sh/volcano/pkg/scheduler/api"
	"volcano.sh/volcano/pkg/scheduler/framework"
	"volcano.sh/volcano/pkg/scheduler/util"
)

// PluginName indicates name of volcano scheduler plugin.
const PluginName = "edf"

type edfPlugin struct {
	// Arguments given for the plugin
	pluginArguments framework.Arguments
	jobAttrs        map[api.JobID]*jobAttr
}
type jobAttr struct {
	submitTime        int
	totalExecutedTime int
	exepectTime       int
}

// New return priority plugin
func New(arguments framework.Arguments) framework.Plugin {
	return &edfPlugin{pluginArguments: arguments}
}

func (pp *edfPlugin) Name() string {
	return PluginName
}

func (pp *edfPlugin) OnSessionOpen(ssn *framework.Session) {

	jobOrderFn := func(l, r interface{}) int {
		lv := l.(*api.JobInfo)
		rv := r.(*api.JobInfo)

		klog.V(4).Infof("Priority JobOrderFn: <%v/%v> priority: %d, <%v/%v> priority: %d",
			lv.Namespace, lv.Name, lv.Priority, rv.Namespace, rv.Name, rv.Priority)

		lJobAttr := pp.jobAttrs[lv.UID]
		rJobAttr := pp.jobAttrs[rv.UID]
		return util.CompareInt(lJobAttr.exepectTime, rJobAttr.exepectTime)
	}

	ssn.AddJobOrderFn(pp.Name(), jobOrderFn)

	preemptableFn := func(preemptor *api.TaskInfo, preemptees []*api.TaskInfo) []*api.TaskInfo {
		preemptorJob := ssn.Jobs[preemptor.Job]
		preemptorJobAttr := pp.jobAttrs[preemptorJob.UID]

		var victims []*api.TaskInfo
		for _, preemptee := range preemptees {
			preempteeJob := ssn.Jobs[preemptee.Job]
			preempteeJobAttr := pp.jobAttrs[preempteeJob.UID]

			if preempteeJobAttr.exepectTime <= preemptorJobAttr.exepectTime {
				klog.V(4).Infof("Can not preempt task <%v/%v> because "+
					"preemptee has greater or equal job priority (%d) than preemptor (%d)",
					preemptee.Namespace, preemptee.Name, preempteeJobAttr.exepectTime, preemptorJobAttr.exepectTime)
			} else {
				victims = append(victims, preemptee)
			}
		}

		klog.V(4).Infof("Victims from Priority plugins are %+v", victims)
		return victims
	}

	ssn.AddPreemptableFn(pp.Name(), preemptableFn)
}

func (pp *edfPlugin) OnSessionClose(ssn *framework.Session) {}
