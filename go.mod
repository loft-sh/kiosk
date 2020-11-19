module github.com/kiosk-sh/kiosk

require (
	github.com/Azure/go-autorest v13.0.0+incompatible // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/loads v0.19.4
	github.com/go-openapi/spec v0.19.3
	github.com/hashicorp/golang-lru v0.5.4
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9 // indirect
	github.com/lib/pq v1.2.0 // indirect
	github.com/mattn/go-sqlite3 v1.11.0 // indirect
	github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c // indirect
	github.com/pkg/errors v0.8.1
	github.com/rancher/kine v0.3.2 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	gomodules.xyz/jsonpatch v2.0.0+incompatible // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.6
	k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/apiserver v0.18.6
	k8s.io/cli-runtime v0.18.6
	k8s.io/client-go v0.18.6
	k8s.io/component-base v0.18.6
	k8s.io/gengo v0.0.0-20200114144118-36b2048a9120
	k8s.io/klog v1.0.0
	k8s.io/kube-aggregator v0.18.6
	k8s.io/kube-controller-manager v0.18.6
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	k8s.io/kubectl v0.18.6
	sigs.k8s.io/apiserver-builder-alpha v1.18.0
	sigs.k8s.io/controller-runtime v0.6.3
)

go 1.13

replace github.com/kubernetes-incubator/reference-docs => github.com/kubernetes-sigs/reference-docs v0.0.0-20170929004150-fcf65347b256

replace github.com/markbates/inflect => github.com/markbates/inflect v1.0.4
