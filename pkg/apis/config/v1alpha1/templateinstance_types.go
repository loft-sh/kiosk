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
	"k8s.io/apimachinery/pkg/types"
)

// TemplateInstanceSpec holds the expected cluster status of the template instance
type TemplateInstanceSpec struct {
	// The template to instantiate. This is an immutable field
	Template string `json:"template"`
}

// TemplateInstanceStatus describes the current status of the template instance in the cluster
type TemplateInstanceStatus struct {
	// Status holds the template instances status
	Status TemplateInstanceDeploymentStatus `json:"status"`
	// A human readable message indicating details about why the namespace is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the namespace is in this state.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Resources holds the status of the deployed resources
	// +optional
	Resources []ResourceStatus `json:"resources,omitempty"`

	// TemplateResourceVersion is the resource version of the template that was applied
	// +optional
	TemplateResourceVersion string `json:"templateResourceVersion,omitempty"`
	// LastAppliedAt indicates when the template was last applied
	// +optional
	LastAppliedAt *metav1.Time `json:"observedAt,omitempty"`
}

// ResourceStatus holds the current status of a resource
type ResourceStatus struct {
	// +optional
	Group string `json:"group,omitempty"`
	// +optional
	Version string `json:"version,omitempty"`
	// +optional
	Kind string `json:"kind,omitempty"`
	// +optional
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	UID types.UID `json:"uid,omitempty"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// TemplateInstanceDeploymentStatus describes the status of template instance deployment
type TemplateInstanceDeploymentStatus string

// These are the valid statuses
const (
	// TemplateInstanceDeploymentStatusDeployed describes a succeeded template instance deployment
	TemplateInstanceDeploymentStatusDeployed TemplateInstanceDeploymentStatus = "Deployed"

	// TemplateInstanceDeploymentStatusFailed describes a failed template instance deployment
	TemplateInstanceDeploymentStatusFailed TemplateInstanceDeploymentStatus = "Failed"

	// TemplateInstanceDeploymentStatusPending describes a not yet deployed template instance
	TemplateInstanceDeploymentStatusPending TemplateInstanceDeploymentStatus = ""
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TemplateInstance is the Schema for the templatesinstance API
type TemplateInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TemplateInstanceSpec `json:"spec,omitempty"`

	// +optional
	Status TemplateInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TemplateInstanceList contains a list of Account
type TemplateInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplateInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TemplateInstance{}, &TemplateInstanceList{})
}
