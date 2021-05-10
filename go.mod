module github.com/cloud-team-poc/openshift-cluster-api-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/openshift/api v0.0.0-20200618202633-7192180f496a // indirect
	k8s.io/api v0.17.9 // indirect
	k8s.io/apimachinery v0.17.9
	k8s.io/client-go v0.17.9
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800 // indirect
	sigs.k8s.io/cluster-api v0.3.16 // indirect
	sigs.k8s.io/cluster-api-provider-aws v0.6.5
	sigs.k8s.io/controller-runtime v0.5.14
)
