/*
Copyright 2020 DevSpace Technologies Inc.

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
	// Space defines default options for created spaces by the account
	// +optional
	Space AccountSpace `json:"space,omitempty"`

	// Subjects are the account users
	// +optional
	Subjects []rbacv1.Subject `json:"subjects,omitempty"`
}

// AccountSpace defines properties how many spaces can be owned by the account and how they should be created
type AccountSpace struct {
	// This defines the cluster role that will be used for the rolebinding when
	// creating a new space for the selected subjects
	// +optional
	ClusterRole *string `json:"clusterRole,omitempty"`

	// Limit defines how many spaces are allowed to be owned by this account. If no value is specified,
	// unlimited spaces can be created by the account (if the users have the rights to create spaces)
	// +optional
	Limit *int `json:"limit,omitempty"`

	// TemplateInstances are templates that should be created by default in a newly created space by
	// this account. Kiosk makes sure that these templates are deployed successfully, before the users of
	// this account will get access to the space
	// +optional
	TemplateInstances []AccountTemplateInstanceTemplate `json:"templateInstances,omitempty"`

	// SpaceTemplate defines a space template with default annotations and labels the space should have after
	// creation
	// +optional
	SpaceTemplate AccountSpaceTemplate `json:"spaceTemplate,omitempty"`
}

// AccountSpaceTemplate defines a space template
type AccountSpaceTemplate struct {
	// The default metadata of the space to create
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// AccountTemplateInstanceTemplate defines a template instance template
type AccountTemplateInstanceTemplate struct {
	// The metadata of the template instace to create
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The spec of the template instance
	// +optional
	Spec TemplateInstanceSpec `json:"spec,omitempty"`
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

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Account
// +k8s:openapi-gen=true
type Account struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec AccountSpec `json:"spec,omitempty"`

	// +optional
	Status AccountStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccountList contains a list of Account
type AccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Account `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Account{}, &AccountList{})
}
