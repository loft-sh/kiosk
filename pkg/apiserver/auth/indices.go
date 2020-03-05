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

package auth

import (
	tenancyv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	apiGVStr = tenancyv1alpha1.GroupVersion.String()
)

// registerIndices adds the needed manager indices for faster listing of resources
func registerIndices(indexer client.FieldIndexer) error {
	// Index account by subjects
	if err := indexer.IndexField(&tenancyv1alpha1.Account{}, constants.IndexBySubjects, func(rawObj runtime.Object) []string {
		// grab the namespace object, extract the owner...
		account := rawObj.(*tenancyv1alpha1.Account)
		subjects := []string{}
		for _, subject := range account.Spec.Subjects {
			subjectID := ConvertSubject("", &subject)
			if subjectID != "" {
				subjects = append(subjects, subjectID)
			}
		}

		return subjects
	}); err != nil {
		return err
	}

	// Index namespaces by account
	if err := indexer.IndexField(&corev1.Namespace{}, constants.IndexByAccount, func(rawObj runtime.Object) []string {
		// grab the namespace object, extract the owner...
		namespace := rawObj.(*corev1.Namespace)
		if namespace.Labels != nil && namespace.Labels[tenancy.SpaceLabelAccount] != "" {
			return []string{namespace.Labels[tenancy.SpaceLabelAccount]}
		}

		return nil
	}); err != nil {
		return err
	}

	// Index rolebinding by subject
	if err := indexer.IndexField(&rbacv1.RoleBinding{}, constants.IndexBySubjects, func(rawObj runtime.Object) []string {
		binding := rawObj.(*rbacv1.RoleBinding)

		subjects := []string{}
		for _, subject := range binding.Subjects {
			subjectID := ConvertSubject(binding.Namespace, &subject)
			if subjectID != "" {
				subjects = append(subjects, subjectID)
			}
		}

		return subjects
	}); err != nil {
		return err
	}

	// Index rolebinding by role ref
	if err := indexer.IndexField(&rbacv1.RoleBinding{}, constants.IndexByRole, func(rawObj runtime.Object) []string {
		binding := rawObj.(*rbacv1.RoleBinding)
		if binding.RoleRef.APIGroup == rbacv1.GroupName && binding.RoleRef.Kind == "Role" {
			return []string{binding.Namespace + "/" + binding.RoleRef.Name}
		}

		return []string{}
	}); err != nil {
		return err
	}

	// Index rolebinding by cluster role ref
	if err := indexer.IndexField(&rbacv1.RoleBinding{}, constants.IndexByClusterRole, func(rawObj runtime.Object) []string {
		binding := rawObj.(*rbacv1.RoleBinding)
		if binding.RoleRef.APIGroup == rbacv1.GroupName && binding.RoleRef.Kind == "ClusterRole" {
			return []string{binding.RoleRef.Name}
		}

		return []string{}
	}); err != nil {
		return err
	}

	// Index cluster role bindings by subjects
	if err := indexer.IndexField(&rbacv1.ClusterRoleBinding{}, constants.IndexBySubjects, func(rawObj runtime.Object) []string {
		binding := rawObj.(*rbacv1.ClusterRoleBinding)

		subjects := []string{}
		for _, subject := range binding.Subjects {
			subjectID := ConvertSubject("", &subject)
			if subjectID != "" {
				subjects = append(subjects, subjectID)
			}
		}

		return subjects
	}); err != nil {
		return err
	}

	// Index cluster role bindings by cluster role
	if err := indexer.IndexField(&rbacv1.ClusterRoleBinding{}, constants.IndexByClusterRole, func(rawObj runtime.Object) []string {
		binding := rawObj.(*rbacv1.ClusterRoleBinding)
		if binding.RoleRef.APIGroup == rbacv1.GroupName && binding.RoleRef.Kind == "ClusterRole" {
			return []string{binding.RoleRef.Name}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
