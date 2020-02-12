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
	"k8s.io/kubernetes/pkg/quota/v1"
	"k8s.io/kubernetes/pkg/quota/v1/evaluator/core"
	"k8s.io/kubernetes/pkg/quota/v1/generic"
	"k8s.io/kubernetes/pkg/quota/v1/install"
)

// NewQuotaConfiguration creates a new quota configuration that can be used to create quota registry
func NewQuotaConfiguration(f quota.ListerForResourceFunc) quota.Configuration {
	return generic.NewConfiguration(core.NewEvaluators(f), install.DefaultIgnoredResources())
}

// NewQuotaRegistry creates a new registry from the given quota config
func NewQuotaRegistry(config quota.Configuration) quota.Registry {
	return generic.NewRegistry(config.Evaluators())
}
