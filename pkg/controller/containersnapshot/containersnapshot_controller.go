package containersnapshot

import (
	"context"
	"os"

	atomv1alpha1 "github.com/supremind/container-snapshot/pkg/apis/atom/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	labelKeyPrefix              = "container-snapshot.atom.supremind.com/"
	imagePushSecretPath         = "/root/.docker"
	dockerSocketPath            = "/var/run/docker.sock"
	envKeyWorkerImage           = "WORKER_IMAGE"
	envKeyWorkerImagePullSecret = "WORKER_IMAGE_PULL_SECRET"
)

var hostPathSocket = corev1.HostPathSocket

var log = logf.Log.WithName("controller_containersnapshot")

// Add creates a new ContainerSnapshot Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileContainerSnapshot{
		client:                mgr.GetClient(),
		scheme:                mgr.GetScheme(),
		workerImage:           os.Getenv(envKeyWorkerImage),
		workerImagePullSecret: os.Getenv(envKeyWorkerImagePullSecret),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("containersnapshot-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ContainerSnapshot
	err = c.Watch(&source.Kind{Type: &atomv1alpha1.ContainerSnapshot{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner ContainerSnapshot
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &atomv1alpha1.ContainerSnapshot{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileContainerSnapshot implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileContainerSnapshot{}

// ReconcileContainerSnapshot reconciles a ContainerSnapshot object
type ReconcileContainerSnapshot struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client                client.Client
	scheme                *runtime.Scheme
	workerImage           string
	workerImagePullSecret string
}

// Reconcile reads that state of the cluster for a ContainerSnapshot object and makes changes based on the state read
// and what is in the ContainerSnapshot.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileContainerSnapshot) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ContainerSnapshot")

	// Fetch the ContainerSnapshot instance
	instance := &atomv1alpha1.ContainerSnapshot{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// todo: check condition to make different decisions on creation, updating, deletion

	// Define a new Pod object
	pod := r.newPodForCR(instance)

	// Set ContainerSnapshot instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue

		// todo: update snapshot condition

		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Pod already exists - don't requeue
	reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return reconcile.Result{}, nil
}

// newPodForCR returns a pod with the same name/namespace as the cr
func (r *ReconcileContainerSnapshot) newPodForCR(cr *atomv1alpha1.ContainerSnapshot) *corev1.Pod {
	labels := map[string]string{
		labelKeyPrefix + "snapshot":  cr.Name,
		labelKeyPrefix + "pod":       cr.Spec.PodName,
		labelKeyPrefix + "container": cr.Spec.ContainerName,
		labelKeyPrefix + "image":     cr.Spec.Image,
	}
	for k, v := range cr.Labels {
		labels[k] = v
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cr.Name + "-",
			Namespace:    cr.Namespace,
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{{
				Name: r.workerImagePullSecret,
			}},
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:  "snapshot-worker",
				Image: r.workerImage,
				// Command: []string{"sleep", "3600"},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "image-push-secret",
						MountPath: imagePushSecretPath,
						ReadOnly:  true,
					},
					{
						Name:      "docker-socket",
						MountPath: dockerSocketPath,
					},
				},
			}},
			NodeName: cr.Status.NodeName,
			Volumes: []corev1.Volume{
				{
					Name: "image-push-secret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: cr.Spec.ImagePushSecret.Name,
						},
					},
				},
				{
					Name: "docker-socket",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: dockerSocketPath,
							Type: &hostPathSocket,
						},
					},
				},
			},
		},
	}
}
