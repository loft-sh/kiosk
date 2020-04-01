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
)

// TemplateInstanceNoOwnerAnnotation if this annotation is set on a template instance,
// the template instance is not setting itself as the owner of the created objects
const TemplateInstanceNoOwnerAnnotation = "templateinstance.config.kiosk.sh/no-owner"

// TemplateInstanceSpec holds the expected cluster status of the template instance
type TemplateInstanceSpec struct {
	// The template to instantiate. This is an immutable field
	Template string `json:"template"`
	// If true the template instance will keep the deployed resources in sync with the template.
	// +optional
	Sync bool `json:"sync,omitempty"`
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

	// TemplateResourceVersion is the resource version of the template that was applied
	// +optional
	TemplateResourceVersion string `json:"templateResourceVersion,omitempty"`
	// TemplateManifests are the manifests that were rendered before
	// +optional
	TemplateManifests string `json:"templateManifests,omitempty"`

	// LastAppliedAt indicates when the template was last applied
	// +optional
	LastAppliedAt *metav1.Time `json:"observedAt,omitempty"`
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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemplateInstance is the Schema for the templatesinstance API
// +k8s:openapi-gen=true
type TemplateInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TemplateInstanceSpec `json:"spec,omitempty"`

	// +optional
	Status TemplateInstanceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemplateInstanceList contains a list of Account
type TemplateInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplateInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TemplateInstance{}, &TemplateInstanceList{})
}
