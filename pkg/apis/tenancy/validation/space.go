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
	if len(space.Annotations) > 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "annotations"), space.Annotations, "field is immutable, try updating the namespace"))
	}
	if len(space.Labels) > 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "labels"), space.Labels, "field is immutable, try updating the namespace"))
	}

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

	// TODO this restriction exists because our authorizer/admission cannot properly express and restrict mutation on the field level.
	for name, value := range newSpace.Annotations {
		if value != oldSpace.Annotations[name] {
			allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "annotations").Key(name), value, "field is immutable, try updating the namespace"))
		}
	}
	// check for deletions
	for name, value := range oldSpace.Annotations {
		if _, inNew := newSpace.Annotations[name]; !inNew {
			allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "annotations").Key(name), value, "field is immutable, try updating the namespace"))
		}
	}

	for name, value := range newSpace.Labels {
		if value != oldSpace.Labels[name] {
			allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "labels").Key(name), value, "field is immutable, , try updating the namespace"))
		}
	}
	for name, value := range oldSpace.Labels {
		if _, inNew := newSpace.Labels[name]; !inNew {
			allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "labels").Key(name), value, "field is immutable, try updating the namespace"))
		}
	}

	return allErrs
}
