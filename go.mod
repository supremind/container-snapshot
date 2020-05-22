module github.com/supremind/container-snapshot

go 1.13

require (
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v17.12.1-ce+incompatible
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.12.2
	github.com/onsi/gomega v1.10.1
	github.com/operator-framework/operator-sdk v0.17.1
	github.com/spf13/pflag v1.0.5
	github.com/supremind/pkg v0.1.0
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/apiserver v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/utils v0.0.0-20191114200735-6ca3b61696b6
	sigs.k8s.io/controller-runtime v0.5.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
)
