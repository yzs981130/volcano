/*
Copyright 2019 The Volcano Authors.

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

package validate

import (
	"context"
	"strings"
	"testing"

	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"pkg.yezhisheng.me/volcano/pkg/apis/batch/v1alpha1"
	busv1alpha1 "pkg.yezhisheng.me/volcano/pkg/apis/bus/v1alpha1"
	schedulingv1beta2 "pkg.yezhisheng.me/volcano/pkg/apis/scheduling/v1beta1"
	fakeclient "pkg.yezhisheng.me/volcano/pkg/client/clientset/versioned/fake"
)

func TestValidateJobCreate(t *testing.T) {
	var invTTL int32 = -1
	var policyExitCode int32 = -1
	namespace := "test"
	priviledged := true

	testCases := []struct {
		Name           string
		Job            v1alpha1.Job
		ExpectErr      bool
		reviewResponse v1beta1.AdmissionResponse
		ret            string
	}{
		{
			Name: "validate valid-job",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-Job",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Event:  busv1alpha1.PodEvictedEvent,
							Action: busv1alpha1.RestartTaskAction,
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "",
			ExpectErr:      false,
		},
		// duplicate task name
		{
			Name: "duplicate-task-job",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicate-task-job",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "duplicated-task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
						{
							Name:     "duplicated-task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "duplicated task name duplicated-task-1",
			ExpectErr:      true,
		},
		// Duplicated Policy Event
		{
			Name: "job-policy-duplicated",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job-policy-duplicated",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Event:  busv1alpha1.PodFailedEvent,
							Action: busv1alpha1.AbortJobAction,
						},
						{
							Event:  busv1alpha1.PodFailedEvent,
							Action: busv1alpha1.RestartJobAction,
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "duplicate",
			ExpectErr:      true,
		},
		// Min Available illegal
		{
			Name: "Min Available illegal",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job-min-illegal",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 2,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "'minAvailable' should not be greater than total replicas in tasks",
			ExpectErr:      true,
		},
		// Job Plugin illegal
		{
			Name: "Job Plugin illegal",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job-plugin-illegal",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Plugins: map[string][]string{
						"big_plugin": {},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "unable to find job plugin: big_plugin",
			ExpectErr:      true,
		},
		// ttl-illegal
		{
			Name: "job-ttl-illegal",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job-ttl-illegal",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					TTLSecondsAfterFinished: &invTTL,
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "'ttlSecondsAfterFinished' cannot be less than zero",
			ExpectErr:      true,
		},
		// min-MinAvailable less than zero
		{
			Name: "minAvailable-lessThanZero",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "minAvailable-lessThanZero",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: -1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: false},
			ret:            "'minAvailable' must be >= 0",
			ExpectErr:      true,
		},
		// maxretry less than zero
		{
			Name: "maxretry-lessThanZero",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "maxretry-lessThanZero",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					MaxRetry:     -1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: false},
			ret:            "'maxRetry' cannot be less than zero.",
			ExpectErr:      true,
		},
		// no task specified in the job
		{
			Name: "no-task",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-task",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks:        []v1alpha1.TaskSpec{},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: false},
			ret:            "No task specified in job spec",
			ExpectErr:      true,
		},
		// replica set less than zero
		{
			Name: "replica-lessThanZero",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "replica-lessThanZero",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: -1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: false},
			ret:            "'replicas' < 0 in task: task-1;",
			ExpectErr:      true,
		},
		// task name error
		{
			Name: "nonDNS-task",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "replica-lessThanZero",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "Task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: false},
			ret: "[a DNS-1123 label must consist of lower case alphanumeric characters or '-', and " +
				"must start and end with an alphanumeric character (e.g. 'my-name',  " +
				"or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')];",
			ExpectErr: true,
		},
		// Policy Event with exit code
		{
			Name: "job-policy-withExitCode",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job-policy-withExitCode",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Event:    busv1alpha1.PodFailedEvent,
							Action:   busv1alpha1.AbortJobAction,
							ExitCode: &policyExitCode,
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "must not specify event and exitCode simultaneously",
			ExpectErr:      true,
		},
		// Both policy event and exit code are nil
		{
			Name: "policy-noEvent-noExCode",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy-noEvent-noExCode",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Action: busv1alpha1.AbortJobAction,
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "either event and exitCode should be specified",
			ExpectErr:      true,
		},
		// invalid policy event
		{
			Name: "invalid-policy-event",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-policy-event",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Event:  busv1alpha1.Event("someFakeEvent"),
							Action: busv1alpha1.AbortJobAction,
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "invalid policy event",
			ExpectErr:      true,
		},
		// invalid policy action
		{
			Name: "invalid-policy-action",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-policy-action",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Event:  busv1alpha1.PodEvictedEvent,
							Action: busv1alpha1.Action("someFakeAction"),
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "invalid policy action",
			ExpectErr:      true,
		},
		// policy exit-code zero
		{
			Name: "policy-extcode-zero",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy-extcode-zero",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Action: busv1alpha1.AbortJobAction,
							ExitCode: func(i int32) *int32 {
								return &i
							}(int32(0)),
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "0 is not a valid error code",
			ExpectErr:      true,
		},
		// duplicate policy exit-code
		{
			Name: "duplicate-exitcode",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicate-exitcode",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							ExitCode: func(i int32) *int32 {
								return &i
							}(int32(1)),
						},
						{
							ExitCode: func(i int32) *int32 {
								return &i
							}(int32(1)),
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "duplicate exitCode 1",
			ExpectErr:      true,
		},
		// Policy with any event and other events
		{
			Name: "job-policy-withExitCode",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job-policy-withExitCode",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Event:  busv1alpha1.AnyEvent,
							Action: busv1alpha1.AbortJobAction,
						},
						{
							Event:  busv1alpha1.PodFailedEvent,
							Action: busv1alpha1.RestartJobAction,
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "if there's * here, no other policy should be here",
			ExpectErr:      true,
		},
		// invalid mount volume
		{
			Name: "invalid-mount-volume",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-mount-volume",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Event:  busv1alpha1.AnyEvent,
							Action: busv1alpha1.AbortJobAction,
						},
					},
					Volumes: []v1alpha1.VolumeSpec{
						{
							MountPath: "",
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            " mountPath is required;",
			ExpectErr:      true,
		},
		// duplicate mount volume
		{
			Name: "duplicate-mount-volume",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicate-mount-volume",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Event:  busv1alpha1.AnyEvent,
							Action: busv1alpha1.AbortJobAction,
						},
					},
					Volumes: []v1alpha1.VolumeSpec{
						{
							MountPath:       "/var",
							VolumeClaimName: "pvc1",
						},
						{
							MountPath:       "/var",
							VolumeClaimName: "pvc2",
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            " duplicated mountPath: /var;",
			ExpectErr:      true,
		},
		{
			Name: "volume without VolumeClaimName and VolumeClaim",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-volume",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
					Policies: []v1alpha1.LifecyclePolicy{
						{
							Event:  busv1alpha1.AnyEvent,
							Action: busv1alpha1.AbortJobAction,
						},
					},
					Volumes: []v1alpha1.VolumeSpec{
						{
							MountPath: "/var",
						},
						{
							MountPath: "/var",
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            " either VolumeClaim or VolumeClaimName must be specified;",
			ExpectErr:      true,
		},
		// task Policy with any event and other events
		{
			Name: "taskpolicy-withAnyandOthrEvent",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "taskpolicy-withAnyandOthrEvent",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
							Policies: []v1alpha1.LifecyclePolicy{
								{
									Event:  busv1alpha1.AnyEvent,
									Action: busv1alpha1.AbortJobAction,
								},
								{
									Event:  busv1alpha1.PodFailedEvent,
									Action: busv1alpha1.RestartJobAction,
								},
							},
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "if there's * here, no other policy should be here",
			ExpectErr:      true,
		},
		// job with no queue created
		{
			Name: "job-with-noQueue",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job-with-noQueue",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "jobQueue",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
										},
									},
								},
							},
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "unable to find job queue",
			ExpectErr:      true,
		},
		{
			Name: "job with priviledged container",
			Job: v1alpha1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-Job",
					Namespace: namespace,
				},
				Spec: v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task-1",
							Replicas: 1,
							Template: v1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"name": "test"},
								},
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:  "fake-name",
											Image: "busybox:1.24",
											SecurityContext: &v1.SecurityContext{
												Privileged: &priviledged,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			reviewResponse: v1beta1.AdmissionResponse{Allowed: true},
			ret:            "",
			ExpectErr:      false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			defaultqueue := schedulingv1beta2.Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: schedulingv1beta2.QueueSpec{
					Weight: 1,
				},
				Status: schedulingv1beta2.QueueStatus{
					State: schedulingv1beta2.QueueStateOpen,
				},
			}
			// create fake volcano clientset
			config.VolcanoClient = fakeclient.NewSimpleClientset()

			//create default queue
			_, err := config.VolcanoClient.SchedulingV1beta1().Queues().Create(context.TODO(), &defaultqueue, metav1.CreateOptions{})
			if err != nil {
				t.Error("Queue Creation Failed")
			}

			ret := validateJobCreate(&testCase.Job, &testCase.reviewResponse)
			//fmt.Printf("test-case name:%s, ret:%v  testCase.reviewResponse:%v \n", testCase.Name, ret,testCase.reviewResponse)
			if testCase.ExpectErr == true && ret == "" {
				t.Errorf("Expect error msg :%s, but got nil.", testCase.ret)
			}
			if testCase.ExpectErr == true && testCase.reviewResponse.Allowed != false {
				t.Errorf("Expect Allowed as false but got true.")
			}
			if testCase.ExpectErr == true && !strings.Contains(ret, testCase.ret) {
				t.Errorf("Expect error msg :%s, but got diff error %v", testCase.ret, ret)
			}

			if testCase.ExpectErr == false && ret != "" {
				t.Errorf("Expect no error, but got error %v", ret)
			}
			if testCase.ExpectErr == false && testCase.reviewResponse.Allowed != true {
				t.Errorf("Expect Allowed as true but got false. %v", testCase.reviewResponse)
			}
		})
	}
}

func TestValidateJobUpdate(t *testing.T) {
	testCases := []struct {
		name           string
		replicas       int32
		minAvailable   int32
		addTask        bool
		mutateTaskName bool
		mutateSpec     bool
		expectErr      bool
	}{
		{
			name:           "scale up",
			replicas:       6,
			minAvailable:   5,
			addTask:        false,
			mutateTaskName: false,
			mutateSpec:     false,
			expectErr:      false,
		},
		{
			name:           "invalid scale down with replicas less than minAvailable",
			replicas:       4,
			minAvailable:   5,
			addTask:        false,
			mutateTaskName: false,
			mutateSpec:     false,
			expectErr:      true,
		},
		{
			name:           "scale down",
			replicas:       4,
			minAvailable:   3,
			addTask:        false,
			mutateTaskName: false,
			mutateSpec:     false,
			expectErr:      false,
		},
		{
			name:           "invalid minAvailable",
			replicas:       4,
			minAvailable:   -1,
			addTask:        false,
			mutateTaskName: false,
			mutateSpec:     false,
			expectErr:      true,
		},
		{
			name:           "invalid add task",
			replicas:       4,
			minAvailable:   5,
			addTask:        true,
			mutateTaskName: false,
			mutateSpec:     false,
			expectErr:      true,
		},
		{
			name:           "invalid mutate task's fields other than replicas",
			replicas:       5,
			minAvailable:   5,
			addTask:        false,
			mutateTaskName: true,
			mutateSpec:     false,
			expectErr:      true,
		},
		{
			name:           "invalid mutate job's spec other than minAvailable",
			replicas:       5,
			minAvailable:   5,
			addTask:        false,
			mutateTaskName: false,
			mutateSpec:     true,
			expectErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			old := newJob()
			new := newJob()
			new.ResourceVersion = "502593"
			new.Status.Succeeded = 2

			new.Spec.MinAvailable = tc.minAvailable
			new.Spec.Tasks[0].Replicas = tc.replicas

			if tc.addTask {
				new.Spec.Tasks = append(new.Spec.Tasks, v1alpha1.TaskSpec{
					Name:     "task-2",
					Replicas: 5,
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"name": "test"},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "fake-name",
									Image: "busybox:1.24",
								},
							},
						},
					},
				})
			}
			if tc.mutateTaskName {
				new.Spec.Tasks[0].Name = "mutated-name"
			}
			if tc.mutateSpec {
				new.Spec.Queue = "mutated-queue"
			}

			err := validateJobUpdate(old, new)
			if err != nil && !tc.expectErr {
				t.Errorf("Expected no error, but got: %v", err)
			}
			if err == nil && tc.expectErr {
				t.Errorf("Expected error, but got none")
			}
		})
	}

}

func newJob() *v1alpha1.Job {
	return &v1alpha1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-Job",
			Namespace: "default",
		},
		Spec: v1alpha1.JobSpec{
			MinAvailable: 5,
			Queue:        "default",
			Tasks: []v1alpha1.TaskSpec{
				{
					Name:     "task-1",
					Replicas: 5,
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"name": "test"},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "fake-name",
									Image: "busybox:1.24",
								},
							},
						},
					},
				},
			},
			Policies: []v1alpha1.LifecyclePolicy{
				{
					Event:  busv1alpha1.PodEvictedEvent,
					Action: busv1alpha1.RestartTaskAction,
				},
			},
		},
	}
}
