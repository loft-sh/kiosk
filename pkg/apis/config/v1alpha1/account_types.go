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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AccountSpec defines a single account configuration
type AccountSpec struct {
	// This defines the cluster role that will be used for the rolebinding when
	// creating a new space for the selected subjects
	// +optional
	SpaceClusterRole *string `json:"spaceClusterRole,omitempty"`

	// SpaceDefaultTemplates are templates that should be initialized during space
	// creation.
	// +optional
	SpaceDefaultTemplates []TemplateInstanceSpec `json:"spaceDefaultTemplates,omitempty"`

	// SpaceLimit is the amount of spaces an account is allowed to create in the given cluster
	// +optional
	SpaceLimit *int `json:"spaceLimit,omitempty"`

	// Subjects are the account users
	// +optional
	Subjects []rbacv1.Subject `json:"subjects,omitempty"`
}

// AccountStatus describes the current status of the account is the cluster
type AccountStatus struct {
	// +optional
	Namespaces []AccountNamespaceStatus `json:"namespaces,omitempty"`
}

// AccountNamespaceStatus is the status for the account access objects that belong to the account
type AccountNamespaceStatus struct {
	// +optional
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// Account is the Schema for the accounts API
type Account struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec AccountSpec `json:"spec,omitempty"`

	// +optional
	Status AccountStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AccountList contains a list of Account
type AccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Account `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Account{}, &AccountList{})
}
