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
	"fmt"

	config "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
)

// DecoratorFunc can mutate the provided object prior to being returned.
type DecoratorFunc func(obj runtime.Object) error

// FilterList filters any list object that conforms to the api conventions,
// provided that 'm' works with the concrete type of list. d is an optional
// decorator for the returned functions. Only matching items are decorated.
func FilterList(items []runtime.Object, m storage.SelectionPredicate, d DecoratorFunc) ([]runtime.Object, error) {
	var filteredItems []runtime.Object
	for _, obj := range items {
		match, err := m.Matches(obj)
		if err != nil {
			return nil, err
		}
		if match {
			if d != nil {
				if err := d(obj); err != nil {
					return nil, err
				}
			}
			filteredItems = append(filteredItems, obj)
		}
	}

	return filteredItems, nil
}

// ListOptionsToSelectors converts the given list options to selectors
func ListOptionsToSelectors(options *metainternalversion.ListOptions) (labels.Selector, fields.Selector) {
	label := labels.Everything()
	if options != nil && options.LabelSelector != nil {
		label = options.LabelSelector
	}
	field := fields.Everything()
	if options != nil && options.FieldSelector != nil {
		field = options.FieldSelector
	}
	return label, field
}

// GetNamespaceAttrs returns labels and fields of a given object for filtering purposes.
func GetNamespaceAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	namespaceObj, ok := obj.(*corev1.Namespace)
	if !ok {
		return nil, nil, fmt.Errorf("not a namespace")
	}

	return labels.Set(namespaceObj.Labels), NamespaceToSelectableFields(namespaceObj), nil
}

// GetAccountAttrs returns labels and fields of a given object for filtering purposes.
func GetAccountAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	accountObj, ok := obj.(*config.Account)
	if !ok {
		return nil, nil, fmt.Errorf("not a account")
	}

	return labels.Set(accountObj.Labels), AccountToSelectableFields(accountObj), nil
}

// MatchAccount returns a generic matcher for a given label and field selector.
func MatchAccount(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAccountAttrs,
	}
}

// MatchNamespace returns a generic matcher for a given label and field selector.
func MatchNamespace(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetNamespaceAttrs,
	}
}

// AccountToSelectableFields returns a field set that represents the object
func AccountToSelectableFields(account *config.Account) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&account.ObjectMeta, false)
	specificFieldsSet := fields.Set{
		// This is a bug, but we need to support it for backward compatibility.
		"name": account.Name,
	}
	return generic.MergeFieldsSets(objectMetaFieldsSet, specificFieldsSet)
}

// NamespaceToSelectableFields returns a field set that represents the object
func NamespaceToSelectableFields(namespace *corev1.Namespace) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&namespace.ObjectMeta, false)
	specificFieldsSet := fields.Set{
		"status.phase": string(namespace.Status.Phase),
		// This is a bug, but we need to support it for backward compatibility.
		"name": namespace.Name,
	}
	return generic.MergeFieldsSets(objectMetaFieldsSet, specificFieldsSet)
}
