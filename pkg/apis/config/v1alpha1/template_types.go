/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
)

// TemplateResources defines a templates resources
type TemplateResources struct {
	// manifest represents kubernetes resources that will be deployed into the target namespace
	// +optional
	Manifests []EmbeddedResource `json:"manifests,omitempty"`

	// helm defines the configuration for a helm deployment
	// +optional
	Helm *HelmConfiguration `json:"helm,omitempty"`
}

// EmbeddedResource holds a kubernetes resource
// +kubebuilder:validation:XPreserveUnknownFields
// +kubebuilder:validation:XEmbeddedResource
type EmbeddedResource struct {
	runtime.RawExtension `json:",inline"`
}

// HelmConfiguration holds the helm configuration
type HelmConfiguration struct {
	// The helm release name. If omitted the template name will be used
	// +optional
	ReleaseName string `json:"releaseName,omitempty"`

	// Values in the form of name=value that will be passed to the helm command during
	// helm template
	// +optional
	SetValues []HelmSetValue `json:"setValues,omitempty"`

	// The additional helm values to use. Expected block string
	// +optional
	Values string `json:"values,omitempty"`

	// Tells us where to find the helm chart to deploy
	Chart HelmChart `json:"chart,omitempty"`
}

// HelmChart holds the information needed to find a chart to deploy
type HelmChart struct {
	// Load helm chart from a repository
	// +optional
	Repository *HelmChartRepository `json:"repository,omitempty"`
}

// HelmChartRepository defines a helm repository where kiosk can load a chart from
type HelmChartRepository struct {
	// Name of the chart to deploy
	Name string `json:"name"`

	// Version is the version of the chart to deploy
	// +optional
	Version string `json:"version,omitempty"`

	// The repo url to use
	// +optional
	RepoURL string `json:"repoUrl,omitempty"`

	// The username to use for the selected repository
	// +optional
	Username *HelmSecretRef `json:"username,omitempty"`

	// The password to use for the selected repository
	// +optional
	Password *HelmSecretRef `json:"password,omitempty"`
}

// HelmSecretRef holds a secret reference to a secret
type HelmSecretRef struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// HelmSetValue defines a name=value pair that will be passed to helm template
type HelmSetValue struct {
	// The path of the value to set
	Name string `json:"name"`

	// The value to set
	Value string `json:"value"`

	// ForceString specifies if the parameter `--set` or `--set-string` should be used
	// +optional
	ForceString bool `json:"forceString,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Template is the Schema for the templates API
// +k8s:openapi-gen=true
type Template struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Resources TemplateResources `json:"resources,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemplateList contains a list of Account
type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Template{}, &TemplateList{})
}
