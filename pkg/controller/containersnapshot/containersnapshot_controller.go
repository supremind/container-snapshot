package containersnapshot

import (
	"context"
	stderr "errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/operator-framework/operator-sdk/pkg/status"
	atomv1alpha1 "github.com/supremind/container-snapshot/pkg/apis/atom/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/utils/pointer"
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
	imagePushSecretPath         = "/config"
	dockerSocketPath            = "/var/run/docker.sock"
	containerIDPrefix           = "docker://"
	envKeyWorkerImage           = "WORKER_IMAGE"
	envKeyWorkerImagePullSecret = "WORKER_IMAGE_PULL_SECRET"
	envKeyWorkerServiceAccount  = "WORKER_SERVICE_ACCOUNT"
	requestTimeout              = 10 * time.Second
)

var (
	errSourcePodNotFound       = stderr.New("can not find source pod")
	errSourceContainerNotFound = stderr.New("can not find source container")
	errSourcePodNotReady       = stderr.New("source pod is not ready")
	errWorkerPodNotFound       = stderr.New("can not find worker pod")
	errTooManyWorkerPods       = stderr.New("find more than one worker pods")
)

var log = logf.Log.WithName("container snapshot operator")

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
		workerServiceAccount:  os.Getenv(envKeyWorkerServiceAccount),
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
	workerServiceAccount  string
}

// Reconcile reads that state of the cluster for a ContainerSnapshot object and makes changes based on the state read
// and what is in the ContainerSnapshot.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileContainerSnapshot) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ContainerSnapshot")

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	// Fetch the ContainerSnapshot instance
	instance := &atomv1alpha1.ContainerSnapshot{}
	err := r.client.Get(ctx, request.NamespacedName, instance)
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

	if !instance.DeletionTimestamp.IsZero() {
		// do nothing on deletion
		return reconcile.Result{}, nil
	}

	switch instance.Status.WorkerState {
	case atomv1alpha1.WorkerCreated, atomv1alpha1.WorkerRunning, atomv1alpha1.WorkerUnknown:
		return r.onUpdate(ctx, instance)
	case atomv1alpha1.WorkerFailed, atomv1alpha1.WorkerComplete:
		// do nothing
		return reconcile.Result{}, nil
	default:
		return r.onCreation(ctx, instance)
	}
}

func (r *ReconcileContainerSnapshot) onCreation(ctx context.Context, cr *atomv1alpha1.ContainerSnapshot) (reconcile.Result, error) {
	reqLogger := logger(cr)
	reqLogger.Info("on snapshot creation")

	// Check if this Pod already exists
	_, e := r.getWorkerPod(ctx, cr.Namespace, cr.UID)
	if e == nil {
		reqLogger.Info("worker pod already exists, update the snapshot if necessary")
		return r.onUpdate(ctx, cr)
	}
	if !errors.IsNotFound(e) && !stderr.Is(e, errWorkerPodNotFound) {
		reqLogger.Error(e, "check if worker pod already exists before creating new one")
		return reconcile.Result{}, e
	}

	stale := false
	defer func() {
		if stale {
			r.applyUpdate(ctx, cr)
		}
	}()

	nodeName, containerID, e := r.getSourceContainer(ctx, cr)
	if e != nil {
		if stderr.Is(e, errSourcePodNotFound) {
			stale = cr.Status.Conditions.SetCondition(status.Condition{
				Type:               atomv1alpha1.SourcePodNotFound,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			})
		} else if stderr.Is(e, errSourceContainerNotFound) {
			stale = cr.Status.Conditions.SetCondition(status.Condition{
				Type:               atomv1alpha1.SourceContainerNotFound,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			})
		} else if stderr.Is(e, errSourcePodNotReady) {
			stale = cr.Status.Conditions.SetCondition(status.Condition{
				Type:               atomv1alpha1.SourcePodNotReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			})
		}

		stale = true
		cr.Status.WorkerState = atomv1alpha1.WorkerFailed

		return reconcile.Result{}, e
	}

	stale = cr.Status.NodeName != nodeName || cr.Status.ContainerID != containerID
	cr.Status.NodeName = nodeName
	cr.Status.ContainerID = containerID

	// Define a new Pod object
	pod := r.newWorkerPod(cr)
	reqLogger = reqLogger.WithValues("pod namespace", pod.Namespace, "pod name", pod.Name)

	// Set ContainerSnapshot instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, pod, r.scheme); err != nil {
		reqLogger.Error(e, "set controller reference for worker pod")
		return reconcile.Result{Requeue: true}, nil
	}

	reqLogger.Info("Creating a new Pod")
	err := r.client.Create(ctx, pod)
	if err != nil {
		return reconcile.Result{}, err
	}

	stale = true
	cr.Status.WorkerState = atomv1alpha1.WorkerCreated

	return reconcile.Result{}, nil
}

func (r *ReconcileContainerSnapshot) onUpdate(ctx context.Context, cr *atomv1alpha1.ContainerSnapshot) (reconcile.Result, error) {
	reqLogger := logger(cr)
	reqLogger.Info("on snapshot updating")

	pod, e := r.getWorkerPod(ctx, cr.Namespace, cr.UID)
	if e != nil {
		return reconcile.Result{}, e
	}

	reqLogger = reqLogger.WithValues("pod name", pod.Name, "pod namespace", pod.Namespace)
	var state atomv1alpha1.WorkerState
	var cond *status.Condition

	switch pod.Status.Phase {
	case corev1.PodPending:
		state = atomv1alpha1.WorkerCreated
	case corev1.PodRunning:
		state = atomv1alpha1.WorkerRunning
	case corev1.PodSucceeded:
		state = atomv1alpha1.WorkerComplete
		cond = parseTerminationState(pod)
	case corev1.PodFailed:
		state = atomv1alpha1.WorkerFailed
		cond = parseTerminationState(pod)
	default:
		state = atomv1alpha1.WorkerUnknown
	}

	stale := false

	if cr.Status.WorkerState != state {
		stale = true
		reqLogger.Info("update snapshot worker state", "from", cr.Status.WorkerState, "to", state)
		cr.Status.WorkerState = state
	}
	if cond != nil {
		stale = true
		cr.Status.Conditions.SetCondition(*cond)
		reqLogger.Info("update snapshot condition", "type", cond.Type, "status", cond.Status)
	}
	if stale {
		return r.applyUpdate(ctx, cr)
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileContainerSnapshot) applyUpdate(ctx context.Context, cr *atomv1alpha1.ContainerSnapshot) (reconcile.Result, error) {
	e := r.client.Status().Update(ctx, cr)
	if e != nil {
		logger(cr).Error(e, "update snapshot worker state")
	}

	return reconcile.Result{}, e
}

func (r *ReconcileContainerSnapshot) getSourceContainer(ctx context.Context, cr *atomv1alpha1.ContainerSnapshot) (nodeName, containerID string, e error) {
	reqLogger := logger(cr)

	pod := &corev1.Pod{}
	e = r.client.Get(ctx, types.NamespacedName{Namespace: cr.Namespace, Name: cr.Spec.PodName}, pod)
	if e != nil {
		reqLogger.Error(e, "can not get source pod")
		e = errSourcePodNotFound
		return
	}

	if pod.Status.Phase != corev1.PodRunning {
		e = errSourcePodNotReady
		reqLogger.Error(e, "source pod should be running")
		return
	}

	nodeName = pod.Spec.NodeName
	for _, c := range pod.Status.ContainerStatuses {
		if c.Name == cr.Spec.ContainerName {
			containerID = strings.TrimPrefix(c.ContainerID, containerIDPrefix)
			break
		}
	}
	if containerID == "" {
		e = errSourceContainerNotFound
		reqLogger.Error(e, "source container not found")
	}

	return
}

// newWorkerPod returns a pod with the same name/namespace as the cr
func (r *ReconcileContainerSnapshot) newWorkerPod(cr *atomv1alpha1.ContainerSnapshot) *corev1.Pod {
	labels := map[string]string{
		labelKeyPrefix + "snapshot":  cr.Name,
		labelKeyPrefix + "pod":       cr.Spec.PodName,
		labelKeyPrefix + "container": cr.Spec.ContainerName,
		labelKeyPrefix + "image":     cr.Spec.Image,
	}
	for k, v := range cr.Labels {
		labels[k] = v
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cr.Name + "-",
			Namespace:    cr.Namespace,
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{{
				Name: r.workerImagePullSecret,
			}},
			RestartPolicy:      corev1.RestartPolicyNever,
			NodeName:           cr.Status.NodeName,
			ServiceAccountName: r.workerServiceAccount,

			Containers: []corev1.Container{{
				Name:    "snapshot-worker",
				Image:   r.workerImage,
				Command: []string{"container-snapshot-worker"},
				Args:    []string{"--container", cr.Status.ContainerID, "--image", cr.Spec.Image, "--snapshot", cr.Name},
				Env: []corev1.EnvVar{{
					Name: "NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				}},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "image-push-secrets",
						MountPath: imagePushSecretPath,
						ReadOnly:  true,
					},
					{
						Name:      "docker-socket",
						MountPath: dockerSocketPath,
					},
				},
			}},
			Volumes: []corev1.Volume{
				{
					Name: "image-push-secrets",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							DefaultMode: pointer.Int32Ptr(0600),
						},
					},
				},
				{
					Name: "docker-socket",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: dockerSocketPath,
							Type: (*corev1.HostPathType)(pointer.StringPtr(string(corev1.HostPathSocket))),
						},
					},
				},
			},
		},
	}

	for _, sec := range cr.Spec.ImagePushSecrets {
		name := names.SimpleNameGenerator.GenerateName("sec-")
		pod.Spec.Volumes[0].VolumeSource.Projected.Sources = append(pod.Spec.Volumes[0].VolumeSource.Projected.Sources, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: sec.Name},
				Items: []corev1.KeyToPath{
					{
						Key:  corev1.DockerConfigKey,
						Path: filepath.Join(name, corev1.DockerConfigKey),
					},
					{
						Key:  corev1.DockerConfigJsonKey,
						Path: filepath.Join(name, corev1.DockerConfigJsonKey),
					},
				},
				Optional: pointer.BoolPtr(true),
			},
		})
	}

	return pod
}

func (r *ReconcileContainerSnapshot) getWorkerPod(ctx context.Context, ns string, uid types.UID) (*corev1.Pod, error) {
	var pods corev1.PodList
	e := r.client.List(ctx, &pods,
		client.InNamespace(ns),
		client.MatchingField("metadata.ownerReferences.uid", string(uid)),
	)
	if e != nil {
		return nil, e
	}
	if len(pods.Items) == 0 {
		return nil, errWorkerPodNotFound
	}
	if len(pods.Items) > 1 {
		return nil, errTooManyWorkerPods
	}

	return &pods.Items[0], nil
}

func logger(cr *atomv1alpha1.ContainerSnapshot) logr.Logger {
	return log.WithValues("snapshot name", cr.Name, "snapshot namespace", cr.Namespace)
}

func parseTerminationState(pod *corev1.Pod) *status.Condition {
	if len(pod.Status.ContainerStatuses) == 1 {
		if term := pod.Status.ContainerStatuses[0].LastTerminationState.Terminated; term != nil {
			switch reason := status.ConditionType(term.Reason); reason {
			case atomv1alpha1.InvalidImage, atomv1alpha1.DockerCommitFailed, atomv1alpha1.DockerPushFailed:
				return &status.Condition{
					Type:               reason,
					Status:             corev1.ConditionTrue,
					Message:            term.Message,
					LastTransitionTime: term.FinishedAt,
				}
			}
		}
	}

	return nil
}
