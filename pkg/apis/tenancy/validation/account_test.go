package validation

import (
	"testing"

	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type testAccountValidation struct {
	in    *tenancy.Account
	inOld *tenancy.Account

	valid bool
}

func TestAccountValidation(t *testing.T) {
	tests := map[string]*testAccountValidation{
		"Invalid account": &testAccountValidation{
			valid: false,
			in: &tenancy.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: tenancy.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind: "ServiceAccount",
							Name: "test",
						},
					},
				},
			},
		},
		"Valid account": &testAccountValidation{
			valid: true,
			in: &tenancy.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: tenancy.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "test",
							Namespace: "test",
						},
					},
				},
			},
		},
		"Invalid update": &testAccountValidation{
			valid: false,
			inOld: &tenancy.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: tenancy.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "test",
							Namespace: "test",
						},
					},
				},
				Status: tenancy.AccountStatus{
					Namespaces: []tenancy.AccountNamespaceStatus{
						tenancy.AccountNamespaceStatus{
							Name: "test",
						},
					},
				},
			},
			in: &tenancy.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: tenancy.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "test",
							Namespace: "test",
						},
					},
				},
			},
		},
		"Valid update": &testAccountValidation{
			valid: true,
			inOld: &tenancy.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: tenancy.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "testfabian",
							Namespace: "test",
						},
					},
				},
			},
			in: &tenancy.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: tenancy.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "test",
							Namespace: "test",
						},
					},
				},
			},
		},
	}

	for testName, test := range tests {
		var errs field.ErrorList
		if test.inOld != nil {
			errs = ValidateAccountUpdate(test.in, test.inOld)
		} else {
			errs = ValidateAccount(test.in)
		}

		if test.valid && len(errs) > 0 {
			t.Fatalf("Test %s: expected account valid, but got %#+v", testName, errs)
		} else if !test.valid && len(errs) == 0 {
			t.Fatalf("Test %s: expected account invalid, but got no errors", testName)
		}
	}
}
