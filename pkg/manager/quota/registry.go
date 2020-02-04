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
	"github.com/kiosk-sh/kiosk/pkg/util"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/kubernetes/pkg/quota/v1"
	"k8s.io/kubernetes/pkg/quota/v1/evaluator/core"
	"k8s.io/kubernetes/pkg/quota/v1/generic"
	"k8s.io/kubernetes/pkg/quota/v1/install"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewQuotaConfiguration creates a new quota configuration that can be used to create quota registry
func NewQuotaConfiguration(mgr manager.Manager) quota.Configuration {
	f := util.ListerFuncForResourceFunc(mgr)

	// we only get the pod evaluator for now
	// evaluators := core.NewEvaluators(f)
	evaluators := []quota.Evaluator{core.NewPodEvaluator(f, clock.RealClock{})}

	return generic.NewConfiguration(evaluators, install.DefaultIgnoredResources())
}

// NewQuotaRegistry creates a new registry from the given quota config
func NewQuotaRegistry(config quota.Configuration) quota.Registry {
	return generic.NewRegistry(config.Evaluators())
}
