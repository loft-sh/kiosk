package validation

import (
	"reflect"

	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"k8s.io/apimachinery/pkg/api/validation"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateName(name string, prefix bool) []string {
	if reasons := path.ValidatePathSegmentName(name, prefix); len(reasons) != 0 {
		return reasons
	}

	if len(name) < 2 {
		return []string{"must be at least 2 characters long"}
	}

	if reasons := apimachineryvalidation.ValidateNamespaceName(name, false); len(reasons) != 0 {
		return reasons
	}
	return nil
}

// ValidateSpace tests required fields for a Space.
func ValidateSpace(space *tenancy.Space) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&space.ObjectMeta, false, ValidateName, field.NewPath("metadata"))
	return allErrs
}

// ValidateSpaceUpdate tests to make sure a space update can be applied. Modifies newSpace with immutable fields.
func ValidateSpaceUpdate(newSpace *tenancy.Space, oldSpace *tenancy.Space) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newSpace.ObjectMeta, &oldSpace.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateSpace(newSpace)...)

	if !reflect.DeepEqual(newSpace.Spec, oldSpace.Spec) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec"), oldSpace.Spec, "field is immutable"))
	}
	if !reflect.DeepEqual(newSpace.Status, oldSpace.Status) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status"), oldSpace.Status, "field is immutable"))
	}

	return allErrs
}
