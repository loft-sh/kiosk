package auth

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"
	rbacv1 "k8s.io/api/rbac/v1"
)

type accessorTestCase struct {
	subject string
	verb    string

	roles               []*rbacv1.Role
	roleBindings        []*rbacv1.RoleBinding
	clusterRoles        []*rbacv1.ClusterRole
	clusterRoleBindings []*rbacv1.ClusterRoleBinding
	accounts            []*configv1alpha1.Account

	expected      []string
	expectedError bool
}

func TestAllowedNamespaces(t *testing.T) {
	tests := map[string]*accessorTestCase{
		"No Namespaces found": &accessorTestCase{
			subject:  "user:foo",
			verb:     "create",
			expected: []string{},
		},
		"All Namespaces allowed": &accessorTestCase{
			subject: "user:foo",
			verb:    "create",
			clusterRoles: []*rbacv1.ClusterRole{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Rules: []rbacv1.PolicyRule{
						rbacv1.PolicyRule{
							Verbs:     []string{"*"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
			clusterRoleBindings: []*rbacv1.ClusterRoleBinding{
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "ClusterRole",
						Name:     "test",
					},
				},
			},
			expected: []string{"*"},
		},
		"Single Namespace via ClusterRole allowed": &accessorTestCase{
			subject: "user:foo",
			verb:    "create",
			clusterRoles: []*rbacv1.ClusterRole{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Rules: []rbacv1.PolicyRule{
						rbacv1.PolicyRule{
							Verbs:         []string{"*"},
							APIGroups:     []string{""},
							Resources:     []string{"namespaces"},
							ResourceNames: []string{"test"},
						},
					},
				},
			},
			clusterRoleBindings: []*rbacv1.ClusterRoleBinding{
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "ClusterRole",
						Name:     "test",
					},
				},
			},
			expected: []string{"test"},
		},
		"Single Namespace via RoleBinding allowed": &accessorTestCase{
			subject: "user:foo",
			verb:    "create",
			clusterRoles: []*rbacv1.ClusterRole{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Rules: []rbacv1.PolicyRule{
						rbacv1.PolicyRule{
							Verbs:     []string{"*"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
			roleBindings: []*rbacv1.RoleBinding{
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "ClusterRole",
						Name:     "test",
					},
				},
			},
			expected: []string{"test"},
		},
		"Single Namespace via Role allowed": &accessorTestCase{
			subject: "user:foo",
			verb:    "create",
			roles: []*rbacv1.Role{
				&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Rules: []rbacv1.PolicyRule{
						rbacv1.PolicyRule{
							Verbs:     []string{"*"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
			roleBindings: []*rbacv1.RoleBinding{
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "Role",
						Name:     "test",
					},
				},
			},
			expected: []string{"test"},
		},
	}

	scheme := testingutil.NewScheme()
	for testName, test := range tests {
		client := testingutil.NewFakeClient(scheme)
		accessor := &accessor{
			client: client,
		}

		// Add cluster role bindings
		if len(test.clusterRoleBindings) > 0 {
			objs := []runtime.Object{}
			for _, o := range test.clusterRoleBindings {
				objs = append(objs, o)
			}
			client.SetIndexValue(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"), constants.IndexBySubjects, test.subject, objs)
		}

		// Add role bindings
		if len(test.roleBindings) > 0 {
			objs := []runtime.Object{}
			for _, o := range test.roleBindings {
				objs = append(objs, o)
			}
			client.SetIndexValue(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"), constants.IndexBySubjects, test.subject, objs)
		}

		// Add clusterroles
		for _, o := range test.clusterRoles {
			client.Create(context.TODO(), o)
		}

		// Add roles
		for _, o := range test.roles {
			client.Create(context.TODO(), o)
		}

		namespaces, err := accessor.RetrieveAllowedNamespaces(context.TODO(), test.subject, test.verb)
		if test.expectedError && err == nil {
			t.Fatalf("Test %s: expected error but got nil", testName)
		} else if !test.expectedError && err != nil {
			t.Fatalf("Test %s: expexted no error but got %v", testName, err)
		}

		// Check if all namespaces are there
		if len(namespaces) != len(test.expected) {
			t.Fatalf("Test %s: got namespaces %#+v, but expected %#+v", testName, namespaces, test.expected)
		}
		for _, namespace := range namespaces {
			found := false
			for _, expected := range test.expected {
				if expected == namespace {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("Test %s: got namespaces %#+v, but expected %#+v", testName, namespaces, test.expected)
			}
		}
	}
}

func TestAllowedAccounts(t *testing.T) {
	tests := map[string]*accessorTestCase{
		"No accounts found": &accessorTestCase{
			subject:  "user:foo",
			verb:     "create",
			expected: []string{},
		},
		"All accounts allowed": &accessorTestCase{
			subject: "user:foo",
			verb:    "create",
			clusterRoles: []*rbacv1.ClusterRole{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Rules: []rbacv1.PolicyRule{
						rbacv1.PolicyRule{
							Verbs:     []string{"*"},
							APIGroups: []string{"*"},
							Resources: []string{"accounts"},
						},
					},
				},
			},
			clusterRoleBindings: []*rbacv1.ClusterRoleBinding{
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "ClusterRole",
						Name:     "test",
					},
				},
			},
			expected: []string{"*"},
		},
		"Single account via ClusterRole allowed": &accessorTestCase{
			subject: "user:foo",
			verb:    "create",
			clusterRoles: []*rbacv1.ClusterRole{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Rules: []rbacv1.PolicyRule{
						rbacv1.PolicyRule{
							Verbs:         []string{"*"},
							APIGroups:     []string{configv1alpha1.GroupVersion.Group},
							Resources:     []string{"accounts"},
							ResourceNames: []string{"test"},
						},
					},
				},
			},
			clusterRoleBindings: []*rbacv1.ClusterRoleBinding{
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "ClusterRole",
						Name:     "test",
					},
				},
			},
			expected: []string{"test"},
		},
		"No account for verb allowed": &accessorTestCase{
			subject: "user:foo",
			verb:    "create",
			clusterRoles: []*rbacv1.ClusterRole{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Rules: []rbacv1.PolicyRule{
						rbacv1.PolicyRule{
							Verbs:         []string{"get"},
							APIGroups:     []string{configv1alpha1.GroupVersion.Group},
							Resources:     []string{"accounts"},
							ResourceNames: []string{"test"},
						},
					},
				},
			},
			clusterRoleBindings: []*rbacv1.ClusterRoleBinding{
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "ClusterRole",
						Name:     "test",
					},
				},
			},
			expected: []string{},
		},
		"Account allowed through account subject": &accessorTestCase{
			subject: "user:foo",
			verb:    "get",
			accounts: []*configv1alpha1.Account{
				&configv1alpha1.Account{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testaccount",
					},
				},
			},
			clusterRoles: []*rbacv1.ClusterRole{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Rules: []rbacv1.PolicyRule{
						rbacv1.PolicyRule{
							Verbs:         []string{"create"},
							APIGroups:     []string{configv1alpha1.GroupVersion.Group},
							Resources:     []string{"accounts"},
							ResourceNames: []string{"test"},
						},
					},
				},
			},
			clusterRoleBindings: []*rbacv1.ClusterRoleBinding{
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "ClusterRole",
						Name:     "test",
					},
				},
			},
			expected: []string{"testaccount"},
		},
	}

	scheme := testingutil.NewScheme()
	for testName, test := range tests {
		client := testingutil.NewFakeClient(scheme)
		accessor := &accessor{
			client: client,
		}

		// Add accounts
		if len(test.accounts) > 0 {
			objs := []runtime.Object{}
			for _, o := range test.accounts {
				objs = append(objs, o)
			}
			client.SetIndexValue(configv1alpha1.GroupVersion.WithKind("Account"), constants.IndexBySubjects, test.subject, objs)
		}

		// Add cluster role bindings
		if len(test.clusterRoleBindings) > 0 {
			objs := []runtime.Object{}
			for _, o := range test.clusterRoleBindings {
				objs = append(objs, o)
			}
			client.SetIndexValue(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"), constants.IndexBySubjects, test.subject, objs)
		}

		// Add clusterroles
		for _, o := range test.clusterRoles {
			client.Create(context.TODO(), o)
		}

		accounts, err := accessor.RetrieveAllowedAccounts(context.TODO(), test.subject, test.verb)
		if test.expectedError && err == nil {
			t.Fatalf("Test %s: expected error but got nil", testName)
		} else if !test.expectedError && err != nil {
			t.Fatalf("Test %s: expexted no error but got %v", testName, err)
		}

		// Check if all accounts are there
		if len(accounts) != len(test.expected) {
			t.Fatalf("Test %s: got accounts %#+v, but expected %#+v", testName, accounts, test.expected)
		}
		for _, account := range accounts {
			found := false
			for _, expected := range test.expected {
				if expected == account {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("Test %s: got accounts %#+v, but expected %#+v", testName, accounts, test.expected)
			}
		}
	}
}
