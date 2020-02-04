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
	"net/http"

	"github.com/go-logr/logr"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"

	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:verbs=update,path=/validate-accountquota,mutating=false,failurePolicy=fail,groups=config.kiosk.sh,resources=accountquotas,versions=v1alpha1,name=vaccountquota.kb.io

// AccountQuotaValidator validates account quotas
type AccountQuotaValidator struct {
	Log    logr.Logger
	Scheme *runtime.Scheme

	decoder *admission.Decoder
}

// Handle handles the validation
func (v *AccountQuotaValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var (
		newQuota *configv1alpha1.AccountQuota
		oldQuota *configv1alpha1.AccountQuota
	)

	// Is not update?
	if req.Operation != v1beta1.Update {
		return admission.Allowed("")
	}

	newQuota = &configv1alpha1.AccountQuota{}
	err := v.decoder.DecodeRaw(req.Object, newQuota)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	oldQuota = &configv1alpha1.AccountQuota{}
	err = v.decoder.DecodeRaw(req.OldObject, oldQuota)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if newQuota.Spec.Account != oldQuota.Spec.Account {
		return admission.Denied("Field spec.account is immutable")
	}

	return admission.Allowed("")
}

// InjectDecoder injects the decoder.
func (v *AccountQuotaValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
