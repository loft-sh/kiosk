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

package quota

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"github.com/kiosk-sh/kiosk/pkg/manager/controllers"
	quotacontroller "github.com/kiosk-sh/kiosk/pkg/manager/quota/controller"

	"github.com/kiosk-sh/kiosk/kube/pkg/quota/v1/generic"
	kubectrlmgrconfigv1alpha1 "k8s.io/kube-controller-manager/config/v1alpha1"
)

// RecommendedDefaultResourceQuotaControllerConfiguration defaults a pointer to a
// ResourceQuotaControllerConfiguration struct. This will set the recommended default
// values, but they may be subject to change between API versions. This function
// is intentionally not registered in the scheme as a "normal" `SetDefaults_Foo`
// function to allow consumers of this type to set whatever defaults for their
// embedded configs. Forcing consumers to use these defaults would be problematic
// as defaulting in the scheme is done as part of the conversion, and there would
// be no easy way to opt-out. Instead, if you want to use this defaulting method
// run it in your wrapper struct of this type in its `SetDefaults_` method.
func RecommendedDefaultResourceQuotaControllerConfiguration(obj *kubectrlmgrconfigv1alpha1.ResourceQuotaControllerConfiguration) {
	zero := metav1.Duration{}
	if obj.ConcurrentResourceQuotaSyncs == 0 {
		obj.ConcurrentResourceQuotaSyncs = 5
	}
	if obj.ResourceQuotaSyncPeriod == zero {
		obj.ResourceQuotaSyncPeriod = metav1.Duration{Duration: 5 * time.Minute}
	}
}

// Register registers the quota controller
func Register(ctrlCtx *controllers.Context) error {
	controllerConfig := &kubectrlmgrconfigv1alpha1.ResourceQuotaControllerConfiguration{}
	RecommendedDefaultResourceQuotaControllerConfiguration(controllerConfig)

	listerFuncForResource := generic.ListerFuncForResourceFunc(ctrlCtx.SharedInformers.ForResource)
	quotaConfiguration := NewQuotaConfiguration(listerFuncForResource)

	ctrlOptions := &quotacontroller.AccountQuotaControllerOptions{
		Manager:                   ctrlCtx.Manager,
		ResyncPeriod:              quotacontroller.StaticResyncPeriodFunc(controllerConfig.ResourceQuotaSyncPeriod.Duration),
		InformerFactory:           ctrlCtx.ObjectOrMetadataInformers,
		ReplenishmentResyncPeriod: controllers.ResyncPeriod(),
		DiscoveryFunc:             ctrlCtx.DiscoveryFunc,
		IgnoredResourcesFunc:      quotaConfiguration.IgnoredResources,
		InformersStarted:          ctrlCtx.InformersStarted,
		Registry:                  generic.NewRegistry(quotaConfiguration.Evaluators()),
	}

	// Create the controller from options
	controller, err := quotacontroller.NewAccountQuotaController(ctrlOptions)
	if err != nil {
		return err
	}

	// Start the controller
	go controller.Run(int(controllerConfig.ConcurrentResourceQuotaSyncs), ctrlCtx.StopChan)

	// Periodically the quota controller to detect new resource types
	go controller.Sync(ctrlCtx.DiscoveryFunc, 30*time.Second, ctrlCtx.StopChan)

	return nil
}
