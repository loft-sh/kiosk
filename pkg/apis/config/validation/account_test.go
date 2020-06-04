package validation

import (
	"testing"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type testAccountValidation struct {
	in    *configv1alpha1.Account
	inOld *configv1alpha1.Account

	valid bool
}

func TestAccountValidation(t *testing.T) {
	tests := map[string]*testAccountValidation{
		"Invalid account": &testAccountValidation{
			valid: false,
			in: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: configv1alpha1.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind: "ServiceAccount",
							Name: "test",
						},
					},
				},
			},
		},
		"Invalid account space template": &testAccountValidation{
			valid: false,
			in: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: configv1alpha1.AccountSpec{
					Space: configv1alpha1.AccountSpace{
						SpaceTemplate: configv1alpha1.AccountSpaceTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{constants.SpaceLabelAccount: "fake"},
							},
						},
					},
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
		"Valid account space template": &testAccountValidation{
			valid: true,
			in: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: configv1alpha1.AccountSpec{
					Space: configv1alpha1.AccountSpace{
						SpaceTemplate: configv1alpha1.AccountSpaceTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"fake": "test"},
							},
						},
					},
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
		"Valid account": &testAccountValidation{
			valid: true,
			in: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: configv1alpha1.AccountSpec{
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
			inOld: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: configv1alpha1.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "test",
							Namespace: "test",
						},
					},
				},
				Status: configv1alpha1.AccountStatus{
					Namespaces: []configv1alpha1.AccountNamespaceStatus{
						configv1alpha1.AccountNamespaceStatus{
							Name: "test",
						},
					},
				},
			},
			in: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: configv1alpha1.AccountSpec{
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
		"Invalid account space template update": &testAccountValidation{
			valid: false,
			inOld: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: configv1alpha1.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "testfabian",
							Namespace: "test",
						},
					},
				},
			},
			in: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: configv1alpha1.AccountSpec{
					Space: configv1alpha1.AccountSpace{
						SpaceTemplate: configv1alpha1.AccountSpaceTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{constants.SpaceLabelAccount: "fake"},
							},
						},
					},
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
		"Valid account space template update": &testAccountValidation{
			valid: true,
			inOld: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: configv1alpha1.AccountSpec{
					Space: configv1alpha1.AccountSpace{
						SpaceTemplate: configv1alpha1.AccountSpaceTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"fake": "test"},
							},
						},
					},
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "testfabian",
							Namespace: "test",
						},
					},
				},
			},
			in: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: configv1alpha1.AccountSpec{
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
			inOld: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: configv1alpha1.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "testfabian",
							Namespace: "test",
						},
					},
				},
			},
			in: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "12345",
				},
				Spec: configv1alpha1.AccountSpec{
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
