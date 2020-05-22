package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/operator-framework/operator-sdk/pkg/status"
	atomv1alpha1 "github.com/supremind/container-snapshot/pkg/apis/atom/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Controller struct {
	client    client.Client
	snapshot  string
	namespace string
}

func NewController(namespace, snapshot string) (*Controller, error) {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "get kube config")
		return nil, err
	}

	// Create a new manager to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Namespace: namespace,
	})
	if err != nil {
		log.Error(err, "init snapshot manager")
		return nil, err
	}

	return &Controller{
		client:    mgr.GetClient(),
		snapshot:  snapshot,
		namespace: namespace,
	}, nil
}

func (c *Controller) UpdateCondition(ctx context.Context, snapshot string, err error) error {
	if err == nil {
		return nil
	}

	snp := &atomv1alpha1.ContainerSnapshot{}
	e := c.client.Get(ctx, types.NamespacedName{Name: snapshot, Namespace: c.namespace}, snp)
	if e != nil {
		return fmt.Errorf("get snapshot instance: %w", e)
	}

	snp.Status.WorkerState = atomv1alpha1.WorkerFailed

	var stale bool
	if errors.Is(err, ErrInvalidImage) {
		stale = snp.Status.Conditions.SetCondition(status.Condition{
			Type:               atomv1alpha1.InvalidImage,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	} else if errors.Is(err, ErrCommit) {
		stale = snp.Status.Conditions.SetCondition(status.Condition{
			Type:               atomv1alpha1.DockerCommitFailed,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	} else if errors.Is(err, ErrPush) {
		stale = snp.Status.Conditions.SetCondition(status.Condition{
			Type:               atomv1alpha1.DockerPushFailed,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	}

	if stale {
		return c.client.Status().Patch(ctx, snp, client.Apply)
	}
	return nil
}
