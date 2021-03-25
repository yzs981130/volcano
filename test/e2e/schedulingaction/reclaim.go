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

package schedulingaction

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	batchv1alpha1 "pkg.yezhisheng.me/volcano/pkg/apis/batch/v1alpha1"
	schedulingv1beta1 "pkg.yezhisheng.me/volcano/pkg/apis/scheduling/v1beta1"
)

var _ = Describe("Reclaim E2E Test", func() {

	CreateReclaimJob := func(ctx *testContext, req v1.ResourceList, name string, queue string, pri string, nodeName string, waitTaskReady bool) (*batchv1alpha1.Job, error) {
		job := &jobSpec{
			tasks: []taskSpec{
				{
					img: defaultNginxImage,
					req: req,
					min: 1,
					rep: 1,
				},
			},
			name:     name,
			queue:    queue,
			nodeName: nodeName,
		}
		if pri != "" {
			job.pri = pri
		}
		batchJob, err := createJobInner(ctx, job)
		if err != nil {
			return nil, err
		}
		if waitTaskReady {
			err = waitTasksReady(ctx, batchJob, 1)
		}
		return batchJob, err
	}

	WaitQueueStatus := func(ctx *testContext, status string, num int32, queue string) error {
		err := waitQueueStatus(func() (bool, error) {
			queue, err := ctx.vcclient.SchedulingV1beta1().Queues().Get(context.TODO(), queue, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Get queue %s failed", queue)
			switch status {
			case "Running":
				return queue.Status.Running == num, nil
			case "Open":
				return queue.Status.State == schedulingv1beta1.QueueStateOpen, nil
			case "Pending":
				return queue.Status.Pending == num, nil
			case "Inqueue":
				return queue.Status.Inqueue == num, nil
			default:
				return false, nil
			}
		})
		return err
	}

	It("Reclaim Case 1: New queue with job created no reclaim when resource is enough", func() {
		q1 := defaultQueue
		q2 := "reclaim-q2"
		ctx := initTestContext(options{
			queues:             []string{q2},
			nodesNumLimit:      4,
			nodesResourceLimit: CPU1Mem1,
		})

		defer cleanupTestContext(ctx)

		By("Setup initial jobs")

		_, err := CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j1", q1, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job1 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j2", q2, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job2 failed")

		By("Create new coming queue and job")
		q3 := "reclaim-q3"
		ctx.queues = append(ctx.queues, q3)
		createQueues(ctx)

		err = WaitQueueStatus(ctx, "Open", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue open")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j3", q3, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job3 failed")

		By("Make sure all job running")

		err = WaitQueueStatus(ctx, "Running", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q2)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q3)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

	})

	It("Reclaim Case 3: New queue with job created no reclaim when job.podGroup.Status.Phase pending", func() {
		q1 := defaultQueue
		q2 := "reclaim-q2"
		j1 := "reclaim-j1"
		j2 := "reclaim-j2"
		j3 := "reclaim-j3"

		ctx := initTestContext(options{
			queues:             []string{q2},
			nodesNumLimit:      3,
			nodesResourceLimit: CPU1Mem1,
			priorityClasses: map[string]int32{
				"low-priority":  10,
				"high-priority": 10000,
			},
		})

		defer cleanupTestContext(ctx)

		By("Setup initial jobs")

		_, err := CreateReclaimJob(ctx, CPU1Mem1, j1, q1, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job1 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, j2, q2, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job2 failed")

		By("Create new coming queue and job")
		q3 := "reclaim-q3"
		ctx.queues = append(ctx.queues, q3)
		createQueues(ctx)

		err = WaitQueueStatus(ctx, "Open", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue open")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, j3, q3, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job3 failed")

		// delete pod of job3 to make sure reclaim-j3 podgroup is pending
		listOptions := metav1.ListOptions{
			LabelSelector: labels.Set(map[string]string{batchv1alpha1.JobNameKey: j3}).String(),
		}

		job3pods, err := ctx.kubeclient.CoreV1().Pods(ctx.namespace).List(context.TODO(), listOptions)
		Expect(err).NotTo(HaveOccurred(), "Get %s pod failed", j3)

		By("Make sure q1 q2 with job running in it.")
		err = WaitQueueStatus(ctx, "Running", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q2)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		for _, pod := range job3pods.Items {
			fmt.Println(pod.Name)
			err = ctx.kubeclient.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %s", pod.Name)
		}

		By("Q3 pending when we delete it.")
		err = WaitQueueStatus(ctx, "Pending", 1, q3)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue pending")
	})

	It("Reclaim Case 4: New queue with job created no reclaim when new queue is not created", func() {
		q1 := defaultQueue
		q2 := "reclaim-q2"
		ctx := initTestContext(options{
			queues:             []string{q2},
			nodesNumLimit:      3,
			nodesResourceLimit: CPU1Mem1,
			priorityClasses: map[string]int32{
				"low-priority":  10,
				"high-priority": 10000,
			},
		})

		defer cleanupTestContext(ctx)

		By("Setup initial jobs")

		_, err := CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j1", q1, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job1 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j2", q2, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job2 failed")

		By("Create new coming job")
		q3 := "reclaim-q3"

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j3", q3, "", "", true)
		Expect(err).Should(HaveOccurred(), "job3 create failed when queue3 is not created")

		By("Make sure all job running")

		err = WaitQueueStatus(ctx, "Running", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q2)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")
	})

	// As we agreed, this is not intended behavior, actually, it is a bug.
	It("Reclaim Case 5: New queue with job created no reclaim when job or task is low-priority", func() {
		q1 := defaultQueue
		q2 := "reclaim-q2"
		ctx := initTestContext(options{
			queues:             []string{q2},
			nodesNumLimit:      3,
			nodesResourceLimit: CPU1Mem1,
			priorityClasses: map[string]int32{
				"low-priority":  10,
				"high-priority": 10000,
			},
		})

		defer cleanupTestContext(ctx)

		By("Setup initial jobs")

		_, err := CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j1", q1, "high-priority", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job1 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j2", q2, "high-priority", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job2 failed")

		By("Create new coming queue and job")
		q3 := "reclaim-q3"

		err = WaitQueueStatus(ctx, "Open", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue open")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j3", q3, "low-priority", "", true)
		Expect(err).Should(HaveOccurred(), "job3 create failed when queue3 is not created")

		By("Make sure all job running")

		err = WaitQueueStatus(ctx, "Running", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q2)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")
	})

	It("Reclaim Case 6: New queue with job created no reclaim when overused", func() {
		q1 := defaultQueue
		q2 := "reclaim-q2"
		q3 := "reclaim-q3"
		ctx := initTestContext(options{
			queues:             []string{q2, q3},
			nodesNumLimit:      3,
			nodesResourceLimit: CPU1Mem1,
			priorityClasses: map[string]int32{
				"low-priority":  10,
				"high-priority": 10000,
			},
		})

		defer cleanupTestContext(ctx)

		By("Setup initial jobs")

		_, err := CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j1", q1, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job1 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j2", q2, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job2 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j3", q3, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job3 failed")

		By("Create job4 to testing overused cases.")
		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j4", q3, "", "", false)
		Expect(err).NotTo(HaveOccurred(), "Wait for job4 failed")

		By("Make sure all job running")

		err = WaitQueueStatus(ctx, "Running", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q2)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q3)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Pending", 1, q3)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue pending")
	})

	It("Reclaim Case 7:  New queue with job created no reclaim when job not satisfied with predicates", func() {
		q1 := defaultQueue
		q2 := "reclaim-q2"
		ctx := initTestContext(options{
			queues:             []string{q2},
			nodesNumLimit:      3,
			nodesResourceLimit: CPU1Mem1,
		})

		defer cleanupTestContext(ctx)

		By("Setup initial jobs")

		_, err := CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j1", q1, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job1 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j2", q2, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job2 failed")

		By("Create new coming queue and job")
		q3 := "reclaim-q3"
		ctx.queues = append(ctx.queues, q3)
		createQueues(ctx)

		err = WaitQueueStatus(ctx, "Open", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue open")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j3", q3, "", "fake-node", false)
		Expect(err).NotTo(HaveOccurred(), "Wait for job3 failed")

		By("Make sure all job running")

		err = WaitQueueStatus(ctx, "Running", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q2)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Pending", 1, q3)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue pending")

	})

	It("Reclaim Case 8: New queue with job created no reclaim when task resources less than reclaimable resource", func() {
		q1 := defaultQueue
		q2 := "reclaim-q2"
		ctx := initTestContext(options{
			queues:             []string{q2},
			nodesNumLimit:      3,
			nodesResourceLimit: CPU1Mem1,
			priorityClasses: map[string]int32{
				"low-priority":  10,
				"high-priority": 10000,
			},
		})

		defer cleanupTestContext(ctx)

		By("Setup initial jobs")

		_, err := CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j1", q1, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job1 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j2", q2, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job2 failed")

		By("Create new coming queue and job")
		q3 := "reclaim-q3"
		ctx.queues = append(ctx.queues, q3)
		createQueues(ctx)

		err = WaitQueueStatus(ctx, "Open", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue open")

		job := &jobSpec{
			tasks: []taskSpec{
				{
					img: defaultNginxImage,
					req: CPU4Mem4,
					min: 1,
					rep: 1,
				},
			},
			name:  "reclaim-j4",
			queue: q3,
		}
		createJob(ctx, job)

		By("Make sure all job running")

		err = WaitQueueStatus(ctx, "Running", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q2)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Pending", 1, q3)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")
	})

	It("Reclaim Case 9:  New queue with job created, all queues.spec.reclaimable is false, no reclaim", func() {
		q1 := defaultQueue
		q2 := "reclaim-q2"
		ctx := initTestContext(options{
			queues:             []string{q2},
			nodesNumLimit:      3,
			nodesResourceLimit: CPU1Mem1,
		})

		defer cleanupTestContext(ctx)

		By("Setup initial jobs")

		_, err := CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j1", q1, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job1 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j2", q2, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job2 failed")

		By("Create new coming queue and job")
		q3 := "reclaim-q3"
		ctx.queues = append(ctx.queues, q3)
		createQueues(ctx)

		setQueueReclaimable(ctx, []string{q1, q2}, false)
		defer setQueueReclaimable(ctx, []string{q1}, true)

		err = WaitQueueStatus(ctx, "Open", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue open")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j3", q3, "", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job3 failed")

		By("Make sure all job running")

		err = WaitQueueStatus(ctx, "Running", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q2)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q3)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

	})

	// Reclaim rely on priority is a bug here.
	It("Reclaim Case 10: Multi reclaimed queue", func() {
		q1 := defaultQueue
		q2 := "reclaim-q2"
		q3 := "reclaim-q3"
		q4 := "reclaim-q4"
		ctx := initTestContext(options{
			queues:             []string{q2, q3, q4},
			nodesNumLimit:      4,
			nodesResourceLimit: CPU1Mem1,
			priorityClasses: map[string]int32{
				"low-priority":  10,
				"high-priority": 10000,
			},
		})

		defer cleanupTestContext(ctx)

		By("Setup initial jobs")

		_, err := CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j1", q1, "low-priority", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job1 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j2", q1, "low-priority", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job2 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j3", q2, "low-priority", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job3 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j4", q2, "low-priority", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job4 failed")

		By("Create coming jobs")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j5", q3, "high-priority", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job4 failed")

		_, err = CreateReclaimJob(ctx, CPU1Mem1, "reclaim-j6", q4, "high-priority", "", true)
		Expect(err).NotTo(HaveOccurred(), "Wait for job4 failed")

		By("Make sure all job running")

		err = WaitQueueStatus(ctx, "Running", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Inqueue", 1, q1)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q2)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Inqueue", 1, q2)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q3)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

		err = WaitQueueStatus(ctx, "Running", 1, q4)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for queue running")

	})

	It("Reclaim", func() {
		q1, q2 := "reclaim-q1", "reclaim-q2"
		ctx := initTestContext(options{
			queues: []string{q1, q2},
			priorityClasses: map[string]int32{
				"low-priority":  10,
				"high-priority": 10000,
			},
		})
		defer cleanupTestContext(ctx)

		slot := oneCPU
		rep := clusterSize(ctx, slot)

		spec := &jobSpec{
			tasks: []taskSpec{
				{
					img: defaultNginxImage,
					req: slot,
					min: 1,
					rep: rep,
				},
			},
		}

		spec.name = "q1-qj-1"
		spec.queue = q1
		spec.pri = "low-priority"
		job1 := createJob(ctx, spec)
		err := waitJobReady(ctx, job1)
		Expect(err).NotTo(HaveOccurred())

		err = waitQueueStatus(func() (bool, error) {
			queue, err := ctx.vcclient.SchedulingV1beta1().Queues().Get(context.TODO(), q1, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return queue.Status.Running == 1, nil
		})
		Expect(err).NotTo(HaveOccurred())

		expected := int(rep) / 2
		// Reduce one pod to tolerate decimal fraction.
		if expected > 1 {
			expected--
		} else {
			err := fmt.Errorf("expected replica <%d> is too small", expected)
			Expect(err).NotTo(HaveOccurred())
		}

		spec.name = "q2-qj-2"
		spec.queue = q2
		spec.pri = "high-priority"
		job2 := createJob(ctx, spec)
		err = waitTasksReady(ctx, job2, expected)
		Expect(err).NotTo(HaveOccurred())

		err = waitTasksReady(ctx, job1, expected)
		Expect(err).NotTo(HaveOccurred())

		// Test Queue status
		spec = &jobSpec{
			name:  "q1-qj-2",
			queue: q1,
			tasks: []taskSpec{
				{
					img: defaultNginxImage,
					req: slot,
					min: rep * 2,
					rep: rep * 2,
				},
			},
		}
		job3 := createJob(ctx, spec)
		err = waitJobStatePending(ctx, job3)
		Expect(err).NotTo(HaveOccurred())
		err = waitQueueStatus(func() (bool, error) {
			queue, err := ctx.vcclient.SchedulingV1beta1().Queues().Get(context.TODO(), q1, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return queue.Status.Pending == 1, nil
		})
		Expect(err).NotTo(HaveOccurred())
	})
})
