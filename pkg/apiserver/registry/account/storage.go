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

package account

import (
	"context"
	"fmt"
	"sort"

	config "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy/validation"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/auth"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/registry/util"

	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type accountStorage struct {
	client    client.Client
	authCache auth.Cache
}

// NewAccountStorage creates a new account storage that implements the rest interface
func NewAccountStorage(client client.Client, authCache auth.Cache) rest.Storage {
	return &accountStorage{
		client:    client,
		authCache: authCache,
	}
}

var _ = rest.Scoper(&accountStorage{})

func (r *accountStorage) NamespaceScoped() bool {
	return false
}

var _ = rest.Storage(&accountStorage{})

func (r *accountStorage) New() runtime.Object {
	return &tenancy.Account{}
}

var _ = rest.Lister(&accountStorage{})

func (r *accountStorage) NewList() runtime.Object {
	return &tenancy.AccountList{}
}

func (r *accountStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, kerrors.NewForbidden(tenancy.Resource("accounts"), "", fmt.Errorf("unable to list spaces without a user on the context"))
	}

	accounts, err := r.authCache.GetAccountsForUser(user, "get")
	if err != nil {
		return nil, err
	}

	accountObjects, err := auth.GetAccounts(ctx, r.client, accounts)
	if err != nil {
		return nil, err
	}

	sort.Slice(accountObjects, func(i int, j int) bool {
		return accountObjects[i].Name < accountObjects[j].Name
	})

	accountList := &tenancy.AccountList{
		Items: []tenancy.Account{},
	}

	m := util.MatchAccount(util.ListOptionsToSelectors(options))
	for _, n := range accountObjects {
		match, err := m.Matches(n)
		if err != nil {
			return nil, err
		}
		if match {
			account, err := ConvertConfigAccount(n)
			if err != nil {
				return nil, err
			}

			accountList.Items = append(accountList.Items, *account)
		}
	}

	return accountList, nil
}

var _ = rest.Getter(&accountStorage{})

func (r *accountStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, kerrors.NewForbidden(tenancy.Resource("account"), name, fmt.Errorf("unable to get account without a user on the context"))
	}

	accounts, err := r.authCache.GetAccountsForUser(user, "get")
	if err != nil {
		return nil, err
	}

	allowed := false
	for _, n := range accounts {
		if n == rbacv1.ResourceAll || n == name {
			allowed = true
			break
		}
	}

	if allowed == false {
		return nil, kerrors.NewNotFound(tenancy.Resource("accounts"), name)
		// return nil, kerrors.NewForbidden(tenancy.Resource("accounts"), name, fmt.Errorf("cannot get account because user is not allowed to"))
	}

	configAccount := &config.Account{}
	err = r.client.Get(ctx, types.NamespacedName{Name: name}, configAccount)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, kerrors.NewNotFound(tenancy.Resource("accounts"), name)
		}

		return nil, err
	}

	return ConvertConfigAccount(configAccount)
}

var _ = rest.Creater(&accountStorage{})

func (r *accountStorage) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, kerrors.NewForbidden(tenancy.Resource("accounts"), "", fmt.Errorf("unable to create account without a user on the context"))
	}

	account, ok := obj.(*tenancy.Account)
	if !ok {
		return nil, fmt.Errorf("not an account: %#v", obj)
	}

	// Validation phase
	rest.FillObjectMetaSystemFields(&account.ObjectMeta)
	errs := validation.ValidateAccount(account)
	if len(errs) > 0 {
		return nil, kerrors.NewInvalid(tenancy.Kind("Account"), account.Name, errs)
	}
	if err := createValidation(ctx, obj); err != nil {
		return nil, err
	}

	accounts, err := r.authCache.GetAccountsForUser(user, "create")
	if err != nil {
		return nil, err
	}

	allowed := false
	if len(accounts) > 0 {
		for _, a := range accounts {
			if a == rbacv1.ResourceAll || a == account.Name {
				allowed = true
				break
			}
		}
	}

	if !allowed {
		return nil, kerrors.NewForbidden(tenancy.Resource("accounts"), account.Name, fmt.Errorf("user cannot create account"))
	}

	configAccount, err := ConvertTenancyAccount(account)
	if err != nil {
		return nil, err
	}

	err = r.client.Create(ctx, configAccount, &client.CreateOptions{
		Raw: options,
	})
	if err != nil {
		return nil, err
	}

	return ConvertConfigAccount(configAccount)
}

var _ = rest.Updater(&accountStorage{})
var _ = rest.CreaterUpdater(&accountStorage{})

func (r *accountStorage) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, false, kerrors.NewForbidden(tenancy.Resource("accounts"), "", fmt.Errorf("unable to create account without a user on the context"))
	}

	accounts, err := r.authCache.GetAccountsForUser(user, "update")
	if err != nil {
		return nil, false, err
	}

	allowed := false
	if len(accounts) > 0 {
		for _, a := range accounts {
			if a == rbacv1.ResourceAll || a == name {
				allowed = true
				break
			}
		}
	}

	if !allowed {
		return nil, false, kerrors.NewForbidden(tenancy.Resource("accounts"), name, fmt.Errorf("user cannot update account"))
	}

	oldObj, err := r.Get(ctx, name, nil)
	if err != nil {
		return nil, false, err
	}

	oldAccount, ok := oldObj.(*tenancy.Account)
	if !ok {
		return nil, false, fmt.Errorf("Old object is not an account")
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}

	newAccount, ok := newObj.(*tenancy.Account)
	if !ok {
		return nil, false, fmt.Errorf("New object is not an account")
	}

	errs := validation.ValidateAccountUpdate(newAccount, oldAccount)
	if len(errs) > 0 {
		return nil, false, kerrors.NewInvalid(tenancy.Kind("Account"), newAccount.Name, errs)
	}
	err = updateValidation(ctx, newObj, oldObj)
	if err != nil {
		return nil, false, err
	}

	newConfigAccount, err := ConvertTenancyAccount(newAccount)
	if err != nil {
		return nil, false, err
	}

	err = r.client.Update(ctx, newConfigAccount)
	if err != nil {
		return nil, false, err
	}

	return newAccount, true, nil
}

var _ = rest.GracefulDeleter(&accountStorage{})

func (r *accountStorage) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, false, kerrors.NewForbidden(tenancy.Resource("accounts"), "", fmt.Errorf("unable to create account without a user on the context"))
	}

	accounts, err := r.authCache.GetAccountsForUser(user, "delete")
	if err != nil {
		return nil, false, err
	}

	allowed := false
	if len(accounts) > 0 {
		for _, a := range accounts {
			if a == rbacv1.ResourceAll || a == name {
				allowed = true
				break
			}
		}
	}

	if !allowed {
		return nil, false, kerrors.NewForbidden(tenancy.Resource("accounts"), name, fmt.Errorf("user cannot delete account"))
	}

	configAccount := &config.Account{}
	err = r.client.Get(ctx, types.NamespacedName{Name: name}, configAccount)
	if err != nil {
		return nil, false, err
	}

	err = r.client.Delete(ctx, configAccount, &client.DeleteOptions{
		Raw: options,
	})
	if err != nil {
		return nil, false, err
	}

	account, err := ConvertConfigAccount(configAccount)
	if err != nil {
		return nil, false, err
	}

	return account, true, err
}
