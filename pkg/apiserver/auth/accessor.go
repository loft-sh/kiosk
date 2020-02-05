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
	"context"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Accessor is the interface for the accessor that retrieves the allowed namespaces & accounts
type Accessor interface {
	RetrieveAllowedNamespaces(ctx context.Context, subject, verb string) ([]string, error)
	RetrieveAllowedAccounts(ctx context.Context, subject, verb string) ([]string, error)
}

// accessor finds the allowed namespaces and accounts for a given subject
type accessor struct {
	client client.Client
}

// RetrieveAllowedNamespaces returns all namespaces the given subject is allowed to access
func (a *accessor) RetrieveAllowedNamespaces(ctx context.Context, subject, verb string) ([]string, error) {
	namespaces := map[string]bool{}

	// Retrieve Cluster Roles
	clusterRoleBindingList := rbacv1.ClusterRoleBindingList{}
	if err := a.client.List(ctx, &clusterRoleBindingList, client.MatchingFields{constants.IndexBySubjects: subject}); err != nil {
		return nil, err
	}

	for _, clusterRoleBinding := range clusterRoleBindingList.Items {
		if clusterRoleBinding.RoleRef.APIGroup == rbacv1.GroupName && clusterRoleBinding.RoleRef.Kind == "ClusterRole" {
			clusterRole := &rbacv1.ClusterRole{}
			err := a.client.Get(ctx, types.NamespacedName{Name: clusterRoleBinding.RoleRef.Name}, clusterRole)
			if err != nil {
				if kerrors.IsNotFound(err) {
					continue
				}

				return nil, err
			}

			for _, rule := range clusterRole.Rules {
				if VerbMatches(&rule, verb) && APIGroupMatches(&rule, corev1.GroupName) && ResourceMatches(&rule, "namespaces", "") {
					if len(rule.ResourceNames) == 0 {
						namespaces[rbacv1.ResourceAll] = true
					} else {
						for _, rn := range rule.ResourceNames {
							namespaces[rn] = true
						}
					}
				}
			}
		}
	}

	// Retrieve Roles
	roleBindingList := rbacv1.RoleBindingList{}
	if err := a.client.List(ctx, &roleBindingList, client.MatchingFields{constants.IndexBySubjects: subject}); err != nil {
		return nil, err
	}

	for _, roleBinding := range roleBindingList.Items {
		if roleBinding.RoleRef.APIGroup == rbacv1.GroupName && roleBinding.RoleRef.Kind == "ClusterRole" {
			clusterRole := &rbacv1.ClusterRole{}
			err := a.client.Get(ctx, types.NamespacedName{Name: roleBinding.RoleRef.Name}, clusterRole)
			if err != nil {
				if kerrors.IsNotFound(err) {
					continue
				}

				return nil, err
			}

			if RulesAllow(authorizer.AttributesRecord{
				Verb:            verb,
				Namespace:       roleBinding.Namespace,
				APIGroup:        corev1.GroupName,
				Resource:        "namespaces",
				Name:            roleBinding.Namespace,
				ResourceRequest: true,
			}, clusterRole.Rules...) {
				namespaces[roleBinding.Namespace] = true
			}
		} else if roleBinding.RoleRef.APIGroup == rbacv1.GroupName && roleBinding.RoleRef.Kind == "Role" {
			role := &rbacv1.Role{}
			err := a.client.Get(ctx, types.NamespacedName{Namespace: roleBinding.Namespace, Name: roleBinding.RoleRef.Name}, role)
			if err != nil {
				if kerrors.IsNotFound(err) {
					continue
				}

				return nil, err
			}

			if RulesAllow(authorizer.AttributesRecord{
				Verb:            verb,
				Namespace:       roleBinding.Namespace,
				APIGroup:        corev1.GroupName,
				Resource:        "namespaces",
				Name:            roleBinding.Namespace,
				ResourceRequest: true,
			}, role.Rules...) {
				namespaces[roleBinding.Namespace] = true
			}
		}
	}

	delete(namespaces, "")

	// Map to []string
	retNamespaces := make([]string, 0, len(namespaces))
	for n := range namespaces {
		retNamespaces = append(retNamespaces, n)
	}

	return retNamespaces, nil
}

// RetrieveAllowedAccounts returns all accounts the given subject is allowed to access by cluster roles
func (a *accessor) RetrieveAllowedAccounts(ctx context.Context, subject, verb string) ([]string, error) {
	accounts := map[string]bool{}

	// Retrieve Cluster Roles
	clusterRoleBindingList := rbacv1.ClusterRoleBindingList{}
	if err := a.client.List(ctx, &clusterRoleBindingList, client.MatchingFields{constants.IndexBySubjects: subject}); err != nil {
		return nil, err
	}

	for _, clusterRoleBinding := range clusterRoleBindingList.Items {
		if clusterRoleBinding.RoleRef.APIGroup == rbacv1.GroupName && clusterRoleBinding.RoleRef.Kind == "ClusterRole" {
			clusterRole := &rbacv1.ClusterRole{}
			err := a.client.Get(ctx, types.NamespacedName{Name: clusterRoleBinding.RoleRef.Name}, clusterRole)
			if err != nil {
				if kerrors.IsNotFound(err) {
					continue
				}

				return nil, err
			}

			for _, rule := range clusterRole.Rules {
				if VerbMatches(&rule, verb) && APIGroupMatches(&rule, configv1alpha1.GroupVersion.Group) && ResourceMatches(&rule, "accounts", "") {
					if len(rule.ResourceNames) == 0 {
						accounts[rbacv1.ResourceAll] = true
					} else {
						for _, rn := range rule.ResourceNames {
							accounts[rn] = true
						}
					}
				}
			}
		}
	}

	delete(accounts, "")

	// Get other subject Accounts for viewing
	if verb == "list" || verb == "get" || verb == "watch" {
		accountList := &configv1alpha1.AccountList{}
		err := a.client.List(ctx, accountList, client.MatchingFields{constants.IndexBySubjects: subject})
		if err != nil {
			return nil, err
		}

		for _, account := range accountList.Items {
			accounts[account.Name] = true
		}
	}

	retAccounts := make([]string, 0, len(accounts))
	for account := range accounts {
		retAccounts = append(retAccounts, account)
	}

	return retAccounts, nil
}
