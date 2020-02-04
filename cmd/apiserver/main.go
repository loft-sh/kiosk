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

package main

import (
	"os"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/auth"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// +kubebuilder:scaffold:imports

	// Make sure dep tools picks up these dependencies
	_ "github.com/go-openapi/loads"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kiosk-sh/kiosk/pkg/apiserver"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Enable cloud provider auth

	"github.com/kiosk-sh/kiosk/pkg/apis"
	"github.com/kiosk-sh/kiosk/pkg/openapi"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = configv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = os.Getenv("DEBUG") != ""
	}))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create the auth cache here because we have to register some informers and handlers
	authCache, err := auth.NewAuthCache(mgr.GetClient(), mgr.GetCache(), ctrl.Log.WithName("authCache"))
	if err != nil {
		setupLog.Error(err, "unable to initialize auth cache")
		os.Exit(1)
	}

	stopChan := ctrl.SetupSignalHandler()

	// Start auth cache
	go authCache.Run(stopChan)

	// Start manager
	go func() {
		setupLog.Info("starting manager")
		if err := mgr.Start(stopChan); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}()

	// Wait for cache sync
	mgr.GetCache().WaitForCacheSync(stopChan)

	version := "v0"
	err = apiserver.StartApiServerWithOptions(&apiserver.StartOptions{
		Apis:        apis.GetAllApiBuilders(mgr, authCache),
		Openapidefs: openapi.GetOpenAPIDefinitions,
		Title:       "Api",
		Version:     version,
	})
	if err != nil {
		panic(err)
	}
}
