module github.com/cloud-team-poc/openshift-cluster-api-operator

go 1.13

require (
	github.com/go-bindata/go-bindata v1.0.0
	github.com/go-logr/logr v0.3.0
	github.com/jetstack/cert-manager v1.3.1
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/openshift/api v0.0.0-20201214114959-164a2fb63b5f
	github.com/openshift/client-go v0.0.0-20201020074620-f8fd44879f7c
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/cluster-api v0.3.16
	sigs.k8s.io/cluster-api-provider-aws v0.6.5
	sigs.k8s.io/controller-runtime v0.8.3
)
