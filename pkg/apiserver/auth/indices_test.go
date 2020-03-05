package auth

import (
	"testing"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	"github.com/kiosk-sh/kiosk/pkg/util"
	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type indexTest struct {
	in       runtime.Object
	field    string
	expected []string
}

func TestIndices(t *testing.T) {
	tests := map[string]*indexTest{
		"account": &indexTest{
			field: constants.IndexBySubjects,
			in: &configv1alpha1.Account{
				Spec: configv1alpha1.AccountSpec{
					Subjects: []rbacv1.Subject{
						rbacv1.Subject{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "User",
							Name:     "foo",
						},
						rbacv1.Subject{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "Group",
							Name:     "foo",
						},
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "foo",
							Namespace: "default",
						},
					},
				},
			},
			expected: []string{
				ConvertSubject("", &rbacv1.Subject{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "User",
					Name:     "foo",
				}),
				ConvertSubject("", &rbacv1.Subject{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Group",
					Name:     "foo",
				}),
				ConvertSubject("", &rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      "foo",
					Namespace: "default",
				}),
			},
		},
		"namespace": &indexTest{
			field: constants.IndexByAccount,
			in: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						tenancy.SpaceLabelAccount: "foo",
					},
				},
			},
			expected: []string{"foo"},
		},
		"roleBindingBySubjects": &indexTest{
			field: constants.IndexBySubjects,
			in: &rbacv1.RoleBinding{
				Subjects: []rbacv1.Subject{
					rbacv1.Subject{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "User",
						Name:     "foo",
					},
				},
			},
			expected: []string{ConvertSubject("", &rbacv1.Subject{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "User",
				Name:     "foo",
			})},
		},
		"roleBindingByRoleRef": &indexTest{
			field: constants.IndexByRole,
			in: &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "foo",
				},
			},
			expected: []string{"test/foo"},
		},
		"roleBindingByClusterRoleRef": &indexTest{
			field: constants.IndexByClusterRole,
			in: &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "ClusterRole",
					Name:     "foo",
				},
			},
			expected: []string{"foo"},
		},
		"cluster rolebinding by subjects": &indexTest{
			field: constants.IndexBySubjects,
			in: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{
					rbacv1.Subject{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "User",
						Name:     "foo",
					},
				},
			},
			expected: []string{ConvertSubject("", &rbacv1.Subject{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "User",
				Name:     "foo",
			})},
		},
		"cluster rolebinding by ClusterRoleRef": &indexTest{
			field: constants.IndexByClusterRole,
			in: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "ClusterRole",
					Name:     "foo",
				},
			},
			expected: []string{"foo"},
		},
	}

	fakeIndexer := &fakeIndexer{
		scheme: testingutil.NewScheme(),
	}

	registerIndices(fakeIndexer)
	for testName, test := range tests {
		out, err := fakeIndexer.GetIndexValues(test.in, test.field)
		if err != nil {
			t.Fatal(err)
		}

		if !util.StringsEqual(out, test.expected) {
			t.Fatalf("Test %s: expected %#+v, but got %#+v", testName, test.expected, out)
		}
	}
}

type fakeIndexer struct {
	scheme  *runtime.Scheme
	indices map[schema.GroupVersionKind]map[string]client.IndexerFunc
}

func (fi *fakeIndexer) GetIndexValues(obj runtime.Object, field string) ([]string, error) {
	gvk, err := apiutil.GVKForObject(obj, fi.scheme)
	if err != nil {
		return nil, err
	}

	return fi.indices[gvk][field](obj), nil
}

func (fi *fakeIndexer) IndexField(obj runtime.Object, field string, extractValue client.IndexerFunc) error {
	gvk, err := apiutil.GVKForObject(obj, fi.scheme)
	if err != nil {
		return err
	}
	if fi.indices == nil {
		fi.indices = map[schema.GroupVersionKind]map[string]client.IndexerFunc{}
	}
	if fi.indices[gvk] == nil {
		fi.indices[gvk] = map[string]client.IndexerFunc{}
	}

	fi.indices[gvk][field] = extractValue
	return nil
}
