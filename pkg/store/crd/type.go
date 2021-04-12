package crd

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type TypeDefinition struct {
	GVK      schema.GroupVersionKind
	Singular string
	Plural   string
	Scope    apiextensionsv1.ResourceScope
}
