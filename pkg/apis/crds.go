package apis

import (
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/store/crd"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

var (
	// TypeDefinitions to create the appropriate crds
	TypeDefinitions = []*crd.TypeDefinition{
		&crd.TypeDefinition{
			GVK:      configv1alpha1.SchemeGroupVersion.WithKind("Account"),
			Singular: "account",
			Plural:   "accounts",
			Scope:    apiextensionsv1beta1.ClusterScoped,
		},
		&crd.TypeDefinition{
			GVK:      configv1alpha1.SchemeGroupVersion.WithKind("AccountQuota"),
			Singular: "accountquota",
			Plural:   "accountquotas",
			Scope:    apiextensionsv1beta1.ClusterScoped,
		},
		&crd.TypeDefinition{
			GVK:      configv1alpha1.SchemeGroupVersion.WithKind("Template"),
			Singular: "template",
			Plural:   "templates",
			Scope:    apiextensionsv1beta1.ClusterScoped,
		},
		&crd.TypeDefinition{
			GVK:      configv1alpha1.SchemeGroupVersion.WithKind("TemplateInstance"),
			Singular: "templateinstance",
			Plural:   "templateinstances",
			Scope:    apiextensionsv1beta1.NamespaceScoped,
		},
	}
)
