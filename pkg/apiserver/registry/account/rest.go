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
	"github.com/kiosk-sh/kiosk/kube/plugin/pkg/auth/authorizer/rbac"
	config "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"github.com/kiosk-sh/kiosk/pkg/authorization"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/registry/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type accountREST struct {
	client client.Client
	filter authorization.FilteredLister
}

// NewAccountREST creates a new account storage that implements the rest interface
func NewAccountREST(client client.Client, scheme *runtime.Scheme) rest.Storage {
	ruleClient := authorization.NewRuleClient(client)
	return &accountREST{
		client: client,
		filter: authorization.NewFilteredLister(client, rbac.New(ruleClient, ruleClient, ruleClient, ruleClient)),
	}
}

var _ = rest.Scoper(&accountREST{})

func (r *accountREST) NamespaceScoped() bool {
	return false
}

var _ = rest.Storage(&accountREST{})

func (r *accountREST) New() runtime.Object {
	return &tenancy.Account{}
}

var _ = rest.Lister(&accountREST{})

func (r *accountREST) NewList() runtime.Object {
	return &tenancy.AccountList{}
}

func (r *accountREST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	configAccountList := &config.AccountList{}
	_, err := r.filter.List(ctx, configAccountList, tenancy.SchemeGroupVersion.WithResource("accounts"), options)
	if err != nil {
		return nil, err
	}

	accountList := &tenancy.AccountList{
		Items: []tenancy.Account{},
	}
	for _, n := range configAccountList.Items {
		account, err := ConvertConfigAccount(&n)
		if err != nil {
			return nil, err
		}

		accountList.Items = append(accountList.Items, *account)
	}

	return accountList, nil
}

var _ = rest.Getter(&accountREST{})

func (r *accountREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	configAccount := &config.Account{}
	err := r.client.Get(ctx, types.NamespacedName{Name: name}, configAccount)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, kerrors.NewNotFound(tenancy.Resource("accounts"), name)
		}
		return nil, err
	}

	return ConvertConfigAccount(configAccount)
}

var _ = rest.Creater(&accountREST{})

func (r *accountREST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	account, ok := obj.(*tenancy.Account)
	if !ok {
		return nil, fmt.Errorf("not an account: %#v", obj)
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

var _ = rest.Updater(&accountREST{})
var _ = rest.CreaterUpdater(&accountREST{})

func (r *accountREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	oldObj, err := r.Get(ctx, name, nil)
	if err != nil {
		return nil, false, err
	}
	newObj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}
	newAccount, ok := newObj.(*tenancy.Account)
	if !ok {
		return nil, false, fmt.Errorf("New object is not an account")
	}

	newConfigAccount, err := ConvertTenancyAccount(newAccount)
	if err != nil {
		return nil, false, err
	}

	err = r.client.Update(ctx, newConfigAccount, &client.UpdateOptions{
		Raw: options,
	})
	if err != nil {
		return nil, false, err
	}

	return newAccount, true, nil
}

var _ = rest.GracefulDeleter(&accountREST{})

func (r *accountREST) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	configAccount := &config.Account{}
	err := r.client.Get(ctx, types.NamespacedName{Name: name}, configAccount)
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
