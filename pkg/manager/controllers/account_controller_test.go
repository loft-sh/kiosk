package controllers

import (
	"context"
	"testing"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/registry/util"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type accountControllerTest struct {
	account           *configv1alpha1.Account
	ownedNamespaces   []*corev1.Namespace
	ownedRoleBindings []*rbacv1.RoleBinding

	expectedRoleBindingInNamespace []string
	expectedAccountStatus          *configv1alpha1.AccountStatus
}

func TestAccountController(t *testing.T) {
	testRoleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.SchemeGroupVersion.Group,
		Kind:     "ClusterRole",
		Name:     "admin",
	}
	testSubjects := []rbacv1.Subject{
		rbacv1.Subject{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "User",
			Name:     "foo",
		},
	}

	tests := map[string]*accountControllerTest{
		"Status namespace update": &accountControllerTest{
			account: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: configv1alpha1.AccountSpec{
					Subjects: testSubjects,
				},
			},
			ownedNamespaces: []*corev1.Namespace{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			ownedRoleBindings: []*rbacv1.RoleBinding{
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					RoleRef:  testRoleRef,
					Subjects: testSubjects,
				},
			},
			expectedRoleBindingInNamespace: []string{"test"},
			expectedAccountStatus: &configv1alpha1.AccountStatus{
				Namespaces: []configv1alpha1.AccountNamespaceStatus{
					configv1alpha1.AccountNamespaceStatus{
						Name: "test",
					},
				},
			},
		},
		"Create/Update/Delete rolebinding": &accountControllerTest{
			account: &configv1alpha1.Account{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: configv1alpha1.AccountSpec{
					Subjects: testSubjects,
				},
			},
			ownedNamespaces: []*corev1.Namespace{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test2",
					},
					Status: corev1.NamespaceStatus{
						Phase: corev1.NamespaceActive,
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test4",
					},
				},
			},
			ownedRoleBindings: []*rbacv1.RoleBinding{
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					RoleRef:  testRoleRef,
					Subjects: testSubjects,
				},
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test2",
						Namespace: "test",
					},
					RoleRef:  testRoleRef,
					Subjects: testSubjects,
				},
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test2",
						Namespace: "test3",
					},
					RoleRef:  testRoleRef,
					Subjects: testSubjects,
				},
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test4",
					},
					RoleRef:  rbacv1.RoleRef{},
					Subjects: []rbacv1.Subject{},
				},
			},
			expectedRoleBindingInNamespace: []string{"test", "test2", "test4"},
			expectedAccountStatus: &configv1alpha1.AccountStatus{
				Namespaces: []configv1alpha1.AccountNamespaceStatus{
					configv1alpha1.AccountNamespaceStatus{
						Name: "test",
					},
					configv1alpha1.AccountNamespaceStatus{
						Name: "test2",
					},
					configv1alpha1.AccountNamespaceStatus{
						Name: "test4",
					},
				},
			},
		},
	}
	scheme := testingutil.NewScheme()

	for testName, test := range tests {
		fakeClient := testingutil.NewFakeClient(scheme)
		fakeClient.Create(context.TODO(), test.account)

		accountController := &AccountReconciler{
			Client: fakeClient,
			Log:    zap.New(func(o *zap.Options) {}),
			Scheme: scheme,
		}

		// Set owned namespaces
		ownedNamespaces := []runtime.Object{}
		for _, o := range test.ownedNamespaces {
			ownedNamespaces = append(ownedNamespaces, o)
			fakeClient.Create(context.TODO(), o)
		}

		fakeClient.SetIndexValue(corev1.SchemeGroupVersion.WithKind("Namespace"), constants.IndexByAccount, test.account.Name, ownedNamespaces)

		// Set owned role bindings
		ownedRoleBindings := []runtime.Object{}
		for _, o := range test.ownedRoleBindings {
			ownedRoleBindings = append(ownedRoleBindings, o)
			fakeClient.Create(context.TODO(), o)
		}

		fakeClient.SetIndexValue(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"), constants.IndexByAccount, test.account.Name, ownedRoleBindings)

		_, err := accountController.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: test.account.Name}})
		if err != nil {
			t.Fatalf("Test %s failed: %v", testName, err)
		}

		// Check if the status is equal
		err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: test.account.Name}, test.account)
		if err != nil {
			t.Fatal(err)
		}
		if !apiequality.Semantic.DeepEqual(&test.account.Status, test.expectedAccountStatus) {
			t.Fatalf("Status is not equal %#+v != %#+v", test.account.Status, test.expectedAccountStatus)
		}

		for _, namespace := range test.expectedRoleBindingInNamespace {
			roleBindingList := &rbacv1.RoleBindingList{}
			err := fakeClient.List(context.TODO(), roleBindingList, client.InNamespace(namespace))
			if err != nil {
				t.Fatal(err)
			}
			if len(roleBindingList.Items) != 1 {
				t.Fatalf("Test %s: expected 1 rolebinding in namespace %s, but got %d", testName, namespace, len(roleBindingList.Items))
			}
			if !apiequality.Semantic.DeepEqual(test.account.Spec.Subjects, roleBindingList.Items[0].Subjects) {
				t.Fatalf("Test %s: subjects are not equal between rolebinding and account", testName)
			}

			clusterRole := util.GetClusterRoleFor(test.account)
			if roleBindingList.Items[0].RoleRef.Name != clusterRole || roleBindingList.Items[0].RoleRef.Kind != "ClusterRole" {
				t.Fatalf("Test %s: invalid role ref (expected ClusterRole %s) %#+v", testName, clusterRole, roleBindingList.Items[0].RoleRef)
			}
		}
	}
}
