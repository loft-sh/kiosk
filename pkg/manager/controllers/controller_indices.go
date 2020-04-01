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

package controllers

import (
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	subjectpkg "github.com/kiosk-sh/kiosk/pkg/util/subject"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	apiGVStr = configv1alpha1.GroupVersion.String()
)

// AddManagerIndices adds the needed manager indices for faster listing of resources
func AddManagerIndices(indexer client.FieldIndexer) error {
	// Index account by subjects
	if err := indexer.IndexField(&configv1alpha1.Account{}, constants.IndexBySubjects, func(rawObj runtime.Object) []string {
		// grab the namespace object, extract the owner...
		account := rawObj.(*configv1alpha1.Account)
		subjects := []string{}
		for _, subject := range account.Spec.Subjects {
			subjectID := subjectpkg.ConvertSubject("", &subject)
			if subjectID != "" {
				subjects = append(subjects, subjectID)
			}
		}

		return subjects
	}); err != nil {
		return err
	}

	// Index account quota by account
	if err := indexer.IndexField(&configv1alpha1.AccountQuota{}, constants.IndexByAccount, func(rawObj runtime.Object) []string {
		quota := rawObj.(*configv1alpha1.AccountQuota)
		if quota.Spec.Account != "" {
			return []string{quota.Spec.Account}
		}

		return nil
	}); err != nil {
		return err
	}

	// Index namespaces by account
	if err := indexer.IndexField(&corev1.Namespace{}, constants.IndexByAccount, func(rawObj runtime.Object) []string {
		// grab the namespace object, extract the owner...
		namespace := rawObj.(*corev1.Namespace)
		if namespace.Labels != nil && namespace.Labels[constants.SpaceLabelAccount] != "" {
			return []string{namespace.Labels[constants.SpaceLabelAccount]}
		}

		return nil
	}); err != nil {
		return err
	}

	// Index clusterrole by owner account
	if err := indexer.IndexField(&rbacv1.ClusterRole{}, constants.IndexByAccount, func(rawObj runtime.Object) []string {
		// grab the cluster clusterrole object, extract the owner...
		cr := rawObj.(*rbacv1.ClusterRole)
		owner := metav1.GetControllerOf(cr)
		if owner == nil || owner.APIVersion != apiGVStr || owner.Kind != "Account" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	// Index clusterrolebinding by owner account
	if err := indexer.IndexField(&rbacv1.ClusterRoleBinding{}, constants.IndexByAccount, func(rawObj runtime.Object) []string {
		// grab the cluster clusterrolebinding object, extract the owner...
		cr := rawObj.(*rbacv1.ClusterRoleBinding)
		owner := metav1.GetControllerOf(cr)
		if owner == nil || owner.APIVersion != apiGVStr || owner.Kind != "Account" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	// Index role by owner account
	if err := indexer.IndexField(&rbacv1.Role{}, constants.IndexByAccount, func(rawObj runtime.Object) []string {
		// grab the role object, extract the owner...
		cr := rawObj.(*rbacv1.Role)
		owner := metav1.GetControllerOf(cr)
		if owner == nil || owner.APIVersion != apiGVStr || owner.Kind != "Account" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	// Index rolebinding by owner account
	if err := indexer.IndexField(&rbacv1.RoleBinding{}, constants.IndexByAccount, func(rawObj runtime.Object) []string {
		// grab the rolebinding object, extract the owner...
		cr := rawObj.(*rbacv1.RoleBinding)
		owner := metav1.GetControllerOf(cr)
		if owner == nil || owner.APIVersion != apiGVStr || owner.Kind != "Account" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	// Index templateinstance by template
	if err := indexer.IndexField(&configv1alpha1.TemplateInstance{}, constants.IndexByTemplate, func(rawObj runtime.Object) []string {
		// grab the rolebinding object, extract the owner...
		cr := rawObj.(*configv1alpha1.TemplateInstance)
		return []string{cr.Spec.Template}
	}); err != nil {
		return err
	}

	return nil
}
