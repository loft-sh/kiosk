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
	"fmt"
	"github.com/kiosk-sh/kiosk/pkg/apis/config/validation"

	"github.com/go-logr/logr"
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/util/encoding"

	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Validator struct {
	Log           logr.Logger
	StrictDecoder encoding.Decoder
	NormalDecoder encoding.Decoder
}

func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var (
		obj    runtime.Object
		oldObj runtime.Object
		err    error
		kind   = schema.GroupVersionKind{
			Group:   req.Kind.Group,
			Version: req.Kind.Version,
			Kind:    req.Kind.Kind,
		}
	)

	// We allow other api groups
	if kind.GroupVersion().String() != configv1alpha1.SchemeGroupVersion.String() {
		return admission.Allowed("")
	}

	if req.Operation == v1beta1.Create {
		obj, err = v.StrictDecoder.Decode(req.Object.Raw)
		if err != nil {
			return admission.Denied(err.Error())
		}
	} else if req.Operation == v1beta1.Update {
		obj, err = v.StrictDecoder.Decode(req.Object.Raw)
		if err != nil {
			return admission.Denied(err.Error())
		}

		oldObj, err = v.NormalDecoder.Decode(req.OldObject.Raw)
		if err != nil {
			return admission.Errored(1, err)
		}
	} else {
		return admission.Errored(1, fmt.Errorf("operation %s not supported", string(req.Operation)))
	}

	var errs field.ErrorList
	switch kind.Kind {
	case "Account":
		if req.Operation == v1beta1.Create {
			errs = validation.ValidateAccount(obj.(*configv1alpha1.Account))
		} else {
			errs = validation.ValidateAccountUpdate(obj.(*configv1alpha1.Account), oldObj.(*configv1alpha1.Account))
		}
	case "AccountQuota":
		if req.Operation == v1beta1.Create {
			errs = validation.ValidateAccountQuota(obj.(*configv1alpha1.AccountQuota))
		} else {
			errs = validation.ValidateAccountQuotaUpdate(obj.(*configv1alpha1.AccountQuota), oldObj.(*configv1alpha1.AccountQuota))
		}
	case "TemplateInstance":
		if req.Operation == v1beta1.Create {
			errs = validation.ValidateTemplateInstance(obj.(*configv1alpha1.TemplateInstance))
		} else {
			errs = validation.ValidateTemplateInstanceUpdate(obj.(*configv1alpha1.TemplateInstance), oldObj.(*configv1alpha1.TemplateInstance))
		}
	}

	if len(errs) > 0 {
		return admission.Denied(errs.ToAggregate().Error())
	}

	return admission.Allowed("")
}
