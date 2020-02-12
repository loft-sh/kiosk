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
	"time"

	"github.com/kiosk-sh/kiosk/pkg/manager/controllers"
	quotacontroller "github.com/kiosk-sh/kiosk/pkg/manager/quota/controller"

	kubectrlmgrconfigv1alpha1 "k8s.io/kube-controller-manager/config/v1alpha1"
	"k8s.io/kubernetes/pkg/controller"
	kubectrldefaults "k8s.io/kubernetes/pkg/controller/resourcequota/config/v1alpha1"
	"k8s.io/kubernetes/pkg/quota/v1/generic"
)

// Register registers the quota controller
func Register(ctrlCtx *controllers.Context) error {
	controllerConfig := &kubectrlmgrconfigv1alpha1.ResourceQuotaControllerConfiguration{}
	kubectrldefaults.RecommendedDefaultResourceQuotaControllerConfiguration(controllerConfig)

	listerFuncForResource := generic.ListerFuncForResourceFunc(ctrlCtx.SharedInformers.ForResource)
	quotaConfiguration := NewQuotaConfiguration(listerFuncForResource)

	ctrlOptions := &quotacontroller.AccountQuotaControllerOptions{
		Manager:                   ctrlCtx.Manager,
		ResyncPeriod:              controller.StaticResyncPeriodFunc(controllerConfig.ResourceQuotaSyncPeriod.Duration),
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
