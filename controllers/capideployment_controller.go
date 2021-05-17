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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sutilspointer "k8s.io/utils/pointer"
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

func (r *CAPIDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	capiCluster := CAPICluster(capiDeployment.Name, capiDeployment.Namespace)
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, capiCluster, func() error {
		return r.reconcileCAPICluster(capiCluster, capiDeployment.Name, capiDeployment.Namespace)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile capi cluster: %w", err)
	}
	err = r.reconcileCAPIComponents(ctx, capiDeployment.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile capi components: %w", err)
	}

	switch infra.Status.PlatformStatus.Type {
	case "AWS":
		err = r.reconcileCAPA(ctx, capiDeployment.Namespace, infra)
	case "BareMetal":
		err = r.reconcileCAPM3(ctx, capiDeployment.Namespace)
	default:
		err = fmt.Errorf("platform %s not yet supported", infra.Status.PlatformStatus.Type)
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile platform deployments: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *CAPIDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.CAPIDeployment{}).
		Complete(r)
}

func CAPICluster(name, namespace string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func CAPIManagerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-api",
		},
	}
}

func reconcileCAPIManagerClusterRoleBinding(binding *rbacv1.ClusterRoleBinding, namespace string) error {
	binding.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: namespace,
		},
	}
	binding.RoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     "cluster-admin",
	}
	return nil
}

func ClusterAPIManagerDeployment(namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "capi-controller-manager",
		},
	}
}

func reconcileCAPIManagerDeployment(deployment *appsv1.Deployment, image string) error {
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: k8sutilspointer.Int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"name": "cluster-api",
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"name": "cluster-api",
				},
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "default",
				Containers: []corev1.Container{
					{
						Name:            "manager",
						Image:           image,
						ImagePullPolicy: corev1.PullAlways,
						Env: []corev1.EnvVar{
							{
								Name: "MY_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
						},
						Command: []string{"/manager"},
						Args:    []string{"--namespace", "$(MY_NAMESPACE)", "--alsologtostderr", "--v=4"},
					},
				},
			},
		},
	}
	return nil
}

func (r *CAPIDeploymentReconciler) reconcileCAPIComponents(ctx context.Context, namespace string) error {
	clusterRoleBinding := CAPIManagerClusterRoleBinding()

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, clusterRoleBinding, func() error {
		return reconcileCAPIManagerClusterRoleBinding(clusterRoleBinding, namespace)
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile capi manager cluster role binding: %w", err)
	}

	deployment := ClusterAPIManagerDeployment(namespace)

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		return reconcileCAPIManagerDeployment(deployment, "us.gcr.io/k8s-artifacts-prod/cluster-api/cluster-api-controller:v0.3.12")
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile capi manager deployment: %w", err)
	}

	return nil
}

func CAPAManagerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-api-aws",
		},
	}
}

func reconcileCAPAManagerClusterRoleBinding(binding *rbacv1.ClusterRoleBinding, namespace string) error {
	binding.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: namespace,
		},
	}
	binding.RoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     "cluster-admin",
	}
	return nil
}
