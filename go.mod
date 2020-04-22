module admiralty.io/multicluster-scheduler

go 1.13

require (
	admiralty.io/multicluster-controller v0.5.0
	admiralty.io/multicluster-service-account v0.6.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/go-test/deep v1.0.1
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.5
	github.com/virtual-kubelet/virtual-kubelet v1.2.1
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.2.0 // indirect
	google.golang.org/genproto v0.0.0-20190716160619-c506a9f90610 // indirect
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/code-generator v0.17.0
	k8s.io/component-base v0.17.0
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.17.0
	k8s.io/sample-controller v0.17.0
	sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/yaml v1.1.0
)

// replace admiralty.io/multicluster-controller => ../multicluster-controller

// replace admiralty.io/multicluster-service-account => ../multicluster-service-account

replace github.com/appscode/jsonpatch => gomodules.xyz/jsonpatch/v2 v2.0.0

replace k8s.io/api => k8s.io/api v0.17.0

replace k8s.io/apimachinery => k8s.io/apimachinery v0.17.0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.0

replace k8s.io/apiserver => k8s.io/apiserver v0.17.0

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.17.0

replace k8s.io/client-go => k8s.io/client-go v0.17.0

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.17.0

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.0

replace k8s.io/code-generator => k8s.io/code-generator v0.17.0

replace k8s.io/component-base => k8s.io/component-base v0.17.0

replace k8s.io/cri-api => k8s.io/cri-api v0.17.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.17.0

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.17.0

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.17.0

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.17.0

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.17.0

replace k8s.io/kubectl => k8s.io/kubectl v0.17.0

replace k8s.io/kubelet => k8s.io/kubelet v0.17.0

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.17.0

replace k8s.io/metrics => k8s.io/metrics v0.17.0

replace k8s.io/node-api => k8s.io/node-api v0.17.0

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.17.0

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.17.0

replace k8s.io/sample-controller => k8s.io/sample-controller v0.17.0
