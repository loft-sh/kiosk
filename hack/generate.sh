#!/bin/bash

set -e

echo "Generate apis..."

GO111MODULE=on
go run gen/cmd/apis/main.go --input-dirs github.com/kiosk-sh/kiosk/pkg/apis/... --go-header-file ./hack/boilerplate.go.txt

echo "Generate conversion, deepcopy, defaulter, openapi, client, lister & informers ..."

GO111MODULE=off
conversion-gen --input-dirs github.com/kiosk-sh/kiosk/pkg/apis/... -o $GOPATH/src --go-header-file ./hack/boilerplate.go.txt -O zz_generated.conversion --extra-peer-dirs k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime
deepcopy-gen --input-dirs github.com/kiosk-sh/kiosk/pkg/apis/... -o $GOPATH/src --go-header-file ./hack/boilerplate.go.txt -O zz_generated.deepcopy
openapi-gen --input-dirs github.com/kiosk-sh/kiosk/pkg/apis/... -o $GOPATH/src --go-header-file ./hack/boilerplate.go.txt -i k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/version,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/util/intstr,k8s.io/api/admission/v1,k8s.io/api/admission/v1beta1,k8s.io/api/admissionregistration/v1,k8s.io/api/admissionregistration/v1beta1,k8s.io/api/apps/v1,k8s.io/api/apps/v1beta1,k8s.io/api/apps/v1beta2,k8s.io/api/auditregistration/v1alpha1,k8s.io/api/authentication/v1,k8s.io/api/authentication/v1beta1,k8s.io/api/authorization/v1,k8s.io/api/authorization/v1beta1,k8s.io/api/autoscaling/v1,k8s.io/api/autoscaling/v2beta1,k8s.io/api/autoscaling/v2beta2,k8s.io/api/batch/v1,k8s.io/api/batch/v1beta1,k8s.io/api/batch/v2alpha1,k8s.io/api/certificates/v1beta1,k8s.io/api/coordination/v1,k8s.io/api/coordination/v1beta1,k8s.io/api/core/v1,k8s.io/api/discovery/v1alpha1,k8s.io/api/events/v1beta1,k8s.io/api/extensions/v1beta1,k8s.io/api/networking/v1,k8s.io/api/networking/v1beta1,k8s.io/api/node/v1alpha1,k8s.io/api/node/v1beta1,k8s.io/api/policy/v1beta1,k8s.io/api/rbac/v1,k8s.io/api/rbac/v1alpha1,k8s.io/api/rbac/v1beta1,k8s.io/api/scheduling/v1,k8s.io/api/scheduling/v1alpha1,k8s.io/api/scheduling/v1beta1,k8s.io/api/settings/v1alpha1,k8s.io/api/storage/v1,k8s.io/api/storage/v1alpha1,k8s.io/api/storage/v1beta1,k8s.io/client-go/pkg/apis/clientauthentication/v1alpha1,k8s.io/client-go/pkg/apis/clientauthentication/v1beta1,k8s.io/api/core/v1 --report-filename violations.report --output-package github.com/kiosk-sh/kiosk/pkg/openapi
client-gen -o $GOPATH/src --go-header-file ./hack/boilerplate.go.txt --input-base github.com/kiosk-sh/kiosk/pkg/apis --input tenancy/v1alpha1 --input config/v1alpha1 --clientset-path github.com/kiosk-sh/kiosk/pkg/client/clientset_generated --clientset-name clientset
lister-gen --input-dirs github.com/kiosk-sh/kiosk/pkg/apis/... -o $GOPATH/src --go-header-file ./hack/boilerplate.go.txt --output-package github.com/kiosk-sh/kiosk/pkg/client/listers_generated
informer-gen --input-dirs github.com/kiosk-sh/kiosk/pkg/apis/... -o $GOPATH/src --go-header-file ./hack/boilerplate.go.txt --output-package github.com/kiosk-sh/kiosk/pkg/client/informers_generated --listers-package github.com/kiosk-sh/kiosk/pkg/client/listers_generated --versioned-clientset-package github.com/kiosk-sh/kiosk/pkg/client/clientset_generated/clientset
