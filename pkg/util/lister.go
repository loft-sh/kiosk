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
	"context"
	"fmt"
	"sync/atomic"

	"github.com/kiosk-sh/kiosk/pkg/util/convert"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	quota "k8s.io/kubernetes/pkg/quota/v1"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// InformerForResource retrieves an informer for the given group version resource
func InformerForResource(mgr manager.Manager, gvr schema.GroupVersionResource) (ctrlcache.Informer, error) {
	gvk, err := mgr.GetRESTMapper().KindFor(gvr)
	if err != nil {
		return nil, err
	}

	return mgr.GetCache().GetInformerForKind(gvk)
}

// ListerFuncForResourceFunc knows how to provision a lister from an informer func.
// The lister returns errors until the informer has synced.
func ListerFuncForResourceFunc(mgr manager.Manager) quota.ListerForResourceFunc {
	return func(gvr schema.GroupVersionResource) (cache.GenericLister, error) {
		gvk, err := mgr.GetRESTMapper().KindFor(gvr)
		if err != nil {
			return nil, err
		}
		// Just make sure an informer exists for this
		informer, err := mgr.GetCache().GetInformerForKind(gvk)
		if err != nil {
			return nil, err
		}
		return &genericLister{
			hasSynced: cachedHasSynced(informer.HasSynced),
			gvk:       gvk,
			scheme:    mgr.GetScheme(),
			client:    mgr.GetClient(),
		}, nil
	}
}

// cachedHasSynced returns a function that calls hasSynced() until it returns true once, then returns true
func cachedHasSynced(hasSynced func() bool) func() bool {
	cache := &atomic.Value{}
	cache.Store(false)
	return func() bool {
		if cache.Load().(bool) {
			// short-circuit if already synced
			return true
		}
		if hasSynced() {
			// remember we synced
			cache.Store(true)
			return true
		}
		return false
	}
}

type genericLister struct {
	hasSynced func() bool
	gvk       schema.GroupVersionKind
	scheme    *runtime.Scheme
	client    client.Client
	namespace string
}

func (p *genericLister) List(selector labels.Selector) ([]runtime.Object, error) {
	if !p.hasSynced() {
		return nil, fmt.Errorf("%v not yet synced", p.gvk)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(p.gvk)

	var err error
	if p.namespace != "" {
		err = p.client.List(context.Background(), list, client.InNamespace(p.namespace))
	} else {
		err = p.client.List(context.Background(), list)
	}
	if err != nil {
		return nil, err
	}

	ret := make([]runtime.Object, len(list.Items))
	for i, item := range list.Items {
		t, err := convert.ConvertFromUnstructured(&item, p.gvk, p.scheme)
		if err != nil {
			return nil, err
		}
		ret[i] = t
	}

	return ret, nil
}

func (p *genericLister) Get(name string) (runtime.Object, error) {
	if !p.hasSynced() {
		return nil, fmt.Errorf("%v not yet synced", p.gvk)
	}

	pod := &unstructured.Unstructured{}
	pod.SetGroupVersionKind(p.gvk)
	err := p.client.Get(context.Background(), types.NamespacedName{Namespace: p.namespace, Name: name}, pod)
	if err != nil {
		return nil, err
	}
	convertedPod, err := convert.ConvertFromUnstructured(pod, p.gvk, p.scheme)
	if err != nil {
		return nil, err
	}
	return convertedPod, nil
}

func (p *genericLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	return &genericLister{
		hasSynced: p.hasSynced,
		client:    p.client,
		gvk:       p.gvk,
		scheme:    p.scheme,
		namespace: namespace,
	}
}
