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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AccountQuotaSpec defines the desired state of AccountQuota
type AccountQuotaSpec struct {
	// account is the name of the account this quota should apply to
	Account string `json:"account"`

	// quota is the quota definition with all the limits and selectors
	// +optional
	Quota corev1.ResourceQuotaSpec `json:"quota,omitempty"`
}

// AccountQuotaStatus defines the observed state of AccountQuota
type AccountQuotaStatus struct {
	// Total defines the actual enforced quota and its current usage across all projects
	// +optional
	Total corev1.ResourceQuotaStatus `json:"total"`

	// Namespaces slices the usage by project.  This division allows for quick resolution of
	// deletion reconciliation inside of a single project without requiring a recalculation
	// across all projects.  This can be used to pull the deltas for a given project.
	// +optional
	// +nullable
	Namespaces AccountQuotasStatusByNamespace `json:"namespaces"`
}

// AccountQuotasStatusByNamespace bundles multiple resource quota status
type AccountQuotasStatusByNamespace []AccountQuotaStatusByNamespace

// AccountQuotaStatusByNamespace holds the status of a specific namespace
type AccountQuotaStatusByNamespace struct {
	// Namespace of the account this account quota applies to
	Namespace string `json:"namespace"`

	// Status indicates how many resources have been consumed by this project
	// +optional
	Status corev1.ResourceQuotaStatus `json:"status"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccountQuota is the Schema for the accountquotas API
// +k8s:openapi-gen=true
type AccountQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AccountQuotaSpec `json:"spec,omitempty"`

	// +optional
	Status AccountQuotaStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccountQuotaList contains a list of AccountQuota
type AccountQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccountQuota `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AccountQuota{}, &AccountQuotaList{})
}
