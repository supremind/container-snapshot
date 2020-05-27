package containersnapshot

import (
	"context"
	"testing"
	"time"

	atomv1alpha1 "github.com/supremind/container-snapshot/pkg/apis/atom/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestContainerSnapshot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Containersnapshot Suite")
}

var _ = Describe("snapshot operator", func() {
	var (
		namespace = "example-ns"
		snpKey    = types.NamespacedName{Name: "example-snapshot", Namespace: namespace}
		now       = metav1.Now()
		ctx       = context.Background()
		re        = &ReconcileContainerSnapshot{
			workerImage:           "worker-image:latest",
			workerImagePullSecret: "worker-image-pull-secret",
			workerServiceAccount:  "container-snapshot-worker",
		}
		simpleSnapshot *atomv1alpha1.ContainerSnapshot
		sourcePod      *corev1.Pod
	)

	BeforeEach(func() {
		simpleSnapshot = &atomv1alpha1.ContainerSnapshot{
			ObjectMeta: metav1.ObjectMeta{Name: "example-snapshot", Namespace: namespace},
			Spec: atomv1alpha1.ContainerSnapshotSpec{
				PodName:       "source-pod",
				ContainerName: "source-container",
				Image:         "reg.example.com/snapshots/example-snapshot:v0.0.1",
				ImagePushSecrets: []corev1.LocalObjectReference{{
					Name: "my-docker-secret",
				}},
			},
		}
		sourcePod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "source-pod", Namespace: namespace},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "source-container",
						Image: "source-image:latest",
					},
					{
						Name:  "sidecar-container",
						Image: "sidecar-image:latest",
					},
				},
				NodeName: "example-node",
			},
			Status: corev1.PodStatus{
				Phase:     corev1.PodRunning,
				StartTime: &metav1.Time{Time: now.Add(-1 * time.Minute)},
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "source-container",
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{
								StartedAt: metav1.Time{Time: now.Add(-1 * time.Minute)},
							},
						},
						Ready:       true,
						Image:       "source-image:latest",
						ImageID:     "docker-pullable:///source-image@sha256:xxxx-source-image",
						ContainerID: "docker://xxxx-source-image",
					},
					{
						Name: "sidecar-container",
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{
								StartedAt: metav1.Time{Time: now.Add(-1 * time.Minute)},
							},
						},
						Ready:       true,
						Image:       "sidecar-image:latest",
						ImageID:     "docker-pullable:///sidecar-image@sha256:xxxx-sidecar-image",
						ContainerID: "docker://xxxx-sidecar-image",
					},
				},
			},
		}

		// Register operator types with the runtime scheme.
		re.scheme = scheme.Scheme
		re.scheme.AddKnownTypes(atomv1alpha1.SchemeGroupVersion, simpleSnapshot)
		// Create a fake client to mock API calls.
		re.client = &indexFakeClient{fake.NewFakeClientWithScheme(re.scheme)}
	})

	Context("creating snapshot", func() {
		var uid types.UID
		JustBeforeEach(func() {
			Expect(re.client.Create(ctx, sourcePod)).Should(Succeed())
			Expect(re.client.Create(ctx, simpleSnapshot)).Should(Succeed())

			snp, e := getSnapshot(ctx, re.client, snpKey)
			Expect(e).Should(Succeed())
			uid = snp.UID
		})

		Context("for running source pod", func() {
			It("should succeed, and create a worker pod", func() {
				Expect(re.Reconcile(reconcile.Request{NamespacedName: snpKey})).Should(Equal(reconcile.Result{}))
				Expect(getWorkerState(ctx, re.client, snpKey)).Should(Equal(atomv1alpha1.WorkerCreated))
				out, e := re.getWorkerPod(ctx, namespace, uid)

				Expect(e).Should(BeNil())
				Expect(out).ShouldNot(BeNil())

				Expect(out.Name).Should(HavePrefix("example-snapshot-"))
				Expect(out.Namespace).Should(Equal(namespace))
				Expect(out.Spec.Containers).Should(HaveLen(1))
				Expect(out.Spec.NodeName).Should(Equal("example-node"))
				container := out.Spec.Containers[0]
				Expect(container.Image).Should(Equal(re.workerImage))
				Expect(container.Command).Should(Equal([]string{"container-snapshot-worker"}))
				Expect(container.Args).Should(Equal([]string{
					"--container", "xxxx-source-image",
					"--image", "reg.example.com/snapshots/example-snapshot:v0.0.1",
					"--snapshot", "example-snapshot",
				}))
			})
		})

		Context("for failed source pod", func() {
			BeforeEach(func() {
				sourcePod.Status.Phase = corev1.PodSucceeded
			})

			It("should fail, and not create any worker pod", func() {
				Expect(func() error {
					_, e := re.Reconcile(reconcile.Request{NamespacedName: snpKey})
					return e
				}()).ShouldNot(Succeed())
				Expect(getWorkerState(ctx, re.client, snpKey)).Should(Equal(atomv1alpha1.WorkerFailed))

				Consistently(func() error {
					_, e := re.getWorkerPod(ctx, namespace, uid)
					return e
				}()).Should(HaveOccurred())
			})
		})
	})

	Context("updating snapshot", func() {
		var worker *corev1.Pod

		BeforeEach(func() {
			Expect(re.client.Create(ctx, sourcePod)).Should(Succeed())
			Expect(re.client.Create(ctx, simpleSnapshot)).Should(Succeed())
			Expect(re.Reconcile(reconcile.Request{NamespacedName: snpKey})).Should(Equal(reconcile.Result{}))

			snp, e := getSnapshot(ctx, re.client, snpKey)
			Expect(e).Should(Succeed())
			worker, e = re.getWorkerPod(ctx, namespace, snp.UID)
			Expect(e).Should(Succeed())
		})

		JustBeforeEach(func() {
			Expect(re.client.Status().Update(ctx, worker)).Should(Succeed())
		})

		Context("when worker is running", func() {
			BeforeEach(func() {
				worker.Status.Phase = corev1.PodRunning
			})

			It("should update snapshot's workerState to running", func() {
				Expect(re.Reconcile(reconcile.Request{NamespacedName: snpKey})).Should(Equal(reconcile.Result{}))
				Expect(getWorkerState(ctx, re.client, snpKey)).Should(Equal(atomv1alpha1.WorkerRunning))
			})
		})

		Context("when worker succeeds", func() {
			BeforeEach(func() {
				worker.Status.Phase = corev1.PodSucceeded
			})

			It("should update snapshot's workerState to complete", func() {
				Expect(re.Reconcile(reconcile.Request{NamespacedName: snpKey})).Should(Equal(reconcile.Result{}))
				Expect(getWorkerState(ctx, re.client, snpKey)).Should(Equal(atomv1alpha1.WorkerComplete))
			})
		})

		Context("when worker fails", func() {
			BeforeEach(func() {
				worker.Status.Phase = corev1.PodFailed
				worker.Status.ContainerStatuses = []corev1.ContainerStatus{{
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode:   1,
							Reason:     string(atomv1alpha1.DockerCommitFailed),
							Message:    "docker commit failed: blah, blah...",
							FinishedAt: metav1.Time{Time: now.Add(1 * time.Minute)},
						},
					},
				}}
			})

			It("should update snapshot's workerState to failed, and collect condition", func() {
				Expect(re.Reconcile(reconcile.Request{NamespacedName: snpKey})).Should(Equal(reconcile.Result{}))
				snp, e := getSnapshot(ctx, re.client, snpKey)
				Expect(e).Should(Succeed())
				Expect(snp.Status.Conditions).Should(HaveLen(1))
				Expect(snp.Status.Conditions[0].Type).Should(Equal(atomv1alpha1.DockerCommitFailed))
			})
		})
	})

	Context("deleting snapshot", func() {
		var uid types.UID
		BeforeEach(func() {
			Expect(re.client.Create(ctx, sourcePod)).Should(Succeed())
			Expect(re.client.Create(ctx, simpleSnapshot)).Should(Succeed())
			Expect(re.Reconcile(reconcile.Request{NamespacedName: snpKey})).Should(Equal(reconcile.Result{}))

			snp, e := getSnapshot(ctx, re.client, snpKey)
			Expect(e).Should(Succeed())
			uid = snp.UID
		})

		JustBeforeEach(func() {
			Eventually(func() error { _, e := re.getWorkerPod(ctx, namespace, uid); return e }).Should(Succeed())
			Expect(re.client.Delete(ctx, simpleSnapshot, client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
			Expect(re.Reconcile(reconcile.Request{NamespacedName: snpKey})).Should(Equal(reconcile.Result{}))
		})

		It("should succeed, and delete the worker pod", func() {
			Eventually(func() error { _, e := getSnapshot(ctx, re.client, snpKey); return e }).Should(HaveOccurred())
		})

		// skip it, fake client knows nothing about delete propagation
		PIt("should delete the worker pod", func() {
			Eventually(func() error { _, e := re.getWorkerPod(ctx, namespace, uid); return e }).Should(HaveOccurred())
		})
	})
})

func getSnapshot(ctx context.Context, c client.Client, key types.NamespacedName) (*atomv1alpha1.ContainerSnapshot, error) {
	snp := atomv1alpha1.ContainerSnapshot{}
	e := c.Get(ctx, key, &snp)
	if e != nil {
		return nil, e
	}

	return &snp, nil
}

func getWorkerState(ctx context.Context, c client.Client, key types.NamespacedName) (atomv1alpha1.WorkerState, error) {
	snp, e := getSnapshot(ctx, c, key)
	if e != nil {
		return "", e
	}
	return snp.Status.WorkerState, nil
}

// fake client does not index or fillter objects by owner references, make it do
type indexFakeClient struct {
	client.Client
}

func (c *indexFakeClient) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	e := c.Client.List(ctx, list, opts...)
	if e != nil {
		return e
	}

	listOpts := client.ListOptions{}
	listOpts.ApplyOptions(opts)
	if listOpts.FieldSelector == nil || listOpts.FieldSelector.Empty() {
		return nil
	}

	objs, e := apimeta.ExtractList(list)
	if e != nil {
		return e
	}

	out := make([]runtime.Object, 0)
	for _, obj := range objs {
		meta, e := apimeta.Accessor(obj)
		if e != nil {
			continue
		}

		for _, owner := range meta.GetOwnerReferences() {
			if listOpts.FieldSelector.Matches(fields.Set{
				"metadata.ownerReferences.uid": string(owner.UID),
			}) {
				out = append(out, obj)
				break
			}
		}
	}

	if len(out) == 0 {
		return errWorkerPodNotFound
	}

	return apimeta.SetList(list, out)
}
