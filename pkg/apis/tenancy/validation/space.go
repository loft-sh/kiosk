package validation

import (
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateSpace tests required fields for a Space.
func ValidateSpace(space *tenancy.Space) field.ErrorList {
	return field.ErrorList{}
}

// ValidateSpaceUpdate tests to make sure a space update can be applied. Modifies newSpace with immutable fields.
func ValidateSpaceUpdate(newSpace *tenancy.Space, oldSpace *tenancy.Space) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateSpace(newSpace)...)
	return allErrs
}
