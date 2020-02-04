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

// +kubebuilder:webhook:verbs=update,path=/validate-templateinstance,mutating=false,failurePolicy=fail,groups=config.kiosk.sh,resources=templateinstances,versions=v1alpha1,name=vtemplateinstance.kb.io

// TemplateInstanceValidator validates a template instance
type TemplateInstanceValidator struct {
	Log    logr.Logger
	Scheme *runtime.Scheme

	decoder *admission.Decoder
}

// Handle handles the validation
func (v *TemplateInstanceValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var (
		newObj *configv1alpha1.TemplateInstance
		oldObj *configv1alpha1.TemplateInstance
	)

	// Is not update?
	if req.Operation != v1beta1.Update {
		return admission.Allowed("")
	}

	newObj = &configv1alpha1.TemplateInstance{}
	err := v.decoder.DecodeRaw(req.Object, newObj)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	oldObj = &configv1alpha1.TemplateInstance{}
	err = v.decoder.DecodeRaw(req.OldObject, oldObj)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if newObj.Spec.Template != oldObj.Spec.Template {
		return admission.Denied("Field spec.template is immutable")
	}

	return admission.Allowed("")
}

// InjectDecoder injects the decoder.
func (v *TemplateInstanceValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
