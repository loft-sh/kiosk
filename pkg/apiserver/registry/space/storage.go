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

package space

import (
	"context"
	"fmt"
	"sort"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy/validation"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/auth"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/registry/util"
	"github.com/kiosk-sh/kiosk/pkg/constants"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type spaceStorage struct {
	scheme    *runtime.Scheme
	authCache auth.Cache
	client    client.Client
}

// NewSpaceStorage creates a new space storage that implements the rest interface
func NewSpaceStorage(client client.Client, authCache auth.Cache, scheme *runtime.Scheme) rest.Storage {
	return &spaceStorage{
		scheme:    scheme,
		authCache: authCache,
		client:    client,
	}
}

var _ = rest.Scoper(&spaceStorage{})

func (r *spaceStorage) NamespaceScoped() bool {
	return false
}

var _ = rest.Storage(&spaceStorage{})

func (r *spaceStorage) New() runtime.Object {
	return &tenancy.Space{}
}

var _ = rest.Lister(&spaceStorage{})

func (r *spaceStorage) NewList() runtime.Object {
	return &tenancy.SpaceList{}
}

func (r *spaceStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, kerrors.NewForbidden(tenancy.Resource("spaces"), "", fmt.Errorf("unable to list spaces without a user on the context"))
	}

	namespaces, err := r.authCache.GetNamespacesForUser(user, "get")
	if err != nil {
		return nil, err
	}

	namespaceObjects, err := auth.GetNamespaces(ctx, r.client, namespaces)
	if err != nil {
		return nil, err
	}

	sort.Slice(namespaceObjects, func(i int, j int) bool {
		return namespaceObjects[i].Name < namespaceObjects[j].Name
	})

	spaceList := &tenancy.SpaceList{
		Items: []tenancy.Space{},
	}

	m := util.MatchNamespace(util.ListOptionsToSelectors(options))
	for _, n := range namespaceObjects {
		match, err := m.Matches(n)
		if err != nil {
			return nil, err
		}
		if match {
			spaceList.Items = append(spaceList.Items, *ConvertNamespace(n))
		}
	}

	return spaceList, nil
}

var _ = rest.Getter(&spaceStorage{})

func (r *spaceStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, kerrors.NewForbidden(tenancy.Resource("spaces"), name, fmt.Errorf("unable to get space without a user on the context"))
	}

	namespaces, err := r.authCache.GetNamespacesForUser(user, "get")
	if err != nil {
		return nil, err
	}

	allowed := false
	if len(namespaces) > 0 {
		if namespaces[0] == "*" {
			allowed = true
		} else {
			for _, n := range namespaces {
				if n == name {
					allowed = true
					break
				}
			}
		}
	}

	if allowed == false {
		return nil, kerrors.NewNotFound(tenancy.Resource("spaces"), name)
		// return nil, kerrors.NewForbidden(tenancy.Resource("spaces"), name, fmt.Errorf("cannot get space because user is not allowed to"))
	}

	namespaceObj := &corev1.Namespace{}
	err = r.client.Get(ctx, types.NamespacedName{Name: name}, namespaceObj)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, kerrors.NewNotFound(tenancy.Resource("spaces"), name)
		}

		return nil, err
	}

	return ConvertNamespace(namespaceObj), nil
}

var _ = rest.Creater(&spaceStorage{})

func (r *spaceStorage) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, kerrors.NewForbidden(tenancy.Resource("spaces"), "", fmt.Errorf("unable to create space without a user on the context"))
	}

	space, ok := obj.(*tenancy.Space)
	if !ok {
		return nil, fmt.Errorf("not a space: %#v", obj)
	}

	// Validation phase
	rest.FillObjectMetaSystemFields(&space.ObjectMeta)
	errs := validation.ValidateSpace(space)
	if len(errs) > 0 {
		return nil, kerrors.NewInvalid(tenancy.Kind("Space"), space.Name, errs)
	}
	if err := createValidation(ctx, obj); err != nil {
		return nil, err
	}

	// A user can create a space if:
	// 1. He is part of the requesting account and below the space limit
	// 2. He can create namespaces and account field is empty
	namespaces, err := r.authCache.GetNamespacesForUser(user, "create")
	if err != nil {
		return nil, err
	}

	// Check if user could create namespace
	canCreate := false
	if len(namespaces) > 0 && space.Spec.Account == "" {
		if namespaces[0] == rbacv1.ResourceAll {
			canCreate = true
		} else {
			for _, n := range namespaces {
				if n == space.Name {
					canCreate = true
					break
				}
			}
		}
	}

	// Check if user can access account and create space
	var account *configv1alpha1.Account
	if canCreate == false {
		accounts, err := r.authCache.GetAccountsForUser(user, "get")
		if err != nil {
			return nil, err
		}

		if len(accounts) > 0 {
			if space.Spec.Account == "" {
				if accounts[0] == rbacv1.ResourceAll || len(accounts) > 1 {
					return nil, kerrors.NewInvalid(tenancy.Kind("Space"), space.Name, field.ErrorList{field.Invalid(field.NewPath("spec").Key("account"), "", "account must be specified")})
				}

				space.Spec.Account = accounts[0]
			}

			// Find account
			account = &configv1alpha1.Account{}
			err := r.client.Get(ctx, types.NamespacedName{Name: space.Spec.Account}, account)
			if err != nil {
				return nil, kerrors.NewInvalid(tenancy.Kind("Space"), space.Name, field.ErrorList{field.Invalid(field.NewPath("spec").Key("account"), space.Spec.Account, "account does not exist")})
			}

			// Check if user can access account
			canAccessAccount := false
			for _, a := range accounts {
				if a == rbacv1.ResourceAll || a == space.Spec.Account {
					canAccessAccount = true
					break
				}
			}
			if canAccessAccount == false {
				return nil, kerrors.NewInvalid(tenancy.Kind("Space"), space.Name, field.ErrorList{field.Invalid(field.NewPath("spec").Key("account"), space.Spec.Account, "user cannot access account")})
			}

			// Check if account is at limit
			if account.Spec.SpaceLimit != nil {
				namespaceList := &corev1.NamespaceList{}
				err := r.client.List(ctx, namespaceList, client.MatchingFields{constants.IndexByAccount: account.Name})
				if err != nil {
					return nil, err
				}

				if len(namespaceList.Items) >= *account.Spec.SpaceLimit {
					return nil, kerrors.NewForbidden(tenancy.Resource("spaces"), space.Name, fmt.Errorf("space limit of %d reached for account %s", *account.Spec.SpaceLimit, account.Name))
				}
			}

			canCreate = true
		}
	}

	if canCreate == false {
		return nil, kerrors.NewForbidden(tenancy.Resource("spaces"), space.Name, fmt.Errorf("user is not allowed to create space"))
	}

	// Create the target namespace
	namespace := ConvertSpace(space)
	if len(options.DryRun) == 0 && account != nil {
		namespace.Annotations[tenancy.SpaceAnnotationInitializing] = "true"
		err = r.client.Create(ctx, namespace, &client.CreateOptions{
			Raw: options,
		})
		if err != nil {
			return nil, err
		}

		// Create the default space templates and role binding
		err = r.initializeSpace(ctx, namespace, account)
		if err != nil {
			r.client.Delete(ctx, namespace)
			return nil, err
		}
	} else {
		err = r.client.Create(ctx, namespace, &client.CreateOptions{
			Raw: options,
		})
		if err != nil {
			return nil, err
		}
	}

	// Wait till we have the namespace in the cache
	err = r.waitForAccess(ctx, namespace.Name)
	if err != nil {
		return nil, err
	}

	return ConvertNamespace(namespace), nil
}

func (r *spaceStorage) initializeSpace(ctx context.Context, namespace *corev1.Namespace, account *configv1alpha1.Account) error {
	// Create template instances
	templateInstances := []*configv1alpha1.TemplateInstance{}
	for _, instSpec := range account.Spec.SpaceDefaultTemplates {
		if instSpec.Template == "" {
			continue
		}

		templateInstance := &configv1alpha1.TemplateInstance{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: instSpec.Template + "-",
				Namespace:    namespace.Name,
			},
			Spec: instSpec,
		}

		err := r.client.Create(ctx, templateInstance)
		if err != nil {
			return err
		}

		templateInstances = append(templateInstances, templateInstance)
	}

	// Wait for template instances to be deployed
	backoff := retry.DefaultBackoff
	backoff.Steps = 8
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		// Get the template instances
		instanceList := &configv1alpha1.TemplateInstanceList{}
		err := r.client.List(ctx, instanceList, client.InNamespace(namespace.Name))
		if err != nil {
			return false, err
		}

		// Check if instances are ready
		for _, inst := range templateInstances {
			found := false
			for _, realInst := range instanceList.Items {
				if inst.Name == realInst.Name {
					found = true
					if realInst.Status.Status == configv1alpha1.TemplateInstanceDeploymentStatusPending {
						return false, nil
					} else if realInst.Status.Status == configv1alpha1.TemplateInstanceDeploymentStatusFailed {
						return false, fmt.Errorf("TemplateInstance '%s' failed with %s: %s", realInst.Name, realInst.Status.Reason, realInst.Status.Message)
					}
				}
			}

			if !found {
				return false, nil
			}
		}

		// if we reach this point we found all instances and they are deployed
		return true, nil
	})
	if err != nil {
		return err
	}

	// Create role binding
	err = util.CreateRoleBinding(ctx, r.client, namespace.Name, account, r.scheme)
	if err != nil {
		return err
	}

	// Update namespace intialization
	delete(namespace.Annotations, tenancy.SpaceAnnotationInitializing)
	err = r.client.Update(ctx, namespace)
	if err != nil {
		return err
	}

	return nil
}

// waitForAccess blocks until the apiserver says the user has access to the namespace
func (r *spaceStorage) waitForAccess(ctx context.Context, namespace string) error {
	backoff := retry.DefaultBackoff
	backoff.Steps = 6 // this effectively waits for 6-ish seconds
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := r.client.Get(ctx, types.NamespacedName{Name: namespace}, &corev1.Namespace{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				return false, nil
			}

			return false, err
		}

		return true, nil
	})

	return err
}

var _ = rest.Updater(&spaceStorage{})
var _ = rest.CreaterUpdater(&spaceStorage{})

func (r *spaceStorage) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, false, kerrors.NewForbidden(tenancy.Resource("spaces"), name, fmt.Errorf("unable to create space without a user on the context"))
	}

	namespaces, err := r.authCache.GetNamespacesForUser(user, "update")
	if err != nil {
		return nil, false, err
	}

	allowed := false
	if len(namespaces) > 0 {
		for _, n := range namespaces {
			if n == rbacv1.ResourceAll || n == name {
				allowed = true
				break
			}
		}
	}

	if allowed == false {
		return nil, false, kerrors.NewForbidden(tenancy.Resource("spaces"), name, fmt.Errorf("user is not allowed to update space"))
	}

	oldObj, err := r.Get(ctx, name, nil)
	if err != nil {
		return nil, false, err
	}

	oldSpace, ok := oldObj.(*tenancy.Space)
	if !ok {
		return nil, false, fmt.Errorf("Old object is not a space")
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}

	newSpace, ok := newObj.(*tenancy.Space)
	if !ok {
		return nil, false, fmt.Errorf("New object is not a space")
	}

	errs := validation.ValidateSpaceUpdate(newSpace, oldSpace)
	if len(errs) > 0 {
		return nil, false, kerrors.NewInvalid(tenancy.Kind("Space"), newSpace.Name, errs)
	}
	err = updateValidation(ctx, newObj, oldObj)
	if err != nil {
		return nil, false, err
	}

	namespace := ConvertSpace(newSpace)
	err = r.client.Update(ctx, namespace)
	if err != nil {
		return nil, false, err
	}

	return newSpace, true, nil
}

var _ = rest.GracefulDeleter(&spaceStorage{})

func (r *spaceStorage) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, false, kerrors.NewForbidden(tenancy.Resource("spaces"), name, fmt.Errorf("unable to create space without a user on the context"))
	}

	namespaces, err := r.authCache.GetNamespacesForUser(user, "delete")
	if err != nil {
		return nil, false, err
	}

	allowed := false
	if len(namespaces) > 0 {
		for _, n := range namespaces {
			if n == rbacv1.ResourceAll || n == name {
				allowed = true
				break
			}
		}
	}

	if allowed == false {
		return nil, false, kerrors.NewForbidden(tenancy.Resource("spaces"), name, fmt.Errorf("user is not allowed to delete space"))
	}

	namespace := &corev1.Namespace{}
	err = r.client.Get(ctx, types.NamespacedName{Name: name}, namespace)
	if err != nil {
		return nil, false, err
	}

	err = r.client.Delete(ctx, namespace, &client.DeleteOptions{
		Raw: options,
	})
	if err != nil {
		return nil, false, err
	}

	return ConvertNamespace(namespace), true, nil
}
