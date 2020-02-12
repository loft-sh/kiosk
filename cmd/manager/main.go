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
	"flag"
	"os"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/manager/controllers"
	"github.com/kiosk-sh/kiosk/pkg/manager/quota"
	"github.com/kiosk-sh/kiosk/pkg/manager/webhooks"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// +kubebuilder:scaffold:imports

	// Make sure dep tools picks up these dependencies
	_ "github.com/go-openapi/loads"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Enable cloud provider auth
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
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = os.Getenv("DEBUG") != ""
	}))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Add required indices
	err = controllers.AddManagerIndices(mgr.GetCache())
	if err != nil {
		setupLog.Error(err, "unable to set manager indices")
		os.Exit(1)
	}

	stopChan := ctrl.SetupSignalHandler()
	ctrlCtx := controllers.NewControllerContext(mgr, stopChan)

	// Add controllers to manager
	err = controllers.Register(mgr)
	if err != nil {
		setupLog.Error(err, "unable to register controller")
		os.Exit(1)
	}

	// Create the admission controller
	err = webhooks.Register(ctrlCtx)
	if err != nil {
		setupLog.Error(err, "unable to register webhooks")
		os.Exit(1)
	}

	// Start quota controller
	go func() {
		err := quota.Register(ctrlCtx)
		if err != nil {
			setupLog.Error(err, "unable to register quota controller")
		}
	}()

	// Start controller context
	go ctrlCtx.Start()

	setupLog.Info("starting manager")
	if err := mgr.Start(stopChan); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
