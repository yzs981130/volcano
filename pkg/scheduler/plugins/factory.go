/*
Copyright 2019 The Kubernetes Authors.

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

package plugins

import (
	"pkg.yezhisheng.me/volcano/pkg/scheduler/framework"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/binpack"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/conformance"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/drf"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/gang"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/lease"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/nodeorder"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/predicates"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/priority"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/proportion"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/reservation"
)

func init() {
	// Plugins for Jobs
	framework.RegisterPluginBuilder(drf.PluginName, drf.New)
	framework.RegisterPluginBuilder(gang.PluginName, gang.New)
	framework.RegisterPluginBuilder(predicates.PluginName, predicates.New)
	framework.RegisterPluginBuilder(priority.PluginName, priority.New)
	framework.RegisterPluginBuilder(nodeorder.PluginName, nodeorder.New)
	framework.RegisterPluginBuilder(conformance.PluginName, conformance.New)
	framework.RegisterPluginBuilder(binpack.PluginName, binpack.New)
	framework.RegisterPluginBuilder(reservation.PluginName, reservation.New)

	framework.RegisterPluginBuilder(lease.PluginName, lease.New)

	// Plugins for Queues
	framework.RegisterPluginBuilder(proportion.PluginName, proportion.New)
}
