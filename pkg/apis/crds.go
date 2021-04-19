package apis

import (
	configv1alpha1 "github.com/loft-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/loft-sh/kiosk/pkg/store/crd"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var (
	// TypeDefinitions to create the appropriate crds
	TypeDefinitions = []*crd.TypeDefinition{
		&crd.TypeDefinition{
			GVK:      configv1alpha1.SchemeGroupVersion.WithKind("Account"),
			Singular: "account",
			Plural:   "accounts",
			Scope:    apiextensionsv1.ClusterScoped,
		},
		&crd.TypeDefinition{
			GVK:      configv1alpha1.SchemeGroupVersion.WithKind("AccountQuota"),
			Singular: "accountquota",
			Plural:   "accountquotas",
			Scope:    apiextensionsv1.ClusterScoped,
		},
		&crd.TypeDefinition{
			GVK:      configv1alpha1.SchemeGroupVersion.WithKind("Template"),
			Singular: "template",
			Plural:   "templates",
			Scope:    apiextensionsv1.ClusterScoped,
		},
		&crd.TypeDefinition{
			GVK:      configv1alpha1.SchemeGroupVersion.WithKind("TemplateInstance"),
			Singular: "templateinstance",
			Plural:   "templateinstances",
			Scope:    apiextensionsv1.NamespaceScoped,
		},
	}
)
