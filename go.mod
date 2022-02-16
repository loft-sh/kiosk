module github.com/loft-sh/kiosk

require (
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/evanphx/json-patch v4.11.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/go-openapi/loads v0.19.4
	github.com/go-openapi/spec v0.19.5
	github.com/hashicorp/golang-lru v0.5.4
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9 // indirect
	github.com/lib/pq v1.2.0 // indirect
	github.com/loft-sh/apiserver v0.0.0-20210607160412-10c99558fdeb
	github.com/mattn/go-sqlite3 v1.11.0 // indirect
	github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c // indirect
	github.com/pkg/errors v0.9.1
	github.com/rancher/kine v0.3.2 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/ugorji/go v1.1.4 // indirect
	gomodules.xyz/jsonpatch v2.0.0+incompatible // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.22.6
	k8s.io/apiextensions-apiserver v0.21.1
	k8s.io/apimachinery v0.22.6
	k8s.io/apiserver v0.21.1
	k8s.io/cli-runtime v0.21.1
	k8s.io/client-go v0.22.6
	k8s.io/component-base v0.21.1
	k8s.io/gengo v0.0.0-20201214224949-b6c5ce23f027
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kube-aggregator v0.21.1
	k8s.io/kube-controller-manager v0.21.1
	k8s.io/kube-openapi v0.0.0-20211109043538-20434351676c
	k8s.io/kubectl v0.21.1
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/controller-runtime v0.9.0
)

go 1.16

replace (
	github.com/go-openapi/jsonpointer => github.com/go-openapi/jsonpointer v0.19.3
	github.com/go-openapi/jsonreference => github.com/go-openapi/jsonreference v0.19.3
	github.com/go-openapi/swag => github.com/go-openapi/swag v0.19.5
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
	github.com/kubernetes-incubator/reference-docs => github.com/kubernetes-sigs/reference-docs v0.0.0-20170929004150-fcf65347b256
	github.com/markbates/inflect => github.com/markbates/inflect v1.0.4
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
)
