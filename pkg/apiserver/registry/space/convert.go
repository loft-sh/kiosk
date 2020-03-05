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

package space

import (
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"

	corev1 "k8s.io/api/core/v1"
)

// ConvertSpace converts a space into a namespace
func ConvertSpace(space *tenancy.Space) *corev1.Namespace {
	namespace := &corev1.Namespace{
		ObjectMeta: space.ObjectMeta,
		Spec: corev1.NamespaceSpec{
			Finalizers: space.Spec.Finalizers,
		},
		Status: corev1.NamespaceStatus{
			Phase: space.Status.Phase,
		},
	}

	if namespace.Labels == nil {
		namespace.Labels = map[string]string{}
	}

	namespace.Labels[tenancy.SpaceLabelAccount] = space.Spec.Account
	return namespace
}

// ConvertNamespace converts a namespace into a space
func ConvertNamespace(namespace *corev1.Namespace) *tenancy.Space {
	space := &tenancy.Space{
		ObjectMeta: namespace.ObjectMeta,
		Spec: tenancy.SpaceSpec{
			Finalizers: namespace.Spec.Finalizers,
		},
		Status: tenancy.SpaceStatus{
			Phase: namespace.Status.Phase,
		},
	}

	if namespace.Labels == nil {
		namespace.Labels = map[string]string{}
	}

	space.Spec.Account = namespace.Labels[tenancy.SpaceLabelAccount]
	delete(space.Labels, tenancy.SpaceLabelAccount)
	return space
}
