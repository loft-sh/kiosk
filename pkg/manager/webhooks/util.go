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
	"fmt"

	"github.com/kiosk-sh/kiosk/pkg/util/convert"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiadmission "k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// NewAttributeFromRequest converts the given request into api attributes
func NewAttributeFromRequest(req admission.Request, d *admission.Decoder, scheme *runtime.Scheme) (apiadmission.Attributes, error) {
	var (
		obj     runtime.Object
		oldObj  runtime.Object
		options runtime.Object
		err     error
		kind    = schema.GroupVersionKind{
			Group:   req.Kind.Group,
			Version: req.Kind.Version,
			Kind:    req.Kind.Kind,
		}
	)

	if req.Operation == v1beta1.Create {
		obj, err = newRuntimeObject(kind, scheme)
		if err != nil {
			return nil, err
		}

		err = d.DecodeRaw(req.Object, obj)
		if err != nil {
			return nil, err
		}
	} else if req.Operation == v1beta1.Update {
		obj, err = newRuntimeObject(kind, scheme)
		if err != nil {
			return nil, err
		}
		err = d.DecodeRaw(req.Object, obj)
		if err != nil {
			return nil, err
		}
		oldObj, err = newRuntimeObject(kind, scheme)
		if err != nil {
			return nil, err
		}
		err = d.DecodeRaw(req.OldObject, oldObj)
		if err != nil {
			return nil, err
		}
	} else if req.Operation == v1beta1.Delete {
		if len(req.OldObject.Raw) > 0 {
			oldObj, err = newRuntimeObject(kind, scheme)
			if err != nil {
				return nil, err
			}
			err = d.DecodeRaw(req.OldObject, oldObj)
			if err != nil {
				return nil, err
			}
		}
	} else {
		return nil, fmt.Errorf("Operation %s not supported", string(req.Operation))
	}

	// We don't really care if the options fail to convert, then we will just pass nil
	options, _ = convert.StringToUnstructured(string(req.Options.Raw))

	resource := schema.GroupVersionResource{
		Group:    req.Resource.Group,
		Version:  req.Resource.Version,
		Resource: req.Resource.Resource,
	}

	dryRun := false
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}

	extras := map[string][]string{}
	for k, v := range req.UserInfo.Extra {
		extras[k] = v
	}

	userInfo := &user.DefaultInfo{
		Name:   req.UserInfo.Username,
		UID:    req.UserInfo.UID,
		Groups: req.UserInfo.Groups,
		Extra:  extras,
	}

	return apiadmission.NewAttributesRecord(obj, oldObj, kind, req.Namespace, req.Name, resource, req.SubResource, apiadmission.Operation(req.Operation), options, dryRun, userInfo), nil
}

func newRuntimeObject(kind schema.GroupVersionKind, scheme *runtime.Scheme) (runtime.Object, error) {
	obj, err := scheme.New(kind)
	if err != nil {
		if runtime.IsNotRegisteredError(err) {
			obj = &unstructured.Unstructured{}
			obj.(*unstructured.Unstructured).SetGroupVersionKind(kind)
			return obj, nil
		}

		return nil, err
	}

	return obj, nil
}
