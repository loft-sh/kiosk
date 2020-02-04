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

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/registry/util"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	"github.com/kiosk-sh/kiosk/pkg/manager/events"
	namespaceutils "github.com/kiosk-sh/kiosk/pkg/util"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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

// +kubebuilder:rbac:groups=config.kiosk.sh,resources=accounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.kiosk.sh,resources=accounts/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;create;update;patch

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

	// Ensure our rolebindings are still correct
	err = r.ensureRoleBindings(ctx, account, namespaceList.Items, log)
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

func (r *AccountReconciler) ensureRoleBindings(ctx context.Context, account *configv1alpha1.Account, namespaces []corev1.Namespace, log logr.Logger) error {
	// List owned role bindings
	roleBindingList := &rbacv1.RoleBindingList{}
	err := r.List(ctx, roleBindingList, client.MatchingFields{constants.IndexByAccount: account.Name})
	if err != nil {
		return err
	}

	// Delete all owned role bindings
	clusterRole := util.GetClusterRoleFor(account)
	if clusterRole == "" || len(account.Spec.Subjects) == 0 {
		for _, rb := range roleBindingList.Items {
			err = r.Delete(ctx, &rb)
			if err != nil {
				return err
			}
		}

		return nil
	}

	// Things to check for
	// 1. Remove multiple owned rolebindings per namespace
	// 2. Remove role bindings from namespaces not owned by the account anymore
	// 3. Update Subjects of RoleBindings if necessary
	// 4. Check if all namespaces have RoleBindings
	createRoleBindings := map[string]bool{}
	deleteRoleBindings := map[string]rbacv1.RoleBinding{}

	// Update role bindings
	expectedRoleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.SchemeGroupVersion.Group,
		Kind:     "ClusterRole",
		Name:     clusterRole,
	}

	// Check which role bindings to update
	for _, rb := range roleBindingList.Items {
		if apiequality.Semantic.DeepEqual(&rb.Subjects, &account.Spec.Subjects) == false || apiequality.Semantic.DeepEqual(&rb.RoleRef, &expectedRoleRef) == false {
			createRoleBindings[rb.Namespace] = true
			deleteRoleBindings[rb.Namespace+"/"+rb.Name] = rb
		}
	}

	// Delete duplicate role bindings in namespace & check if there are namespaces without rolebindings
	for _, n := range namespaces {
		found := false
		for _, rb := range roleBindingList.Items {
			if n.Name == rb.Namespace {
				if _, ok := createRoleBindings[rb.Namespace]; ok {
					deleteRoleBindings[rb.Namespace+"/"+rb.Name] = rb
					found = true
					continue
				}

				if found == true {
					deleteRoleBindings[rb.Namespace+"/"+rb.Name] = rb
				}

				found = true
			}
		}

		if !found && n.Status.Phase == corev1.NamespaceActive && !namespaceutils.IsNamespaceInitializing(&n) {
			createRoleBindings[n.Name] = true
		}
	}

	// Delete role bindings from namespaces that don't belong to the account anymore
	for _, rb := range roleBindingList.Items {
		if _, ok := deleteRoleBindings[rb.Namespace+"/"+rb.Name]; ok {
			continue
		}

		// We delete the role binding if we will create another in the namespace
		if _, ok := createRoleBindings[rb.Namespace]; ok {
			deleteRoleBindings[rb.Namespace+"/"+rb.Name] = rb
			continue
		}

		found := false
		for _, n := range namespaces {
			if n.Name == rb.Namespace {
				found = true
				break
			}
		}

		if !found {
			deleteRoleBindings[rb.Namespace+"/"+rb.Name] = rb
		}
	}

	// Create all new role bindings before we delete the old
	for namespace := range createRoleBindings {
		// We have to create the rolebinding first and then delete the old one
		err = util.CreateRoleBinding(ctx, r, namespace, account, r.Scheme)
		if err != nil {
			return err
		}
	}

	// Delete the rolebindings that are not needed anymore
	for _, rb := range deleteRoleBindings {
		err = r.Delete(ctx, &rb)
		if err != nil {
			if kerrors.IsNotFound(err) {
				continue
			}

			return err
		}
	}

	return nil
}

// SetupWithManager adds the controller to the manager
func (r *AccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(&source.Kind{Type: &corev1.Namespace{}}, &events.NamespaceEventHandler{}).
		Owns(&rbacv1.RoleBinding{}).
		For(&configv1alpha1.Account{}).
		Complete(r)
}
