package controllers

import (
	"context"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sutilspointer "k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *CAPIDeploymentReconciler) reconcileCAPA(ctx context.Context, namespace string, infra *configv1.Infrastructure) error {
	region := getAWSRegion(infra)
	if region == "" {
		return fmt.Errorf("region can't be nil, something went wrong")
	}

	capaCluster := CAPACluster(infra.ClusterName, namespace)
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, capaCluster, func() error {
		return r.reconcileCAPACluster(capaCluster, region)
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile capa cluster: %w", err)
	}

	err = r.reconcileCAPAComponents(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to reconcile capi components: %w", err)
	}
	return nil
}

func getAWSRegion(infra *configv1.Infrastructure) string {
	if infra.Status.PlatformStatus == nil || infra.Status.PlatformStatus.AWS == nil {
		return ""
	}

	return infra.Status.PlatformStatus.AWS.Region
}

func (r *CAPIDeploymentReconciler) reconcileCAPAComponents(ctx context.Context, namespace string) error {
	clusterRoleBinding := CAPAManagerClusterRoleBinding()

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, clusterRoleBinding, func() error {
		return reconcileCAPAManagerClusterRoleBinding(clusterRoleBinding, namespace)
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile capi manager cluster role binding: %w", err)
	}

	deployment := ClusterAPIAWSManagerDeployment(namespace)

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		return reconcileCAPIAWSProviderDeployment(deployment, "quay.io/ademicev/cluster-api-aws-controller-amd64:dev")
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile capa manager deployment: %w", err)
	}

	return nil
}
func ClusterAPIAWSManagerDeployment(namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "capa-controller-manager",
		},
	}
}

func reconcileCAPIAWSProviderDeployment(deployment *appsv1.Deployment, image string) error {
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: k8sutilspointer.Int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"control-plane": "capa-controller-manager",
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"control-plane": "capa-controller-manager",
				},
			},
			Spec: corev1.PodSpec{
				ServiceAccountName:            "default",
				TerminationGracePeriodSeconds: k8sutilspointer.Int64Ptr(10),
				Tolerations: []corev1.Toleration{
					{
						Key:    "node-role.kubernetes.io/master",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "credentials",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "capa-manager-bootstrap-credentials",
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:            "manager",
						Image:           image,
						ImagePullPolicy: corev1.PullAlways,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "credentials",
								MountPath: "/home/.aws",
							},
						},
						Env: []corev1.EnvVar{
							{
								Name: "MY_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
							{
								Name:  "AWS_SHARED_CREDENTIALS_FILE",
								Value: "/home/.aws/credentials",
							},
						},
						Command: []string{"/manager"},
						Args:    []string{"--namespace", "$(MY_NAMESPACE)", "--alsologtostderr", "--v=4"},
						Ports: []corev1.ContainerPort{
							{
								Name:          "healthz",
								ContainerPort: 9440,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromString("healthz"),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/readyz",
									Port: intstr.FromString("healthz"),
								},
							},
						},
					},
				},
			},
		},
	}

	return nil
}
func (r *CAPIDeploymentReconciler) reconcileCAPICluster(cluster *clusterv1.Cluster, infraName, infraNamespace string) error {
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

func CAPACluster(name, namespace string) *infrav1.AWSCluster {
	return &infrav1.AWSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: map[string]string{"cluster.x-k8s.io/managed-by": ""},
		},
	}
}

func (r *CAPIDeploymentReconciler) reconcileCAPACluster(awsCluster *infrav1.AWSCluster, region string) error {
	awsCluster.Annotations = map[string]string{"cluster.x-k8s.io/managed-by": ""}
	awsCluster.Spec = infrav1.AWSClusterSpec{
		Region: region,
	}

	return nil
}
