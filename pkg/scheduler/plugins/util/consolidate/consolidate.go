package consolidate

import (
	"context"

	"volcano.sh/volcano/pkg/scheduler/api"
	schedulerframework "volcano.sh/volcano/pkg/scheduler/framework"
	"volcano.sh/volcano/pkg/scheduler/plugins/util"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	pluginhelper "k8s.io/kubernetes/pkg/scheduler/framework/plugins/helper"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
)

// Consolidate is a plugin that checks if a pod node selector matches the node label.
type Consolidate struct {
	handle framework.FrameworkHandle
	ssn    *schedulerframework.Session
}

var _ framework.ScorePlugin = &Consolidate{}

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name        = "Consolidate"
	scaledRatio = 100
)

// Name returns name of the plugin. It is used in logs, etc.
func (pl *Consolidate) Name() string {
	return Name
}

// Score invoked at the Score extension point.
func (pl *Consolidate) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	// get node from ssn.Nodes
	var node *api.NodeInfo
	for _, n := range pl.ssn.Nodes {
		if n.Name == nodeName {
			node = n
		}
	}
	// never happens
	if node == nil {
		panic("no node in ssn.Node in prioritize")
	}

	var score int64
	var nodeAllocatableGPU, nodeRequestedGPU float64
	gpuDemand := float64(util.GetPodTotalGPUReq(pod))
	if c, exist := node.Capability.ScalarResources[api.GPUResourceName]; exist {
		nodeAllocatableGPU = c
	}
	if c, exist := node.Allocatable.ScalarResources[api.GPUResourceName]; exist {
		nodeRequestedGPU = nodeAllocatableGPU - c
	}

	// TODO: check if can place pod on this node, no need to do possibly
	// TODO: check gpuDemand true value
	if gpuDemand+nodeRequestedGPU < nodeAllocatableGPU {
		score = int64((gpuDemand + nodeRequestedGPU) * scaledRatio / nodeAllocatableGPU)
	}
	return score, nil
}

// NormalizeScore invoked after scoring all nodes.
func (pl *Consolidate) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	return pluginhelper.DefaultNormalizeScore(framework.MaxNodeScore, false, scores)
}

// ScoreExtensions of the Score plugin.
func (pl *Consolidate) ScoreExtensions() framework.ScoreExtensions {
	return pl
}

// New initializes a new plugin and returns it.
func New(_ *runtime.Unknown, h framework.FrameworkHandle, s *schedulerframework.Session) (framework.Plugin, error) {
	return &Consolidate{
		handle: h,
		ssn:    s,
	}, nil
}
