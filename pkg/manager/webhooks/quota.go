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
	"context"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime"
	apiadmission "k8s.io/apiserver/pkg/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-quota,mutating=false,failurePolicy=fail,groups="*",resources="*",versions="*",name=accountquota.kiosk.sh

// QuotaValidator validates pods
type QuotaValidator struct {
	Log    logr.Logger
	Scheme *runtime.Scheme

	decoder *admission.Decoder

	AdmissionController apiadmission.ValidationInterface
}

// Handle handles the admission request
func (v *QuotaValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if v.AdmissionController == nil {
		return admission.Denied("Admission controller is not ready")
	} // We allow admissions we don't handle
	if v.AdmissionController.Handles(apiadmission.Operation(req.Operation)) == false {
		return admission.Allowed("")
	}

	// Convert request
	attributes, err := NewAttributeFromRequest(req, v.decoder, v.Scheme)
	if err != nil {
		return admission.Errored(1, err)
	}

	// Check if the admission controller allows it
	err = v.AdmissionController.Validate(ctx, attributes, nil)
	if err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

// InjectDecoder injects the decoder.
func (v *QuotaValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
