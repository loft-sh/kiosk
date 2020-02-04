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
	"log"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func invalidateClusterRoleBinding(clusterRoleBinding *rbacv1.ClusterRoleBinding, enqueue EnqueueSubject) {
	for _, subject := range clusterRoleBinding.Subjects {
		subjectID := ConvertSubject("", &subject)
		if subjectID != "" {
			enqueue(subjectID)
		}
	}
}

func invalidateRoleBinding(roleBinding *rbacv1.RoleBinding, enqueue EnqueueSubject) {
	for _, subject := range roleBinding.Subjects {
		subjectID := ConvertSubject(roleBinding.Namespace, &subject)
		if subjectID != "" {
			enqueue(subjectID)
		}
	}
}

func invalidateAccount(account *configv1alpha1.Account, enqueue EnqueueSubject) {
	for _, subject := range account.Spec.Subjects {
		subjectID := ConvertSubject("", &subject)
		if subjectID != "" {
			enqueue(subjectID)
		}
	}
}

type AccountHandler struct {
	enqueue EnqueueSubject
}

// OnAdd implements interface
func (r *AccountHandler) OnAdd(obj interface{}) {
	account, ok := obj.(*configv1alpha1.Account)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateAccount(account, r.enqueue)
}

// OnUpdate implements interface
func (r *AccountHandler) OnUpdate(oldObj, newObj interface{}) {
	// Invalidate old
	account, ok := oldObj.(*configv1alpha1.Account)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateAccount(account, r.enqueue)

	// Invalidate new
	account, ok = newObj.(*configv1alpha1.Account)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateAccount(account, r.enqueue)
}

// OnDelete implements interface
func (r *AccountHandler) OnDelete(obj interface{}) {
	account, ok := obj.(*configv1alpha1.Account)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateAccount(account, r.enqueue)
}

type RoleBindingHandler struct {
	enqueue EnqueueSubject
}

// OnAdd implements interface
func (r *RoleBindingHandler) OnAdd(obj interface{}) {
	roleBinding, ok := obj.(*rbacv1.RoleBinding)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateRoleBinding(roleBinding, r.enqueue)
}

// OnUpdate implements interface
func (r *RoleBindingHandler) OnUpdate(oldObj, newObj interface{}) {
	// Invalidate old
	roleBinding, ok := oldObj.(*rbacv1.RoleBinding)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateRoleBinding(roleBinding, r.enqueue)

	// Invalidate new
	roleBinding, ok = newObj.(*rbacv1.RoleBinding)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateRoleBinding(roleBinding, r.enqueue)
}

// OnDelete implements interface
func (r *RoleBindingHandler) OnDelete(obj interface{}) {
	roleBinding, ok := obj.(*rbacv1.RoleBinding)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateRoleBinding(roleBinding, r.enqueue)
}

type RoleHandler struct {
	client  client.Client
	enqueue EnqueueSubject
}

// OnAdd implements interface
func (r *RoleHandler) OnAdd(obj interface{}) {
	ctx := context.Background()
	role, ok := obj.(*rbacv1.Role)
	if !ok {
		panic("Supplied object has wrong type")
	}

	roleBindingList := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, roleBindingList, client.MatchingFields{constants.IndexByRole: role.Namespace + "/" + role.Name}); err != nil {
		log.Println("Error listing role bindings: " + err.Error())
		return
	}

	for _, roleBinding := range roleBindingList.Items {
		invalidateRoleBinding(&roleBinding, r.enqueue)
	}
}

// OnUpdate implements interface
func (r *RoleHandler) OnUpdate(oldObj, newObj interface{}) {
	ctx := context.Background()
	role, ok := newObj.(*rbacv1.Role)
	if !ok {
		panic("Supplied object has wrong type")
	}

	roleBindingList := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, roleBindingList, client.MatchingFields{constants.IndexByRole: role.Namespace + "/" + role.Name}); err != nil {
		log.Println("Error listing role bindings: " + err.Error())
		return
	}

	for _, roleBinding := range roleBindingList.Items {
		invalidateRoleBinding(&roleBinding, r.enqueue)
	}
}

// OnDelete implements interface
func (r *RoleHandler) OnDelete(obj interface{}) {
	ctx := context.Background()
	role, ok := obj.(*rbacv1.Role)
	if !ok {
		panic("Supplied object has wrong type")
	}

	roleBindingList := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, roleBindingList, client.MatchingFields{constants.IndexByRole: role.Namespace + "/" + role.Name}); err != nil {
		log.Println("Error listing role bindings: " + err.Error())
		return
	}

	for _, roleBinding := range roleBindingList.Items {
		invalidateRoleBinding(&roleBinding, r.enqueue)
	}
}

type ClusterRoleBindingHandler struct {
	enqueue EnqueueSubject
}

// OnAdd implements interface
func (r *ClusterRoleBindingHandler) OnAdd(obj interface{}) {
	clusterRoleBinding, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateClusterRoleBinding(clusterRoleBinding, r.enqueue)
}

// OnUpdate implements interface
func (r *ClusterRoleBindingHandler) OnUpdate(oldObj, newObj interface{}) {
	// Invalidate old
	clusterRoleBinding, ok := oldObj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateClusterRoleBinding(clusterRoleBinding, r.enqueue)

	// Invalidate new
	clusterRoleBinding, ok = newObj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateClusterRoleBinding(clusterRoleBinding, r.enqueue)
}

// OnDelete implements interface
func (r *ClusterRoleBindingHandler) OnDelete(obj interface{}) {
	clusterRoleBinding, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		panic("Supplied object has wrong type")
	}

	invalidateClusterRoleBinding(clusterRoleBinding, r.enqueue)
}

type ClusterRoleHandler struct {
	enqueue EnqueueSubject
	client  client.Client
}

// OnAdd implements interface
func (r *ClusterRoleHandler) OnAdd(obj interface{}) {
	ctx := context.Background()
	role, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		panic("Supplied object has wrong type")
	}

	// Invalidate role bindings
	roleBindingList := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, roleBindingList, client.MatchingFields{constants.IndexByClusterRole: role.Name}); err != nil {
		log.Println("Error listing role bindings: " + err.Error())
		return
	}

	for _, roleBinding := range roleBindingList.Items {
		invalidateRoleBinding(&roleBinding, r.enqueue)
	}

	// Invalidate cluster role bindings
	clusterRoleBindingList := &rbacv1.ClusterRoleBindingList{}
	if err := r.client.List(ctx, clusterRoleBindingList, client.MatchingFields{constants.IndexByClusterRole: role.Name}); err != nil {
		log.Println("Error listing cluster role bindings: " + err.Error())
		return
	}

	for _, clusterRoleBinding := range clusterRoleBindingList.Items {
		invalidateClusterRoleBinding(&clusterRoleBinding, r.enqueue)
	}
}

// OnUpdate implements interface
func (r *ClusterRoleHandler) OnUpdate(oldObj, newObj interface{}) {
	ctx := context.Background()
	role, ok := newObj.(*rbacv1.ClusterRole)
	if !ok {
		panic("Supplied object has wrong type")
	}

	// Invalidate role bindings
	roleBindingList := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, roleBindingList, client.MatchingFields{constants.IndexByClusterRole: role.Name}); err != nil {
		log.Println("Error listing role bindings: " + err.Error())
		return
	}

	for _, roleBinding := range roleBindingList.Items {
		invalidateRoleBinding(&roleBinding, r.enqueue)
	}

	// Invalidate cluster role bindings
	clusterRoleBindingList := &rbacv1.ClusterRoleBindingList{}
	if err := r.client.List(ctx, clusterRoleBindingList, client.MatchingFields{constants.IndexByClusterRole: role.Name}); err != nil {
		log.Println("Error listing cluster role bindings: " + err.Error())
		return
	}

	for _, clusterRoleBinding := range clusterRoleBindingList.Items {
		invalidateClusterRoleBinding(&clusterRoleBinding, r.enqueue)
	}
}

// OnDelete implements interface
func (r *ClusterRoleHandler) OnDelete(obj interface{}) {
	ctx := context.Background()
	role, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		panic("Supplied object has wrong type")
	}

	// Invalidate role bindings
	roleBindingList := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, roleBindingList, client.MatchingFields{constants.IndexByClusterRole: role.Name}); err != nil {
		log.Println("Error listing role bindings: " + err.Error())
		return
	}

	for _, roleBinding := range roleBindingList.Items {
		invalidateRoleBinding(&roleBinding, r.enqueue)
	}

	// Invalidate cluster role bindings
	clusterRoleBindingList := &rbacv1.ClusterRoleBindingList{}
	if err := r.client.List(ctx, clusterRoleBindingList, client.MatchingFields{constants.IndexByClusterRole: role.Name}); err != nil {
		log.Println("Error listing cluster role bindings: " + err.Error())
		return
	}

	for _, clusterRoleBinding := range clusterRoleBindingList.Items {
		invalidateClusterRoleBinding(&clusterRoleBinding, r.enqueue)
	}
}
