module admiralty.io/multicluster-scheduler

go 1.12

replace (
	// admiralty.io/multicluster-controller => ../multicluster-controller
	// admiralty.io/multicluster-service-account => ../multicluster-service-account
	github.com/appscode/jsonpatch => gomodules.xyz/jsonpatch/v2 v2.0.0
	k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471 // release-1.13
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628 // release-1.13
)

require (
	admiralty.io/multicluster-controller v0.3.1
	admiralty.io/multicluster-service-account v0.6.1
	github.com/appscode/jsonpatch v0.0.0-00010101000000-000000000000 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/go-test/deep v1.0.1
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef // indirect
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/onsi/gomega v1.4.3
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.2.0 // indirect
	golang.org/x/net v0.0.0-20190813141303-74dc4d7220e7
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	gomodules.xyz/jsonpatch/v2 v2.0.1 // indirect
	gopkg.in/inf.v0 v0.9.1
	k8s.io/api v0.0.0-20191025225708-5524a3672fbb
	k8s.io/apiextensions-apiserver v0.0.0-20191026071228-81c2f4fbaa0d // indirect
	k8s.io/apimachinery v0.0.0-20191025225532-af6325b3a843
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/sample-controller v0.0.0-20190625130054-294bc0f66822
	sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/testing_frameworks v0.1.2 // indirect
	sigs.k8s.io/yaml v1.1.0
)
