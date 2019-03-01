/*
Copyright 2018 The Knative Authors

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

package clusterchannelprovisioner

import (
	"context"
	"fmt"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/pkg/system"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// Channel is the name of the Channel resource in eventing.knative.dev/v1alpha1.
	Channel = "Channel"

	// Name of the corev1.Events emitted from the reconciliation process
	ccpReconciled          = "CcpReconciled"
	ccpUpdateStatusFailed  = "CcpUpdateStatusFailed"
	k8sServiceCreateFailed = "K8sServiceCreateFailed"
	k8sServiceDeleteFailed = "K8sServiceDeleteFailed"
)

var (
	// provisionerNames contains the list of provisioners' names served by this controller
	provisionerNames = []string{"in-memory-channel", "in-memory"}
	provisionerGVK   = schema.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "ClusterChannelProvisioner",
	}
)

type reconciler struct {
	client   client.Client
	recorder record.EventRecorder
	logger   *zap.Logger
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	//TODO use this to store the logger and set a deadline
	ctx := context.TODO()
	logger := r.logger.With(zap.Any("request", request))

	// Workaround until https://github.com/kubernetes-sigs/controller-runtime/issues/214 is fixed.
	// The reconcile requests will include a namespace if they are triggered because of changes to the
	// objects owned by this ClusterChannelProvisioner (e.g k8s service). Since ClusterChannelProvisioner is
	// cluster-scoped we need to unset the namespace or otherwise the provisioner object cannot be looked up.
	request.NamespacedName.Namespace = ""

	ccp := &eventingv1alpha1.ClusterChannelProvisioner{}
	err := r.client.Get(ctx, request.NamespacedName, ccp)

	// The ClusterChannelProvisioner may have been deleted since it was added to the workqueue. If so,
	// there is nothing to be done.
	if errors.IsNotFound(err) {
		logger.Info("Could not find ClusterChannelProvisioner", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		logger.Error("Unable to Get ClusterChannelProvisioner", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this ClusterChannelProvisioner?
	if !shouldReconcile(ccp.Namespace, ccp.Name, ccp.GroupVersionKind()) {
		logger.Info("Not reconciling ClusterChannelProvisioner, it is not controlled by this Controller", zap.String("APIVersion", ccp.APIVersion), zap.String("Kind", ccp.Kind), zap.String("Namespace", ccp.Namespace), zap.String("name", ccp.Name))
		return reconcile.Result{}, nil
	}
	logger.Info("Reconciling ClusterChannelProvisioner.")

	// Modify a copy of this object, rather than the original.
	ccp = ccp.DeepCopy()

	err = r.reconcile(ctx, ccp)
	if err != nil {
		logger.Info("Error reconciling ClusterChannelProvisioner", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	} else {
		logger.Info("ClusterChannelProvisioner reconciled")
		r.recorder.Eventf(ccp, corev1.EventTypeNormal, ccpReconciled, "ClusterChannelProvisioner reconciled: %q", ccp.Name)
	}

	if updateStatusErr := util.UpdateClusterChannelProvisionerStatus(ctx, r.client, ccp); updateStatusErr != nil {
		logger.Info("Error updating ClusterChannelProvisioner Status", zap.Error(updateStatusErr))
		r.recorder.Eventf(ccp, corev1.EventTypeWarning, ccpUpdateStatusFailed, "Failed to update ClusterChannelProvisioner's status: %v", err)
		return reconcile.Result{}, updateStatusErr
	}

	return reconcile.Result{}, err
}

// IsControlled determines if the in-memory Channel Controller should control (and therefore
// reconcile) a given object, based on that object's ClusterChannelProvisioner reference.
func IsControlled(ref *corev1.ObjectReference) bool {
	if ref != nil {
		return shouldReconcile(ref.Namespace, ref.Name, ref.GroupVersionKind())
	}
	return false
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterChannelProvisioner. This Controller only handles in-memory channels.
func shouldReconcile(namespace, name string, gvk schema.GroupVersionKind) bool {
	if gvk.Group == provisionerGVK.Group &&
		gvk.Version == provisionerGVK.Version &&
		gvk.Kind == provisionerGVK.Kind {
		for _, p := range provisionerNames {
			if namespace == "" && name == p {
				return true
			}
		}
	}
	return false
}

func (r *reconciler) reconcile(ctx context.Context, ccp *eventingv1alpha1.ClusterChannelProvisioner) error {
	logger := r.logger.With(zap.Any("clusterChannelProvisioner", ccp))

	// We are syncing one thing.
	// 1. The K8s Service to talk to all in-memory Channels.
	//     - There is a single K8s Service for all requests going any in-memory Channel.

	if ccp.DeletionTimestamp != nil {
		// K8s garbage collection will delete the dispatcher service, once this ClusterChannelProvisioner
		// is deleted, so we don't need to do anything.
		return nil
	}

	// The name of the svc has changed since version 0.2.1. Hence, delete old dispatcher service (in-memory-channel-clusterbus)
	// that was created previously in version 0.2.0 to ensure backwards compatibility.
	err := r.deleteOldDispatcherService(ctx, ccp)
	if err != nil {
		logger.Info("Error deleting the old ClusterChannelProvisioner's K8s Service", zap.Error(err))
		r.recorder.Eventf(ccp, corev1.EventTypeWarning, k8sServiceDeleteFailed, "Failed to delete the old ClusterChannelProvisioner's K8s Service: %v", err)
		return err
	}

	ccp.Status.MarkReady()
	return nil
}

func (r *reconciler) deleteOldDispatcherService(ctx context.Context, ccp *eventingv1alpha1.ClusterChannelProvisioner) error {
	svcName := fmt.Sprintf("%s-clusterbus", ccp.Name)
	svcKey := types.NamespacedName{
		Namespace: system.Namespace(),
		Name:      svcName,
	}
	svc := &corev1.Service{}
	err := r.client.Get(ctx, svcKey, svc)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return r.client.Delete(ctx, svc)
}
