package validation

import (
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/api/validation/path"
	"reflect"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/util/convert"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/validation"
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

func verifySubjects(account *configv1alpha1.Account) field.ErrorList {
	subjects := []rbac.Subject{}
	err := convert.ObjectToObject(account.Spec.Subjects, &subjects)
	if err != nil {
		return field.ErrorList{field.InternalError(field.NewPath("spec.subjects"), err)}
	}

	return validateSubjects(subjects)
}

func validateSubjects(subjects []rbac.Subject) field.ErrorList {
	allErrs := field.ErrorList{}

	subjectsPath := field.NewPath("spec.subjects")
	for i, subject := range subjects {
		allErrs = append(allErrs, ValidateRoleBindingSubject(subject, false, subjectsPath.Index(i))...)
	}

	return allErrs
}

// ValidateAccount tests required fields for an account
func ValidateAccount(account *configv1alpha1.Account) field.ErrorList {
	result := validation.ValidateObjectMeta(&account.ObjectMeta, false, ValidateName, field.NewPath("metadata"))

	// Verify subjects
	result = append(result, verifySubjects(account)...)

	return result
}

// ValidateAccountUpdate tests updated fields for an account
func ValidateAccountUpdate(newAccount *configv1alpha1.Account, oldAccount *configv1alpha1.Account) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newAccount.ObjectMeta, &oldAccount.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateAccount(newAccount)...)

	if !reflect.DeepEqual(newAccount.Status, oldAccount.Status) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status"), oldAccount.Status, "field is immutable"))
	}

	// Verify subjects
	allErrs = append(allErrs, verifySubjects(newAccount)...)
	return allErrs
}

// ValidateRoleBindingSubject is exported to allow types outside of the RBAC API group to embed a rbac.Subject and reuse this validation logic
func ValidateRoleBindingSubject(subject rbac.Subject, isNamespaced bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(subject.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
	}

	switch subject.Kind {
	case rbac.ServiceAccountKind:
		if len(subject.Name) > 0 {
			for _, msg := range validation.ValidateServiceAccountName(subject.Name, false) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), subject.Name, msg))
			}
		}
		if len(subject.APIGroup) > 0 {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiGroup"), subject.APIGroup, []string{""}))
		}
		if !isNamespaced && len(subject.Namespace) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), ""))
		}

	case rbac.UserKind:
		// TODO(ericchiang): What other restrictions on user name are there?
		if subject.APIGroup != rbac.GroupName {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiGroup"), subject.APIGroup, []string{rbac.GroupName}))
		}

	case rbac.GroupKind:
		// TODO(ericchiang): What other restrictions on group name are there?
		if subject.APIGroup != rbac.GroupName {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiGroup"), subject.APIGroup, []string{rbac.GroupName}))
		}

	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kind"), subject.Kind, []string{rbac.ServiceAccountKind, rbac.UserKind, rbac.GroupKind}))
	}

	return allErrs
}
