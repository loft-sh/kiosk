package validation

import (
	"reflect"

	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"github.com/kiosk-sh/kiosk/pkg/util/convert"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	rbac "k8s.io/kubernetes/pkg/apis/rbac"
	rbacvalidation "k8s.io/kubernetes/pkg/apis/rbac/validation"
)

func verifySubjects(account *tenancy.Account) field.ErrorList {
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
		allErrs = append(allErrs, rbacvalidation.ValidateRoleBindingSubject(subject, false, subjectsPath.Index(i))...)
	}

	return allErrs
}

// ValidateAccount tests required fields for an account
func ValidateAccount(account *tenancy.Account) field.ErrorList {
	result := validation.ValidateObjectMeta(&account.ObjectMeta, false, ValidateName, field.NewPath("metadata"))

	// Verify subjects
	result = append(result, verifySubjects(account)...)

	return result
}

// ValidateAccountUpdate tests updated fields for an account
func ValidateAccountUpdate(newAccount *tenancy.Account, oldAccount *tenancy.Account) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newAccount.ObjectMeta, &oldAccount.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateAccount(newAccount)...)

	if !reflect.DeepEqual(newAccount.Status, oldAccount.Status) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status"), oldAccount.Status, "field is immutable"))
	}

	// Verify subjects
	allErrs = append(allErrs, verifySubjects(newAccount)...)
	return allErrs
}
