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

package router

import (
	"k8s.io/api/admission/v1beta1"
	whv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/client-go/kubernetes"

	"pkg.yezhisheng.me/volcano/pkg/client/clientset/versioned"
)

//The AdmitFunc returns response.
type AdmitFunc func(v1beta1.AdmissionReview) *v1beta1.AdmissionResponse

type AdmissionServiceConfig struct {
	SchedulerName string
	KubeClient    kubernetes.Interface
	VolcanoClient versioned.Interface
}

type AdmissionService struct {
	Path    string
	Func    AdmitFunc
	Handler AdmissionHandler

	ValidatingConfig *whv1beta1.ValidatingWebhookConfiguration
	MutatingConfig   *whv1beta1.MutatingWebhookConfiguration

	Config *AdmissionServiceConfig
}
