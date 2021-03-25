/*
Copyright 2018 The Volcano Authors.

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

package mutate

import (
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/api/admission/v1beta1"
	whv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	"pkg.yezhisheng.me/volcano/pkg/apis/batch/v1alpha1"
	"pkg.yezhisheng.me/volcano/pkg/webhooks/router"
	"pkg.yezhisheng.me/volcano/pkg/webhooks/schema"
	"pkg.yezhisheng.me/volcano/pkg/webhooks/util"
)

const (
	// DefaultQueue constant stores the name of the queue as "default"
	DefaultQueue = "default"

	defaultSchedulerName = "volcano"
)

func init() {
	router.RegisterAdmission(service)
}

var service = &router.AdmissionService{
	Path: "/jobs/mutate",
	Func: Jobs,

	MutatingConfig: &whv1beta1.MutatingWebhookConfiguration{
		Webhooks: []whv1beta1.MutatingWebhook{{
			Name: "mutatejob.volcano.sh",
			Rules: []whv1beta1.RuleWithOperations{
				{
					Operations: []whv1beta1.OperationType{whv1beta1.Create},
					Rule: whv1beta1.Rule{
						APIGroups:   []string{"batch.volcano.sh"},
						APIVersions: []string{"v1alpha1"},
						Resources:   []string{"jobs"},
					},
				},
			},
		}},
	},
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// MutateJobs mutate jobs.
func Jobs(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	klog.V(3).Infof("mutating jobs")

	job, err := schema.DecodeJob(ar.Request.Object, ar.Request.Resource)
	if err != nil {
		return util.ToAdmissionResponse(err)
	}

	var patchBytes []byte
	switch ar.Request.Operation {
	case v1beta1.Create:
		patchBytes, _ = createPatch(job)
		break
	default:
		err = fmt.Errorf("expect operation to be 'CREATE' ")
		return util.ToAdmissionResponse(err)
	}

	klog.V(3).Infof("AdmissionResponse: patch=%v", string(patchBytes))
	reviewResponse := v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
	}
	pt := v1beta1.PatchTypeJSONPatch
	reviewResponse.PatchType = &pt

	return &reviewResponse
}

func createPatch(job *v1alpha1.Job) ([]byte, error) {
	var patch []patchOperation
	pathQueue := patchDefaultQueue(job)
	if pathQueue != nil {
		patch = append(patch, *pathQueue)
	}
	pathScheduler := patchDefaultScheduler(job)
	if pathScheduler != nil {
		patch = append(patch, *pathScheduler)
	}
	pathSpec := mutateSpec(job.Spec.Tasks, "/spec/tasks")
	if pathSpec != nil {
		patch = append(patch, *pathSpec)
	}
	return json.Marshal(patch)
}

func patchDefaultQueue(job *v1alpha1.Job) *patchOperation {
	//Add default queue if not specified.
	if job.Spec.Queue == "" {
		return &patchOperation{Op: "add", Path: "/spec/queue", Value: DefaultQueue}
	}
	return nil
}

func patchDefaultScheduler(job *v1alpha1.Job) *patchOperation {
	// Add default scheduler name if not specified.
	if job.Spec.SchedulerName == "" {
		return &patchOperation{Op: "add", Path: "/spec/schedulerName", Value: defaultSchedulerName}
	}
	return nil
}

func mutateSpec(tasks []v1alpha1.TaskSpec, basePath string) *patchOperation {
	patched := false
	for index := range tasks {
		// add default task name
		taskName := tasks[index].Name
		if len(taskName) == 0 {
			patched = true
			tasks[index].Name = v1alpha1.DefaultTaskSpec + strconv.Itoa(index)
		}

		if tasks[index].Template.Spec.HostNetwork && tasks[index].Template.Spec.DNSPolicy == "" {
			tasks[index].Template.Spec.DNSPolicy = v1.DNSClusterFirstWithHostNet
		}

	}
	if !patched {
		return nil
	}
	return &patchOperation{
		Op:    "replace",
		Path:  basePath,
		Value: tasks,
	}
}
