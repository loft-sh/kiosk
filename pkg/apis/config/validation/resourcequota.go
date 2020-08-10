/*
Copyright 2014 The Kubernetes Authors.
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

package validation

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"strings"
)

// CODE IS TAKEN FROM https://github.com/kubernetes/kubernetes/blob/7740b8124c2f684de3caeae0f2cc5d2a1329d43e/pkg/apis/core/validation/validation.go

const isNegativeErrorMsg string = apimachineryvalidation.IsNegativeErrorMsg
const isInvalidQuotaResource string = `must be a standard resource for quota`
const isNotIntegerErrorMsg string = `must be an integer`

func ValidateResourceQuotaSpec(resourceQuotaSpec *corev1.ResourceQuotaSpec, fld *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	fldPath := fld.Child("hard")
	for k, v := range resourceQuotaSpec.Hard {
		resPath := fldPath.Key(string(k))
		allErrs = append(allErrs, ValidateResourceQuotaResourceName(string(k), resPath)...)
		allErrs = append(allErrs, ValidateResourceQuantityValue(string(k), v, resPath)...)
	}
	allErrs = append(allErrs, validateResourceQuotaScopes(resourceQuotaSpec, fld)...)
	allErrs = append(allErrs, validateScopeSelector(resourceQuotaSpec, fld)...)

	return allErrs
}

// validateResourceQuotaScopes ensures that each enumerated hard resource constraint is valid for set of scopes
func validateResourceQuotaScopes(resourceQuotaSpec *corev1.ResourceQuotaSpec, fld *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(resourceQuotaSpec.Scopes) == 0 {
		return allErrs
	}
	hardLimits := sets.NewString()
	for k := range resourceQuotaSpec.Hard {
		hardLimits.Insert(string(k))
	}
	fldPath := fld.Child("scopes")
	scopeSet := sets.NewString()
	for _, scope := range resourceQuotaSpec.Scopes {
		if !IsStandardResourceQuotaScope(string(scope)) {
			allErrs = append(allErrs, field.Invalid(fldPath, resourceQuotaSpec.Scopes, "unsupported scope"))
		}
		for _, k := range hardLimits.List() {
			if IsStandardQuotaResourceName(k) && !IsResourceQuotaScopeValidForResource(scope, k) {
				allErrs = append(allErrs, field.Invalid(fldPath, resourceQuotaSpec.Scopes, "unsupported scope applied to resource"))
			}
		}
		scopeSet.Insert(string(scope))
	}
	invalidScopePairs := []sets.String{
		sets.NewString(string(corev1.ResourceQuotaScopeBestEffort), string(corev1.ResourceQuotaScopeNotBestEffort)),
		sets.NewString(string(corev1.ResourceQuotaScopeTerminating), string(corev1.ResourceQuotaScopeNotTerminating)),
	}
	for _, invalidScopePair := range invalidScopePairs {
		if scopeSet.HasAll(invalidScopePair.List()...) {
			allErrs = append(allErrs, field.Invalid(fldPath, resourceQuotaSpec.Scopes, "conflicting scopes"))
		}
	}
	return allErrs
}

// validateScopeSelector tests that the specified scope selector has valid data
func validateScopeSelector(resourceQuotaSpec *corev1.ResourceQuotaSpec, fld *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if resourceQuotaSpec.ScopeSelector == nil {
		return allErrs
	}
	allErrs = append(allErrs, validateScopedResourceSelectorRequirement(resourceQuotaSpec, fld.Child("scopeSelector"))...)
	return allErrs
}

// validateScopedResourceSelectorRequirement tests that the match expressions has valid data
func validateScopedResourceSelectorRequirement(resourceQuotaSpec *corev1.ResourceQuotaSpec, fld *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	hardLimits := sets.NewString()
	for k := range resourceQuotaSpec.Hard {
		hardLimits.Insert(string(k))
	}
	fldPath := fld.Child("matchExpressions")
	scopeSet := sets.NewString()
	for _, req := range resourceQuotaSpec.ScopeSelector.MatchExpressions {
		if !IsStandardResourceQuotaScope(string(req.ScopeName)) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scopeName"), req.ScopeName, "unsupported scope"))
		}
		for _, k := range hardLimits.List() {
			if IsStandardQuotaResourceName(k) && !IsResourceQuotaScopeValidForResource(req.ScopeName, k) {
				allErrs = append(allErrs, field.Invalid(fldPath, resourceQuotaSpec.ScopeSelector, "unsupported scope applied to resource"))
			}
		}
		switch req.ScopeName {
		case corev1.ResourceQuotaScopeBestEffort, corev1.ResourceQuotaScopeNotBestEffort, corev1.ResourceQuotaScopeTerminating, corev1.ResourceQuotaScopeNotTerminating:
			if req.Operator != corev1.ScopeSelectorOpExists {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("operator"), req.Operator,
					"must be 'Exist' only operator when scope is any of ResourceQuotaScopeTerminating, ResourceQuotaScopeNotTerminating, ResourceQuotaScopeBestEffort and ResourceQuotaScopeNotBestEffort"))
			}
		}

		switch req.Operator {
		case corev1.ScopeSelectorOpIn, corev1.ScopeSelectorOpNotIn:
			if len(req.Values) == 0 {
				allErrs = append(allErrs, field.Required(fldPath.Child("values"),
					"must be at least one value when `operator` is 'In' or 'NotIn' for scope selector"))
			}
		case corev1.ScopeSelectorOpExists, corev1.ScopeSelectorOpDoesNotExist:
			if len(req.Values) != 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("values"), req.Values,
					"must be no value when `operator` is 'Exist' or 'DoesNotExist' for scope selector"))
			}
		default:
			allErrs = append(allErrs, field.Invalid(fldPath.Child("operator"), req.Operator, "not a valid selector operator"))
		}
		scopeSet.Insert(string(req.ScopeName))
	}
	invalidScopePairs := []sets.String{
		sets.NewString(string(corev1.ResourceQuotaScopeBestEffort), string(corev1.ResourceQuotaScopeNotBestEffort)),
		sets.NewString(string(corev1.ResourceQuotaScopeTerminating), string(corev1.ResourceQuotaScopeNotTerminating)),
	}
	for _, invalidScopePair := range invalidScopePairs {
		if scopeSet.HasAll(invalidScopePair.List()...) {
			allErrs = append(allErrs, field.Invalid(fldPath, resourceQuotaSpec.Scopes, "conflicting scopes"))
		}
	}

	return allErrs
}

// Validate resource names that can go in a resource quota
// Refer to docs/design/resources.md for more details.
func ValidateResourceQuotaResourceName(value string, fldPath *field.Path) field.ErrorList {
	allErrs := validateResourceName(value, fldPath)

	if len(strings.Split(value, "/")) == 1 {
		if !IsStandardQuotaResourceName(value) {
			return append(allErrs, field.Invalid(fldPath, value, isInvalidQuotaResource))
		}
	}
	return allErrs
}

// Validate compute resource typename.
// Refer to docs/design/resources.md for more details.
func validateResourceName(value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, msg := range validation.IsQualifiedName(value) {
		allErrs = append(allErrs, field.Invalid(fldPath, value, msg))
	}
	if len(allErrs) != 0 {
		return allErrs
	}

	if len(strings.Split(value, "/")) == 1 {
		if !IsStandardResourceName(value) {
			return append(allErrs, field.Invalid(fldPath, value, "must be a standard resource type or fully qualified"))
		}
	}

	return allErrs
}

var standardResources = sets.NewString(
	string(corev1.ResourceCPU),
	string(corev1.ResourceMemory),
	string(corev1.ResourceEphemeralStorage),
	string(corev1.ResourceRequestsCPU),
	string(corev1.ResourceRequestsMemory),
	string(corev1.ResourceRequestsEphemeralStorage),
	string(corev1.ResourceLimitsCPU),
	string(corev1.ResourceLimitsMemory),
	string(corev1.ResourceLimitsEphemeralStorage),
	string(corev1.ResourcePods),
	string(corev1.ResourceQuotas),
	string(corev1.ResourceServices),
	string(corev1.ResourceReplicationControllers),
	string(corev1.ResourceSecrets),
	string(corev1.ResourceConfigMaps),
	string(corev1.ResourcePersistentVolumeClaims),
	string(corev1.ResourceStorage),
	string(corev1.ResourceRequestsStorage),
	string(corev1.ResourceServicesNodePorts),
	string(corev1.ResourceServicesLoadBalancers),
)

// IsStandardResourceName returns true if the resource is known to the system
func IsStandardResourceName(str string) bool {
	return standardResources.Has(str) || IsQuotaHugePageResourceName(corev1.ResourceName(str))
}

var standardQuotaResources = sets.NewString(
	string(corev1.ResourceCPU),
	string(corev1.ResourceMemory),
	string(corev1.ResourceEphemeralStorage),
	string(corev1.ResourceRequestsCPU),
	string(corev1.ResourceRequestsMemory),
	string(corev1.ResourceRequestsStorage),
	string(corev1.ResourceRequestsEphemeralStorage),
	string(corev1.ResourceLimitsCPU),
	string(corev1.ResourceLimitsMemory),
	string(corev1.ResourceLimitsEphemeralStorage),
	string(corev1.ResourcePods),
	string(corev1.ResourceQuotas),
	string(corev1.ResourceServices),
	string(corev1.ResourceReplicationControllers),
	string(corev1.ResourceSecrets),
	string(corev1.ResourcePersistentVolumeClaims),
	string(corev1.ResourceConfigMaps),
	string(corev1.ResourceServicesNodePorts),
	string(corev1.ResourceServicesLoadBalancers),
)

// IsStandardQuotaResourceName returns true if the resource is known to
// the quota tracking system
func IsStandardQuotaResourceName(str string) bool {
	return standardQuotaResources.Has(str) || IsQuotaHugePageResourceName(corev1.ResourceName(str))
}

// IsQuotaHugePageResourceName returns true if the resource name has the quota
// related huge page resource prefix.
func IsQuotaHugePageResourceName(name corev1.ResourceName) bool {
	return strings.HasPrefix(string(name), corev1.ResourceHugePagesPrefix) || strings.HasPrefix(string(name), corev1.ResourceRequestsHugePagesPrefix)
}

// ValidateResourceQuantityValue enforces that specified quantity is valid for specified resource
func ValidateResourceQuantityValue(resource string, value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateNonnegativeQuantity(value, fldPath)...)
	if IsIntegerResourceName(resource) {
		if value.MilliValue()%int64(1000) != int64(0) {
			allErrs = append(allErrs, field.Invalid(fldPath, value, isNotIntegerErrorMsg))
		}
	}
	return allErrs
}

// Validates that a Quantity is not negative
func ValidateNonnegativeQuantity(value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if value.Cmp(resource.Quantity{}) < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value.String(), isNegativeErrorMsg))
	}
	return allErrs
}

var integerResources = sets.NewString(
	string(corev1.ResourcePods),
	string(corev1.ResourceQuotas),
	string(corev1.ResourceServices),
	string(corev1.ResourceReplicationControllers),
	string(corev1.ResourceSecrets),
	string(corev1.ResourceConfigMaps),
	string(corev1.ResourcePersistentVolumeClaims),
	string(corev1.ResourceServicesNodePorts),
	string(corev1.ResourceServicesLoadBalancers),
)

// IsIntegerResourceName returns true if the resource is measured in integer values
func IsIntegerResourceName(str string) bool {
	return integerResources.Has(str) || IsExtendedResourceName(corev1.ResourceName(str))
}

// IsExtendedResourceName returns true if:
// 1. the resource name is not in the default namespace;
// 2. resource name does not have "requests." prefix,
// to avoid confusion with the convention in quota
// 3. it satisfies the rules in IsQualifiedName() after converted into quota resource name
func IsExtendedResourceName(name corev1.ResourceName) bool {
	if IsNativeResource(name) || strings.HasPrefix(string(name), corev1.DefaultResourceRequestsPrefix) {
		return false
	}
	// Ensure it satisfies the rules in IsQualifiedName() after converted into quota resource name
	nameForQuota := fmt.Sprintf("%s%s", corev1.DefaultResourceRequestsPrefix, string(name))
	if errs := validation.IsQualifiedName(string(nameForQuota)); len(errs) != 0 {
		return false
	}
	return true
}

// IsNativeResource returns true if the resource name is in the
// *kubernetes.io/ namespace. Partially-qualified (unprefixed) names are
// implicitly in the kubernetes.io/ namespace.
func IsNativeResource(name corev1.ResourceName) bool {
	return !strings.Contains(string(name), "/") ||
		strings.Contains(string(name), corev1.ResourceDefaultNamespacePrefix)
}

var standardResourceQuotaScopes = sets.NewString(
	string(corev1.ResourceQuotaScopeTerminating),
	string(corev1.ResourceQuotaScopeNotTerminating),
	string(corev1.ResourceQuotaScopeBestEffort),
	string(corev1.ResourceQuotaScopeNotBestEffort),
	string(corev1.ResourceQuotaScopePriorityClass),
)

// IsStandardResourceQuotaScope returns true if the scope is a standard value
func IsStandardResourceQuotaScope(str string) bool {
	return standardResourceQuotaScopes.Has(str)
}

var podObjectCountQuotaResources = sets.NewString(
	string(corev1.ResourcePods),
)

var podComputeQuotaResources = sets.NewString(
	string(corev1.ResourceCPU),
	string(corev1.ResourceMemory),
	string(corev1.ResourceLimitsCPU),
	string(corev1.ResourceLimitsMemory),
	string(corev1.ResourceRequestsCPU),
	string(corev1.ResourceRequestsMemory),
)

// IsResourceQuotaScopeValidForResource returns true if the resource applies to the specified scope
func IsResourceQuotaScopeValidForResource(scope corev1.ResourceQuotaScope, resource string) bool {
	switch scope {
	case corev1.ResourceQuotaScopeTerminating, corev1.ResourceQuotaScopeNotTerminating, corev1.ResourceQuotaScopeNotBestEffort, corev1.ResourceQuotaScopePriorityClass:
		return podObjectCountQuotaResources.Has(resource) || podComputeQuotaResources.Has(resource)
	case corev1.ResourceQuotaScopeBestEffort:
		return podObjectCountQuotaResources.Has(resource)
	default:
		return true
	}
}
