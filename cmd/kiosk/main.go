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
	"context"
	"github.com/kiosk-sh/kiosk/pkg/apis"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	tenancyv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/tenancy/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apiserver"
	_ "github.com/kiosk-sh/kiosk/pkg/apiserver/registry"
	"github.com/kiosk-sh/kiosk/pkg/openapi"
	"github.com/kiosk-sh/kiosk/pkg/store/apiservice"
	"github.com/kiosk-sh/kiosk/pkg/store/crd"
	"github.com/kiosk-sh/kiosk/pkg/store/validatingwebhookconfiguration"
	"github.com/kiosk-sh/kiosk/pkg/util/certhelper"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"os"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"

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
	// API extensions are not in the above scheme set,
	// and must thus be added separately.
	_ = apiextensionsv1beta1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)
	_ = apiregistrationv1.AddToScheme(scheme)

	_ = tenancy.AddToScheme(scheme)
	_ = tenancyv1alpha1.AddToScheme(scheme)
	_ = configv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = os.Getenv("DEBUG") != ""
	}))

	// Make sure the certificates are there
	err := certhelper.WriteCertificates()
	if err != nil {
		setupLog.Error(err, "unable to generate certificates")
		os.Exit(1)
	}

	// retrieve in cluster config
	config := ctrl.GetConfigOrDie()

	// Make sure the needed crds are installed in the cluster
	err = initialize(config)
	if err != nil {
		klog.Fatal(err)
	}

	// create the manager
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: ":8080",
		LeaderElection:     false,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Inject the client and scheme
	injectClient(mgr.GetClient(), scheme)

	// Add required indices
	err = controllers.AddManagerIndices(mgr.GetCache())
	if err != nil {
		setupLog.Error(err, "unable to set manager indices")
		os.Exit(1)
	}

	stopChan := ctrl.SetupSignalHandler()
	ctrlCtx := controllers.NewControllerContext(mgr, stopChan)

	// Register generic controllers
	err = controllers.Register(mgr)
	if err != nil {
		setupLog.Error(err, "unable to register controller")
		os.Exit(1)
	}

	// Register quota controller
	err = quota.Register(ctrlCtx)
	if err != nil {
		setupLog.Error(err, "unable to register quota controller")
		os.Exit(1)
	}

	// Register webhooks
	err = webhooks.Register(ctrlCtx)
	if err != nil {
		setupLog.Error(err, "unable to register webhooks")
		os.Exit(1)
	}

	// Start controller context
	go ctrlCtx.Start()

	// Start the local manager
	go func() {
		setupLog.Info("starting manager")
		err = mgr.Start(stopChan)
		if err != nil {
			panic(err)
		}
	}()

	// Make sure the manager is synced
	mgr.GetCache().WaitForCacheSync(stopChan)

	// Start the api server
	go func() {
		version := "v0"
		err = apiserver.StartApiServerWithOptions(&apiserver.StartOptions{
			Apis:        apis.GetAllApiBuilders(),
			Openapidefs: openapi.GetOpenAPIDefinitions,
			Title:       "Api",
			Version:     version,
		})
		if err != nil {
			panic(err)
		}
	}()

	// setup validatingwebhookconfiguration
	err = validatingwebhookconfiguration.EnsureValidatingWebhookConfiguration(context.Background(), mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "unable to set up validating webhook configuration")
		os.Exit(1)
	}

	// setup apiservice
	err = apiservice.EnsureAPIService(context.Background(), mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "unable to set up apiservice")
		os.Exit(1)
	}

	// Wait till stopChan is closed
	<-stopChan
}

func initialize(config *rest.Config) error {
	klog.Info("Initialize...")
	defer klog.Info("Done initializing...")

	client, err := client2.New(config, client2.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	klog.Info("Installing crds...")

	builder := crd.NewBuilder(client)
	_, err = builder.CreateCRDs(context.Background(), apis.TypeDefinitions...)
	return err
}

func injectClient(client client2.Client, scheme *runtime.Scheme) {
	tenancy.Client = client
	tenancy.Scheme = scheme
}
