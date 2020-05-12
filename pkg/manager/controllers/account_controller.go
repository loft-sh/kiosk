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
	"github.com/kiosk-sh/kiosk/pkg/util"
	"github.com/kiosk-sh/kiosk/pkg/util/clienthelper"
	"github.com/kiosk-sh/kiosk/pkg/util/clusterrole"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"
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

	// Ensure our role bindings have the correct subjects
	err = r.syncRoleBindings(ctx, account, log)
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

func (r *AccountReconciler) syncRoleBindings(ctx context.Context, account *configv1alpha1.Account, log logr.Logger) error {
	roleBindings := &rbacv1.RoleBindingList{}
	err := r.List(ctx, roleBindings, client.MatchingFields{constants.IndexByAccount: account.Name})
	if err != nil {
		return err
	}

	for _, crb := range roleBindings.Items {
		// Update role binding
		if apiequality.Semantic.DeepEqual(crb.Subjects, account.Spec.Subjects) == false {
			crb.Subjects = account.Spec.Subjects
			err := r.Update(ctx, &crb)
			if err != nil {
				return err
			}

			log.V(1).Info("updated role binding " + crb.Namespace + "/" + crb.Name)
		}
	}

	return nil
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
				GenerateName: RBACGenerateName(account),
			},
			Rules: newRules,
		}

		err = clienthelper.CreateWithOwner(ctx, r.Client, clusterRole, account, r.Scheme)
		if err != nil {
			return err
		}

		log.Info("Created cluster role " + clusterRole.Name)
		clusterRoles = []rbacv1.ClusterRole{*clusterRole}
	}

	// Get relevant cluster role bindings
	clusterRoleBindingsList := &rbacv1.ClusterRoleBindingList{}
	err = r.List(ctx, clusterRoleBindingsList, client.MatchingFields{constants.IndexByAccount: account.Name})
	if err != nil {
		return err
	}

	// sync cluster role bindings
	// tasks:
	// - make sure all cluster role bindings we own have the correct subjects
	// - make sure there is at least one cluster role binding for the cluster role ensured above
	found := false
	for _, crb := range clusterRoleBindingsList.Items {
		if crb.RoleRef.Name == clusterRoles[0].Name {
			found = true
		}

		// Update cluster role binding if subjects differ
		if apiequality.Semantic.DeepEqual(crb.Subjects, account.Spec.Subjects) == false {
			crb.Subjects = account.Spec.Subjects
			err := r.Client.Update(ctx, &crb)
			if err != nil {
				return err
			}

			log.Info("Updated cluster role binding " + crb.Name)
		}
	}

	// if no cluster role binding was found for the above role, then create one
	if !found {
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: RBACGenerateName(account),
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

		log.Info("Created cluster role binding " + clusterRoleBinding.Name)
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

// NamespaceEventHandler handles events
type NamespaceEventHandler struct {
	Log logr.Logger
}

// Create implements EventHandler
func (e *NamespaceEventHandler) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.handleEvent(evt.Meta, q)
}

// Update implements EventHandler
func (e *NamespaceEventHandler) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	if util.GetAccountFromNamespace(evt.MetaOld) == util.GetAccountFromNamespace(evt.MetaNew) {
		return
	}

	e.handleEvent(evt.MetaOld, q)
	e.handleEvent(evt.MetaNew, q)
}

// Delete implements EventHandler
func (e *NamespaceEventHandler) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.handleEvent(evt.Meta, q)
}

// Generic implements EventHandler
func (e *NamespaceEventHandler) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.handleEvent(evt.Meta, q)
}

func (e *NamespaceEventHandler) handleEvent(meta metav1.Object, q workqueue.RateLimitingInterface) {
	if meta == nil {
		return
	}

	labels := meta.GetLabels()
	if labels == nil {
		return
	}

	if owner, ok := labels[constants.SpaceLabelAccount]; ok && owner != "" {
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Name: owner,
		}})
	}
}

// SetupWithManager adds the controller to the manager
func (r *AccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(&source.Kind{Type: &corev1.Namespace{}}, &NamespaceEventHandler{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.RoleBinding{}).
		For(&configv1alpha1.Account{}).
		Complete(r)
}

func RBACGenerateName(account *configv1alpha1.Account) string {
	name := account.Name
	if len(name) > 42 {
		name = name[:42]
	}

	return "kiosk-account-" + name + "-"
}
