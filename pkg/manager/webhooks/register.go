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

package webhooks

import (
	"github.com/kiosk-sh/kiosk/pkg/manager/controllers"
	"github.com/kiosk-sh/kiosk/pkg/manager/quota"
	"github.com/kiosk-sh/kiosk/pkg/util/encoding"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// Register registers the webhooks to the manager
func Register(ctrlCtx *controllers.Context) error {
	hookServer := ctrlCtx.Manager.GetWebhookServer()

	// Create the admission controller
	admissionController := quota.NewAccountResourceQuota(ctrlCtx)
	hookServer.Register("/quota", &webhook.Admission{Handler: &QuotaValidator{
		Log:                 ctrl.Log.WithName("webhooks").WithName("Quota"),
		Scheme:              ctrlCtx.Manager.GetScheme(),
		AdmissionController: admissionController,
	}})

	hookServer.Register("/validate", &webhook.Admission{Handler: &Validator{
		Log:           ctrl.Log.WithName("webhooks").WithName("Validator"),
		StrictDecoder: encoding.NewDecoder(ctrlCtx.Manager.GetScheme(), true),
		NormalDecoder: encoding.NewDecoder(ctrlCtx.Manager.GetScheme(), false),
	}})

	return nil
}
