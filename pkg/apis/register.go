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

package apis

import (
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	_ "github.com/kiosk-sh/kiosk/pkg/apis/tenancy/install" // Install the tenancy group
	tenancyv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/tenancy/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/auth"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/registry/account"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/registry/space"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"sigs.k8s.io/apiserver-builder-alpha/pkg/builders"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	localSchemeBuilder = runtime.SchemeBuilder{
		tenancyv1alpha1.AddToScheme,
	}
	AddToScheme = localSchemeBuilder.AddToScheme
)

// GetAllApiBuilders returns all known APIGroupBuilders
// so they can be registered with the apiserver
func GetAllApiBuilders(mgr manager.Manager, authCache auth.Cache) []*builders.APIGroupBuilder {
	// Set rest funcs
	tenancy.TenancyAccountStorageProvider = func(getter generic.RESTOptionsGetter) rest.Storage {
		return account.NewAccountStorage(mgr.GetClient(), authCache)
	}

	tenancy.TenancySpaceStorageProvider = func(getter generic.RESTOptionsGetter) rest.Storage {
		return space.NewSpaceStorage(mgr.GetClient(), authCache, mgr.GetScheme())
	}

	return []*builders.APIGroupBuilder{
		GetTenancyAPIBuilder(),
	}
}

var tenancyApiGroup = builders.NewApiGroupBuilder(
	"tenancy.kiosk.sh",
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy").
	WithUnVersionedApi(tenancy.ApiVersion).
	WithVersionedApis(tenancyv1alpha1.ApiVersion).
	WithRootScopedKinds()

func GetTenancyAPIBuilder() *builders.APIGroupBuilder {
	return tenancyApiGroup
}
