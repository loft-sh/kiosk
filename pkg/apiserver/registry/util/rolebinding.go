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

package util

import (
	"context"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetClusterRoleFor returns the cluster role for the given account if there is any
func GetClusterRoleFor(account *configv1alpha1.Account) string {
	clusterRole := "admin"
	if account.Spec.Space.ClusterRole != nil {
		clusterRole = *account.Spec.Space.ClusterRole
	}

	return clusterRole
}

// CreateRoleBinding creates a new role binding for the target account with the given name and namespace
func CreateRoleBinding(ctx context.Context, client client.Client, namespace string, owner *configv1alpha1.Account, scheme *runtime.Scheme) error {
	clusterRole := GetClusterRoleFor(owner)
	if clusterRole == "" {
		// If the account has no cluster role we just return
		return nil
	}

	// Create role binding
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: owner.Name + "-",
			Namespace:    namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     clusterRole,
		},
		Subjects: owner.Spec.Subjects,
	}

	// Set owner controller
	err := ctrl.SetControllerReference(owner, roleBinding, scheme)
	if err != nil {
		return err
	}

	// Create the actual role binding in the cluster
	err = client.Create(ctx, roleBinding)
	if err != nil {
		return err
	}

	return nil
}
