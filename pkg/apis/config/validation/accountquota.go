package validation

import (
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateAccountQuota tests required fields for an account quota
func ValidateAccountQuota(accountQuota *configv1alpha1.AccountQuota) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateAccountQuotaSpec(&accountQuota.Spec)...)

	return allErrs
}

// ValidateAccountQuotaUpdate tests updated fields for an account quota
func ValidateAccountQuotaUpdate(newAccountQuota *configv1alpha1.AccountQuota, oldAccountQuota *configv1alpha1.AccountQuota) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newAccountQuota.Spec.Account, oldAccountQuota.Spec.Account, field.NewPath("spec", "account"))...)
	allErrs = append(allErrs, ValidateAccountQuotaSpec(&newAccountQuota.Spec)...)

	return allErrs
}

func ValidateAccountQuotaSpec(accountQuotaSpec *configv1alpha1.AccountQuotaSpec) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateResourceQuotaSpec(&accountQuotaSpec.Quota, field.NewPath("spec", "quota"))...)
	return allErrs
}
