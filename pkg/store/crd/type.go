package crd

import (
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type TypeDefinition struct {
	GVK      schema.GroupVersionKind
	Singular string
	Plural   string
	Scope    apiextensionsv1beta1.ResourceScope
}
