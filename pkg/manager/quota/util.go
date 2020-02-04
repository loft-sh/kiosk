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

package quota

import (
	corev1 "k8s.io/api/core/v1"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
)

func GetResourceQuotasStatusByNamespace(namespaceStatuses configv1alpha1.AccountQuotasStatusByNamespace, namespace string) (corev1.ResourceQuotaStatus, bool) {
	for i := range namespaceStatuses {
		curr := namespaceStatuses[i]
		if curr.Namespace == namespace {
			return curr.Status, true
		}
	}
	return corev1.ResourceQuotaStatus{}, false
}

func RemoveResourceQuotasStatusByNamespace(namespaceStatuses *configv1alpha1.AccountQuotasStatusByNamespace, namespace string) {
	newNamespaceStatuses := configv1alpha1.AccountQuotasStatusByNamespace{}
	for i := range *namespaceStatuses {
		curr := (*namespaceStatuses)[i]
		if curr.Namespace == namespace {
			continue
		}
		newNamespaceStatuses = append(newNamespaceStatuses, curr)
	}
	*namespaceStatuses = newNamespaceStatuses
}

func InsertResourceQuotasStatus(namespaceStatuses *configv1alpha1.AccountQuotasStatusByNamespace, newStatus configv1alpha1.AccountQuotaStatusByNamespace) {
	newNamespaceStatuses := configv1alpha1.AccountQuotasStatusByNamespace{}
	found := false
	for i := range *namespaceStatuses {
		curr := (*namespaceStatuses)[i]
		if curr.Namespace == newStatus.Namespace {
			// do this so that we don't change serialization order
			newNamespaceStatuses = append(newNamespaceStatuses, newStatus)
			found = true
			continue
		}
		newNamespaceStatuses = append(newNamespaceStatuses, curr)
	}
	if !found {
		newNamespaceStatuses = append(newNamespaceStatuses, newStatus)
	}
	*namespaceStatuses = newNamespaceStatuses
}
