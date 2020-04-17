package validation

import (
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateTemplateInstance tests required fields for an account quota
func ValidateTemplateInstance(templateInstance *configv1alpha1.TemplateInstance) field.ErrorList {
	return field.ErrorList{}
}

// ValidateTemplateInstanceUpdate tests updated fields for an account quota
func ValidateTemplateInstanceUpdate(newTemplateInstance *configv1alpha1.TemplateInstance, oldTemplateInstance *configv1alpha1.TemplateInstance) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newTemplateInstance.Spec.Template, oldTemplateInstance.Spec.Template, field.NewPath("spec", "template"))...)

	return allErrs
}
