package validation

import (
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"k8s.io/apimachinery/pkg/api/validation"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateTemplateInstance tests required fields for an account quota
func ValidateTemplateInstance(templateInstance *configv1alpha1.TemplateInstance) field.ErrorList {
	result := validation.ValidateObjectMeta(&templateInstance.ObjectMeta, false, ValidateName, field.NewPath("metadata"))
	return result
}

// ValidateTemplateInstanceUpdate tests updated fields for an account quota
func ValidateTemplateInstanceUpdate(newTemplateInstance *configv1alpha1.TemplateInstance, oldTemplateInstance *configv1alpha1.TemplateInstance) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newTemplateInstance.ObjectMeta, &oldTemplateInstance.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newTemplateInstance.Spec.Template, oldTemplateInstance.Spec.Template, field.NewPath("spec", "template"))...)

	return allErrs
}
