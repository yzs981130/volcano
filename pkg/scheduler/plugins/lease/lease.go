package lease

import (
	"context"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
	"volcano.sh/volcano/pkg/scheduler/plugins/util"
	"volcano.sh/volcano/pkg/scheduler/plugins/util/consolidate"
	"volcano.sh/volcano/pkg/scheduler/plugins/util/k8s"

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
}

func (lp *leasePlugin) OnSessionClose(ssn *framework.Session) {
	// clear jobAttr
	lp.jobAttrs = map[api.JobID]*leaseAttr{}
}
