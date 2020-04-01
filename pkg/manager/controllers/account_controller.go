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
	"context"
	tenancyv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/tenancy/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/util/clienthelper"
	"github.com/kiosk-sh/kiosk/pkg/util/clusterrole"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	"github.com/kiosk-sh/kiosk/pkg/manager/events"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

// AccountReconciler reconciles a Account object
type AccountReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for an Account object and makes changes based on the state read
func (r *AccountReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("account", req.NamespacedName)

	log.Info("Account reconcile started")

	// Retrieve account
	account := &configv1alpha1.Account{}
	if err := r.Get(ctx, req.NamespacedName, account); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Get owned namespaces
	namespaceList := &corev1.NamespaceList{}
	err := r.List(ctx, namespaceList, client.MatchingFields{constants.IndexByAccount: account.Name})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure our clusterroles are still correct
	err = r.syncClusterRoles(ctx, account, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update account status
	oldStatus := account.Status.DeepCopy()

	account.Status.Namespaces = []configv1alpha1.AccountNamespaceStatus{}
	for _, namespace := range namespaceList.Items {
		account.Status.Namespaces = append(account.Status.Namespaces, configv1alpha1.AccountNamespaceStatus{Name: namespace.Name})
	}

	// Update status
	// Do a semantic deep equal since we compare resource quantities as well.
	// See https://github.com/kubernetes/apimachinery/issues/75 for more information
	if apiequality.Semantic.DeepEqual(&oldStatus, &account.Status) == false {
		err = r.Status().Update(ctx, account)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *AccountReconciler) syncClusterRoles(ctx context.Context, account *configv1alpha1.Account, log logr.Logger) error {
	// Ensure cluster role
	clusterRoleList := &rbacv1.ClusterRoleList{}
	err := r.List(ctx, clusterRoleList, client.MatchingFields{constants.IndexByAccount: account.Name})
	if err != nil {
		return err
	}

	newRules := r.getClusterRoleRules(account)
	clusterRoles, err := clusterrole.SyncClusterRoles(ctx, r.Client, clusterRoleList.Items, newRules, true)
	if err != nil {
		return err
	} else if len(clusterRoles) == 0 {
		// Create new cluster role
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kiosk-account-" + account.Name + "-",
			},
			Rules: newRules,
		}

		err = clienthelper.CreateWithOwner(ctx, r.Client, clusterRole, account, r.Scheme)
		if err != nil {
			return err
		}

		clusterRoles = []rbacv1.ClusterRole{*clusterRole}
	}

	// Get relevant cluster role bindings
	clusterRoleBindingsList := &rbacv1.ClusterRoleBindingList{}
	err = r.List(ctx, clusterRoleBindingsList, client.MatchingFields{constants.IndexByAccount: account.Name})
	if err != nil {
		return err
	}

	// sync cluster role bindings
	bindings, err := clusterrole.SyncClusterRoleBindings(ctx, r.Client, clusterRoleBindingsList.Items, clusterRoles[0].Name, account.Spec.Subjects, true)
	if err != nil {
		return err
	} else if len(bindings) == 0 {
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kiosk-account-" + account.Name + "-",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRoles[0].Name,
			},
			Subjects: account.Spec.Subjects,
		}

		err = clienthelper.CreateWithOwner(ctx, r.Client, clusterRoleBinding, account, r.Scheme)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *AccountReconciler) getClusterRoleRules(account *configv1alpha1.Account) []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		{
			Verbs:     []string{"list"},
			APIGroups: []string{tenancyv1alpha1.SchemeGroupVersion.Group},
			Resources: []string{"accounts", "spaces"},
		},
		{
			Verbs:     []string{"get"},
			APIGroups: []string{tenancyv1alpha1.SchemeGroupVersion.Group},
			Resources: []string{"spaces"},
		},
		{
			Verbs:         []string{"get"},
			APIGroups:     []string{tenancyv1alpha1.SchemeGroupVersion.Group},
			Resources:     []string{"accounts"},
			ResourceNames: []string{account.Name},
		},
	}

	return rules
}

// SetupWithManager adds the controller to the manager
func (r *AccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(&source.Kind{Type: &corev1.Namespace{}}, &events.NamespaceEventHandler{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Owns(&rbacv1.ClusterRole{}).
		For(&configv1alpha1.Account{}).
		Complete(r)
}
