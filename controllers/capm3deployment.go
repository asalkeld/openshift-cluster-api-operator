package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/cloud-team-poc/openshift-cluster-api-operator/assets/baremetal"
)

func (r *CAPIDeploymentReconciler) reconcileCAPM3(ctx context.Context, namespace string) error {
	genericCodec := serializer.NewCodecFactory(r.Scheme).UniversalDeserializer()

	for _, assetName := range baremetal.AssetNames() {
		r.Log.Info("applying", "asset", assetName)
		objBytes, err := baremetal.Asset(assetName)
		if err != nil {
			return fmt.Errorf("could not read %s : %w", assetName, err)
		}

		obj, _, err := genericCodec.Decode(objBytes, nil, nil)
		if err != nil {
			return fmt.Errorf("could not decode %s : %w", assetName, err)
		}

		clientObject, ok := obj.(client.Object)
		if ok && clientObject.GetNamespace() != "" {
			// install all resources into the required namespace
			clientObject.SetNamespace(namespace)
		}

		// TODO(Angus) we are going to need to set the images, for now log them.
		if d, ok := obj.(*appsv1.Deployment); ok {
			for _, c := range d.Spec.Template.Spec.Containers {
				r.Log.Info("using image", "name", c.Image, "container", c.Name)
			}
		}

		origObject := clientObject.DeepCopyObject()
		_, err = controllerutil.CreateOrPatch(ctx, r.Client, clientObject, func() error {
			clientObject = origObject.(client.Object)
			return nil
		})
		if err != nil {
			return fmt.Errorf("could not CreateOrUpdate %s : %w", assetName, err)
		}
	}
	return nil
}
