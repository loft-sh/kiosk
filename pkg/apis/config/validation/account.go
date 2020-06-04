package validation

import (
	"reflect"

	"github.com/kiosk-sh/kiosk/pkg/constants"

	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/util/convert"
)

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

func verifySpace(account *configv1alpha1.Account) field.ErrorList {
	return validateSpace(account.Spec.Space)
}

func validateSpace(space configv1alpha1.AccountSpace) field.ErrorList {
	allErrs := field.ErrorList{}

	spacePath := field.NewPath("spec.space")
	allErrs = append(allErrs, ValidateAccountSpaceTemplate(space.SpaceTemplate, spacePath)...)
	return allErrs
}

// ValidateAccount tests required fields for an account
func ValidateAccount(account *configv1alpha1.Account) field.ErrorList {
	result := field.ErrorList{}
	// Verify subjects
	result = append(result, verifySubjects(account)...)
	result = append(result, verifySpace(account)...)

	return result
}

// ValidateAccountUpdate tests updated fields for an account
func ValidateAccountUpdate(newAccount *configv1alpha1.Account, oldAccount *configv1alpha1.Account) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateAccount(newAccount)...)

	if !reflect.DeepEqual(newAccount.Status, oldAccount.Status) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status"), oldAccount.Status, "field is immutable"))
	}

	// Verify subjects
	allErrs = append(allErrs, verifySubjects(newAccount)...)
	allErrs = append(allErrs, verifySpace(newAccount)...)
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

// ValidateAccountSpaceTemplate is exported to validate space template labels and reuse this validation logic
func ValidateAccountSpaceTemplate(space configv1alpha1.AccountSpaceTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if !spaceTemplateLabelAccountValidation(space.Labels) {
		allErrs = append(allErrs, field.Required(fldPath.Child("metadata"), ""))
	}

	return allErrs
}

func spaceTemplateLabelAccountValidation(labels map[string]string) bool {
	for key, value := range labels {
		if key == constants.SpaceLabelAccount {
			if value != "" {
				return false
			}
		}
	}
	return true
}
