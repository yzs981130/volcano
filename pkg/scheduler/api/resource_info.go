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

package api

import (
	"fmt"
	"math"

	v1 "k8s.io/api/core/v1"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"

	"pkg.yezhisheng.me/volcano/pkg/scheduler/util/assert"
)

// Resource struct defines all the resource type
type Resource struct {
	MilliCPU float64
	Memory   float64

	// ScalarResources
	ScalarResources map[v1.ResourceName]float64

	// MaxTaskNum is only used by predicates; it should NOT
	// be accounted in other operators, e.g. Add.
	MaxTaskNum int
}

const (
	// GPUResourceName need to follow https://github.com/NVIDIA/k8s-device-plugin/blob/66a35b71ac4b5cbfb04714678b548bd77e5ba719/server.go#L20
	GPUResourceName = "nvidia.com/gpu"
)

// EmptyResource creates a empty resource object and returns
func EmptyResource() *Resource {
	return &Resource{}
}

// Clone is used to clone a resource type
func (r *Resource) Clone() *Resource {
	clone := &Resource{
		MilliCPU:   r.MilliCPU,
		Memory:     r.Memory,
		MaxTaskNum: r.MaxTaskNum,
	}

	if r.ScalarResources != nil {
		clone.ScalarResources = make(map[v1.ResourceName]float64)
		for k, v := range r.ScalarResources {
			clone.ScalarResources[k] = v
		}
	}

	return clone
}

var minMilliCPU float64 = 10
var minMilliScalarResources float64 = 10
var minMemory float64 = 1

// NewResource create a new resource object from resource list
func NewResource(rl v1.ResourceList) *Resource {
	r := EmptyResource()
	for rName, rQuant := range rl {
		switch rName {
		case v1.ResourceCPU:
			r.MilliCPU += float64(rQuant.MilliValue())
		case v1.ResourceMemory:
			r.Memory += float64(rQuant.Value())
		case v1.ResourcePods:
			r.MaxTaskNum += int(rQuant.Value())
		default:
			//NOTE: When converting this back to k8s resource, we need record the format as well as / 1000
			if v1helper.IsScalarResourceName(rName) {
				r.AddScalar(rName, float64(rQuant.MilliValue()))
			}
		}
	}
	return r
}

// IsEmpty returns bool after checking any of resource is less than min possible value
func (r *Resource) IsEmpty() bool {
	if !(r.MilliCPU < minMilliCPU && r.Memory < minMemory) {
		return false
	}

	for _, rQuant := range r.ScalarResources {
		if rQuant >= minMilliScalarResources {
			return false
		}
	}

	return true
}

// IsZero checks whether that resource is less than min possible value
func (r *Resource) IsZero(rn v1.ResourceName) bool {
	switch rn {
	case v1.ResourceCPU:
		return r.MilliCPU < minMilliCPU
	case v1.ResourceMemory:
		return r.Memory < minMemory
	default:
		if r.ScalarResources == nil {
			return true
		}

		_, found := r.ScalarResources[rn]
		assert.Assertf(found, "unknown resource %s", rn)

		return r.ScalarResources[rn] < minMilliScalarResources
	}
}

// Add is used to add the two resources
func (r *Resource) Add(rr *Resource) *Resource {
	r.MilliCPU += rr.MilliCPU
	r.Memory += rr.Memory

	for rName, rQuant := range rr.ScalarResources {
		if r.ScalarResources == nil {
			r.ScalarResources = map[v1.ResourceName]float64{}
		}
		r.ScalarResources[rName] += rQuant
	}

	return r
}

// Scale updates resource to the provided scale
func (r *Resource) Scale(scale float64) *Resource {
	r.MilliCPU *= scale
	r.Memory *= scale
	for rName, rQuant := range r.ScalarResources {
		r.ScalarResources[rName] = rQuant * scale
	}
	return r
}

//Sub subtracts two Resource objects.
func (r *Resource) Sub(rr *Resource) *Resource {
	assert.Assertf(rr.LessEqual(r), "resource is not sufficient to do operation: <%v> sub <%v>", r, rr)

	r.MilliCPU -= rr.MilliCPU
	r.Memory -= rr.Memory

	for rrName, rrQuant := range rr.ScalarResources {
		if r.ScalarResources == nil {
			return r
		}
		r.ScalarResources[rrName] -= rrQuant
	}

	return r
}

// SetMaxResource compares with ResourceList and takes max value for each Resource.
func (r *Resource) SetMaxResource(rr *Resource) {
	if r == nil || rr == nil {
		return
	}

	if rr.MilliCPU > r.MilliCPU {
		r.MilliCPU = rr.MilliCPU
	}
	if rr.Memory > r.Memory {
		r.Memory = rr.Memory
	}

	for rrName, rrQuant := range rr.ScalarResources {
		if r.ScalarResources == nil {
			r.ScalarResources = make(map[v1.ResourceName]float64)
			for k, v := range rr.ScalarResources {
				r.ScalarResources[k] = v
			}
			return
		}

		if rrQuant > r.ScalarResources[rrName] {
			r.ScalarResources[rrName] = rrQuant
		}
	}
}

//FitDelta Computes the delta between a resource object representing available
//resources an operand representing resources being requested.  Any
//field that is less than 0 after the operation represents an
//insufficient resource.
func (r *Resource) FitDelta(rr *Resource) *Resource {
	if rr.MilliCPU > 0 {
		r.MilliCPU -= rr.MilliCPU + minMilliCPU
	}

	if rr.Memory > 0 {
		r.Memory -= rr.Memory + minMemory
	}

	for rrName, rrQuant := range rr.ScalarResources {
		if r.ScalarResources == nil {
			r.ScalarResources = map[v1.ResourceName]float64{}
		}

		if rrQuant > 0 {
			r.ScalarResources[rrName] -= rrQuant + minMilliScalarResources
		}
	}

	return r
}

// Multi multiples the resource with ratio provided
func (r *Resource) Multi(ratio float64) *Resource {
	r.MilliCPU *= ratio
	r.Memory *= ratio
	for rName, rQuant := range r.ScalarResources {
		r.ScalarResources[rName] = rQuant * ratio
	}
	return r
}

// Less checks whether a resource is less than other
func (r *Resource) Less(rr *Resource) bool {
	lessFunc := func(l, r float64) bool {
		return l < r
	}

	if !lessFunc(r.MilliCPU, rr.MilliCPU) {
		return false
	}
	if !lessFunc(r.Memory, rr.Memory) {
		return false
	}

	if r.ScalarResources == nil {
		if rr.ScalarResources != nil {
			for _, rrQuant := range rr.ScalarResources {
				if rrQuant <= minMilliScalarResources {
					return false
				}
			}
		}
		return true
	}

	if rr.ScalarResources == nil {
		return false
	}

	for rName, rQuant := range r.ScalarResources {
		rrQuant := rr.ScalarResources[rName]
		if !lessFunc(rQuant, rrQuant) {
			return false
		}
	}

	return true
}

// LessEqualStrict checks whether a resource is less or equal than other
func (r *Resource) LessEqualStrict(rr *Resource) bool {
	lessFunc := func(l, r float64) bool {
		return l <= r
	}

	if !lessFunc(r.MilliCPU, rr.MilliCPU) {
		return false
	}
	if !lessFunc(r.Memory, rr.Memory) {
		return false
	}

	for rName, rQuant := range r.ScalarResources {
		if !lessFunc(rQuant, rr.ScalarResources[rName]) {
			return false
		}
	}

	return true
}

// LessEqual checks whether a resource is less than other resource
func (r *Resource) LessEqual(rr *Resource) bool {
	lessEqualFunc := func(l, r, diff float64) bool {
		if l < r || math.Abs(l-r) < diff {
			return true
		}
		return false
	}

	if !lessEqualFunc(r.MilliCPU, rr.MilliCPU, minMilliCPU) {
		return false
	}
	if !lessEqualFunc(r.Memory, rr.Memory, minMemory) {
		return false
	}

	if r.ScalarResources == nil {
		return true
	}

	for rName, rQuant := range r.ScalarResources {
		if rQuant <= minMilliScalarResources {
			continue
		}
		if rr.ScalarResources == nil {
			return false
		}

		rrQuant := rr.ScalarResources[rName]
		if !lessEqualFunc(rQuant, rrQuant, minMilliScalarResources) {
			return false
		}
	}

	return true
}

// Diff calculate the difference between two resource
func (r *Resource) Diff(rr *Resource) (*Resource, *Resource) {
	increasedVal := EmptyResource()
	decreasedVal := EmptyResource()
	if r.MilliCPU > rr.MilliCPU {
		increasedVal.MilliCPU += r.MilliCPU - rr.MilliCPU
	} else {
		decreasedVal.MilliCPU += rr.MilliCPU - r.MilliCPU
	}

	if r.Memory > rr.Memory {
		increasedVal.Memory += r.Memory - rr.Memory
	} else {
		decreasedVal.Memory += rr.Memory - r.Memory
	}

	for rName, rQuant := range r.ScalarResources {
		rrQuant := rr.ScalarResources[rName]

		if rQuant > rrQuant {
			if increasedVal.ScalarResources == nil {
				increasedVal.ScalarResources = map[v1.ResourceName]float64{}
			}
			increasedVal.ScalarResources[rName] += rQuant - rrQuant
		} else {
			if decreasedVal.ScalarResources == nil {
				decreasedVal.ScalarResources = map[v1.ResourceName]float64{}
			}
			decreasedVal.ScalarResources[rName] += rrQuant - rQuant
		}
	}

	return increasedVal, decreasedVal
}

// String returns resource details in string format
func (r *Resource) String() string {
	str := fmt.Sprintf("cpu %0.2f, memory %0.2f", r.MilliCPU, r.Memory)
	for rName, rQuant := range r.ScalarResources {
		str = fmt.Sprintf("%s, %s %0.2f", str, rName, rQuant)
	}
	return str
}

// Get returns the resource value for that particular resource type
func (r *Resource) Get(rn v1.ResourceName) float64 {
	switch rn {
	case v1.ResourceCPU:
		return r.MilliCPU
	case v1.ResourceMemory:
		return r.Memory
	default:
		if r.ScalarResources == nil {
			return 0
		}
		return r.ScalarResources[rn]
	}
}

// ResourceNames returns all resource types
func (r *Resource) ResourceNames() []v1.ResourceName {
	resNames := []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory}

	for rName := range r.ScalarResources {
		resNames = append(resNames, rName)
	}

	return resNames
}

// AddScalar adds a resource by a scalar value of this resource.
func (r *Resource) AddScalar(name v1.ResourceName, quantity float64) {
	r.SetScalar(name, r.ScalarResources[name]+quantity)
}

// SetScalar sets a resource by a scalar value of this resource.
func (r *Resource) SetScalar(name v1.ResourceName, quantity float64) {
	// Lazily allocate scalar resource map.
	if r.ScalarResources == nil {
		r.ScalarResources = map[v1.ResourceName]float64{}
	}
	r.ScalarResources[name] = quantity
}
