module github.com/cloud-team-poc/openshift-cluster-api-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	k8s.io/apimachinery v0.17.9
	k8s.io/client-go v0.17.9
	sigs.k8s.io/cluster-api-provider-aws v0.6.5
	sigs.k8s.io/controller-runtime v0.5.14
)
