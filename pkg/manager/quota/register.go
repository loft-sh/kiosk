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

	quotacontroller "github.com/kiosk-sh/kiosk/pkg/manager/quota/controller"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Register registers the quota controller
func Register(mgr manager.Manager) error {
	quotaConfiguration := NewQuotaConfiguration(mgr)
	ctrlOptions := &quotacontroller.AccountQuotaControllerOptions{
		Manager:                   mgr,
		ResyncPeriod:              func() time.Duration { return time.Hour * 10 },
		Registry:                  NewQuotaRegistry(quotaConfiguration),
		IgnoredResourcesFunc:      func() map[schema.GroupResource]struct{} { return quotaConfiguration.IgnoredResources() },
		ReplenishmentResyncPeriod: func() time.Duration { return time.Hour * 10 },
	}

	controller, err := quotacontroller.NewAccountQuotaController(ctrlOptions)
	if err != nil {
		return err
	}

	// Start the controller
	go func() {
		mgr.GetCache().WaitForCacheSync(nil)
		controller.Run(10, make(chan struct{}))
	}()

	return nil
}
