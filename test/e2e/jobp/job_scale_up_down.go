/*
Copyright 2020 The Volcano Authors.

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

package jobp

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"pkg.yezhisheng.me/volcano/pkg/controllers/job/plugins/svc"
)

var _ = Describe("Dynamic Job scale up and down", func() {
	It("Scale up", func() {
		By("init test ctx")
		ctx := initTestContext(options{})
		defer cleanupTestContext(ctx)

		jobName := "scale-up-job"
		By("create job")
		job := createJob(ctx, &jobSpec{
			name: jobName,
			plugins: map[string][]string{
				"svc": {},
			},
			tasks: []taskSpec{
				{
					name: "default",
					img:  defaultNginxImage,
					min:  2,
					rep:  2,
					req:  halfCPU,
				},
			},
		})

		// job phase: pending -> running
		err := waitJobReady(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// scale up
		job.Spec.MinAvailable = 4
		job.Spec.Tasks[0].Replicas = 4
		err = updateJob(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// wait for tasks scaled up
		err = waitJobReady(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// check configmap updated
		pluginName := fmt.Sprintf("%s-svc", jobName)
		cm, err := ctx.kubeclient.CoreV1().ConfigMaps(ctx.namespace).Get(context.TODO(),
			pluginName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		hosts := svc.GenerateHosts(job)
		Expect(hosts).To(Equal(cm.Data))

		// TODO: check others

		By("delete job")
		err = ctx.vcclient.BatchV1alpha1().Jobs(job.Namespace).Delete(context.TODO(), job.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		err = waitJobCleanedUp(ctx, job)
		Expect(err).NotTo(HaveOccurred())

	})

	It("Scale down", func() {
		By("init test ctx")
		ctx := initTestContext(options{})
		defer cleanupTestContext(ctx)

		jobName := "scale-down-job"
		By("create job")
		job := createJob(ctx, &jobSpec{
			name: jobName,
			plugins: map[string][]string{
				"svc": {},
			},
			tasks: []taskSpec{
				{
					name: "default",
					img:  defaultNginxImage,
					min:  2,
					rep:  2,
					req:  halfCPU,
				},
			},
		})

		// job phase: pending -> running
		err := waitJobReady(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// scale down
		job.Spec.MinAvailable = 1
		job.Spec.Tasks[0].Replicas = 1
		err = updateJob(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// wait for tasks scaled up
		err = waitJobReady(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// check configmap updated
		pluginName := fmt.Sprintf("%s-svc", jobName)
		cm, err := ctx.kubeclient.CoreV1().ConfigMaps(ctx.namespace).Get(context.TODO(),
			pluginName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		hosts := svc.GenerateHosts(job)
		Expect(hosts).To(Equal(cm.Data))

		// TODO: check others

		By("delete job")
		err = ctx.vcclient.BatchV1alpha1().Jobs(job.Namespace).Delete(context.TODO(), job.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		err = waitJobCleanedUp(ctx, job)
		Expect(err).NotTo(HaveOccurred())

	})

	It("Scale down to zero and scale up", func() {
		By("init test ctx")
		ctx := initTestContext(options{})
		defer cleanupTestContext(ctx)

		jobName := "scale-down-job"
		By("create job")
		job := createJob(ctx, &jobSpec{
			name: jobName,
			plugins: map[string][]string{
				"svc": {},
			},
			tasks: []taskSpec{
				{
					name: "default",
					img:  defaultNginxImage,
					min:  2,
					rep:  2,
					req:  halfCPU,
				},
			},
		})

		// job phase: pending -> running
		err := waitJobReady(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// scale down
		job.Spec.MinAvailable = 0
		job.Spec.Tasks[0].Replicas = 0
		err = updateJob(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// wait for tasks scaled up
		err = waitJobReady(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// check configmap updated
		pluginName := fmt.Sprintf("%s-svc", jobName)
		cm, err := ctx.kubeclient.CoreV1().ConfigMaps(ctx.namespace).Get(context.TODO(),
			pluginName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		hosts := svc.GenerateHosts(job)
		Expect(hosts).To(Equal(cm.Data))

		// scale up
		job.Spec.MinAvailable = 2
		job.Spec.Tasks[0].Replicas = 2
		err = updateJob(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// wait for tasks scaled up
		err = waitJobReady(ctx, job)
		Expect(err).NotTo(HaveOccurred())

		// check configmap updated
		cm, err = ctx.kubeclient.CoreV1().ConfigMaps(ctx.namespace).Get(context.TODO(),
			pluginName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		hosts = svc.GenerateHosts(job)
		Expect(hosts).To(Equal(cm.Data))

		// TODO: check others

		By("delete job")
		err = ctx.vcclient.BatchV1alpha1().Jobs(job.Namespace).Delete(context.TODO(), job.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		err = waitJobCleanedUp(ctx, job)
		Expect(err).NotTo(HaveOccurred())
	})
})
