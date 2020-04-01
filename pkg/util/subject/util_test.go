package subject

import (
	"github.com/kiosk-sh/kiosk/pkg/constants"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

type convertSubjectTest struct {
	namespace string
	subject   *rbacv1.Subject

	expected string
}

func TestConvertSubject(t *testing.T) {
	tests := map[string]*convertSubjectTest{
		"User": &convertSubjectTest{
			namespace: "test",
			subject: &rbacv1.Subject{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "User",
				Name:     "foo",
			},
			expected: constants.UserPrefix + "foo",
		},
		"Group": &convertSubjectTest{
			namespace: "test",
			subject: &rbacv1.Subject{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "Group",
				Name:     "foo",
			},
			expected: constants.GroupPrefix + "foo",
		},
		"SeriveAccount no namespace": &convertSubjectTest{
			namespace: "test",
			subject: &rbacv1.Subject{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ServiceAccount",
				Name:     "foo",
			},
			expected: constants.UserPrefix + "system:serviceaccount:test:foo",
		},
		"SeriveAccount with namespace": &convertSubjectTest{
			namespace: "test",
			subject: &rbacv1.Subject{
				APIGroup:  rbacv1.SchemeGroupVersion.Group,
				Kind:      "ServiceAccount",
				Name:      "foo",
				Namespace: "loo",
			},
			expected: constants.UserPrefix + "system:serviceaccount:loo:foo",
		},
	}

	for testName, test := range tests {
		real := ConvertSubject(test.namespace, test.subject)
		if real != test.expected {
			t.Fatalf("Test %s: exptected %s but got %s", testName, test.expected, real)
		}
	}
}
