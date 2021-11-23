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
	config "github.com/loft-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/loft-sh/kiosk/pkg/apis/tenancy"
	"github.com/loft-sh/kiosk/pkg/authorization"
	"github.com/loft-sh/kiosk/pkg/authorization/rbac"
	kioskwatch "github.com/loft-sh/kiosk/pkg/watch"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type accountREST struct {
	client     client.Client
	filter     authorization.FilteredLister
	authorizer authorizer.Authorizer
}

// NewAccountREST creates a new account storage that implements the rest interface
func NewAccountREST(cachedClient client.Client, uncachedClient client.Client, scheme *runtime.Scheme) rest.Storage {
	authorizer := rbac.New(cachedClient)
	return &accountREST{
		client:     uncachedClient,
		authorizer: authorizer,
		filter:     authorization.NewFilteredLister(uncachedClient, authorizer),
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

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

func (r *accountREST) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	var table metav1.Table
	fn := func(obj runtime.Object) error {
		account, ok := obj.(*tenancy.Account)
		if !ok {
			return fmt.Errorf("cannot convert to account: %#+v", obj)
		}

		table.Rows = append(table.Rows, metav1.TableRow{
			Cells:  []interface{}{account.GetName(), len(account.Status.Namespaces), account.GetCreationTimestamp().Time.UTC().Format(time.RFC3339)},
			Object: runtime.RawExtension{Object: obj},
		})
		return nil
	}
	switch {
	case meta.IsListType(object):
		if err := meta.EachListItem(object, fn); err != nil {
			return nil, err
		}
	default:
		if err := fn(object); err != nil {
			return nil, err
		}
	}
	if m, err := meta.ListAccessor(object); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.SelfLink = m.GetSelfLink()
		table.Continue = m.GetContinue()
		table.RemainingItemCount = m.GetRemainingItemCount()
	} else {
		if m, err := meta.CommonAccessor(object); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
			table.SelfLink = m.GetSelfLink()
		}
	}
	if opt, ok := tableOptions.(*metav1.TableOptions); !ok || !opt.NoHeaders {
		table.ColumnDefinitions = []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "Spaces", Type: "integer", Description: "The number of spaces this account owns"},
			{Name: "Created At", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
		}
	}
	return &table, nil
}

func (r *accountREST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	configAccountList := &config.AccountList{}
	_, err := r.filter.List(ctx, configAccountList, tenancy.SchemeGroupVersion.WithResource("accounts"), options)
	if err != nil {
		return nil, err
	}

	accountList := &tenancy.AccountList{
		ListMeta: configAccountList.ListMeta,
		Items:    []tenancy.Account{},
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

var _ = rest.Watcher(&accountREST{})

func (r *accountREST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("user is missing in context")
	}
	if options == nil {
		options = &metainternalversion.ListOptions{}
	}

	w := &watcher{
		userInfo:      userInfo,
		labelSelector: options.LabelSelector,
		authorizer:    r.authorizer,
		result:        make(chan watch.Event),
	}
	kioskwatch.AccountRegistry.Subscribe(w)
	return w, nil
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
	if createValidation != nil {
		err := createValidation(ctx, account)
		if err != nil {
			return nil, err
		}
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

	if updateValidation != nil {
		err := updateValidation(ctx, newAccount, oldObj)
		if err != nil {
			return nil, false, err
		}
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

	account, err := ConvertConfigAccount(configAccount)
	if err != nil {
		return nil, false, err
	}
	if deleteValidation != nil {
		err = deleteValidation(ctx, account)
		if err != nil {
			return nil, false, err
		}
	}

	// we have to use a background context here, because it might
	// be possible that the user is cancelling the request and we want
	// to fully delete the account and its children
	err = r.client.Delete(context.Background(), configAccount, &client.DeleteOptions{
		Raw: options,
	})
	if err != nil {
		return nil, false, err
	}

	account, err = ConvertConfigAccount(configAccount)
	if err != nil {
		return nil, false, err
	}

	return account, true, err
}
