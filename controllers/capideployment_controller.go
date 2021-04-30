/*


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

package controllers

import (
	"context"
	"fmt"

	operatorv1 "github.com/cloud-team-poc/openshift-cluster-api-operator/api/v1"
	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CAPIDeploymentReconciler reconciles a CAPIDeployment object
type CAPIDeploymentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const (
	globalInfrastuctureName = "cluster"
)

// +kubebuilder:rbac:groups=capi.openshift.io,resources=capideployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=capi.openshift.io,resources=capideployments/status,verbs=get;update;patch

func (r *CAPIDeploymentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("capideployment", req.NamespacedName)

	capiDeployment := &operatorv1.CAPIDeployment{}

	if err := r.Client.Get(ctx, req.NamespacedName, capiDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	infra := &configv1.Infrastructure{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: globalInfrastuctureName}, infra); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get infrastructure object: %w", err)
	}

	// Reconcile the CAPI Cluster resource
	capiCluster := CAPICluster(infra.ClusterName, capiDeployment.Namespace)
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, capiCluster, func() error {
		return reconcileCAPICluster(capiCluster, infra.ClusterName, capiDeployment.Namespace)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile capi cluster: %w", err)
	}

	// Create CAPA Cluster
	region := getAWSRegion(infra)
	if region == "" {
		return ctrl.Result{}, fmt.Errorf("region can't be nil, something went wrong")
	}

	capaCluster := CAPACluster(infra.ClusterName, capiDeployment.Namespace, region)
	if err := r.Client.Create(ctx, capaCluster); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *CAPIDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.CAPIDeployment{}).
		Complete(r)
}

func getAWSRegion(infra *configv1.Infrastructure) string {
	if infra.Status.PlatformStatus == nil || infra.Status.PlatformStatus.AWS == nil {
		return ""
	}

	return infra.Status.PlatformStatus.AWS.Region
}

func CAPICluster(name, namespace string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func reconcileCAPICluster(cluster *clusterv1.Cluster, infraName, infraNamespace string) error {
	// We only create this resource once and then let CAPI own it
	if !cluster.CreationTimestamp.IsZero() {
		return nil
	}

	cluster.Spec = clusterv1.ClusterSpec{
		InfrastructureRef: &corev1.ObjectReference{
			APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
			Kind:       "AWSCluster",
			Namespace:  infraNamespace,
			Name:       infraName,
		},
	}

	return nil
}

func CAPACluster(name, namespace, region string) *infrav1.AWSCluster {
	return &infrav1.AWSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: map[string]string{"cluster.x-k8s.io/managed-by": ""},
		},
		Spec: infrav1.AWSClusterSpec{
			Region: region,
		},
	}
}
