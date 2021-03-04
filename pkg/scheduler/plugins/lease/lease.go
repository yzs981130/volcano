package lease

import (
	"k8s.io/klog"

	"volcano.sh/volcano/pkg/scheduler/api"
	"volcano.sh/volcano/pkg/scheduler/framework"
)

// PluginName indicates name of volcano scheduler plugin.
const PluginName = "lease"

type leasePlugin struct {
	// Arguments given for the plugin
	pluginArguments framework.Arguments
	// Key is Job ID
	jobAttrs map[api.JobID]*leaseAttr
}

type leaseAttr struct {
	utilized	float64
	deserved	float64
	fairness	float64
}

// New return priority plugin
func New(arguments framework.Arguments) framework.Plugin {
	return &leasePlugin{pluginArguments: arguments}
}

func (lp *leasePlugin) Name() string {
	return PluginName
}

func (lp *leasePlugin) OnSessionOpen(ssn *framework.Session) {
	// use lp.jobAttrs[lv.UID].fairness for job selection
	jobOrderFn := func(l, r interface{}) int {
		lv := l.(*api.JobInfo)
		rv := r.(*api.JobInfo)

		klog.V(4).Infof("Lease JobOrderFn: <%v/%v> priority: %d, <%v/%v> priority: %d",
			lv.Namespace, lv.Name, lp.jobAttrs[lv.UID].fairness, rv.Namespace, rv.Name, lp.jobAttrs[rv.UID].fairness)

		if lv.Priority > rv.Priority {
			return -1
		}

		if lv.Priority < rv.Priority {
			return 1
		}

		return 0
	}

	ssn.AddJobOrderFn(lp.Name(), jobOrderFn)
}

func (lp *leasePlugin) OnSessionClose(ssn *framework.Session) {}
