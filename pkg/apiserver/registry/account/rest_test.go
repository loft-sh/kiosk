package account

import (
	"context"
	tenancyv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/tenancy/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"

	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"

	"testing"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

var clusterBinding = &rbacv1.ClusterRoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name:            "test",
		UID:             "123",
		ResourceVersion: "1",
	},
	Subjects: []rbacv1.Subject{
		{
			Kind:     "User",
			Name:     "foo",
			APIGroup: rbacv1.GroupName,
		},
	},
	RoleRef: rbacv1.RoleRef{
		Name:     "test",
		Kind:     "ClusterRole",
		APIGroup: rbacv1.GroupName,
	},
}

func TestBasic(t *testing.T) {
	accountStorage := &accountREST{}

	if accountStorage.NamespaceScoped() == true {
		t.Fatal("Expected cluster scope")
	}
	if _, ok := accountStorage.New().(*tenancy.Account); !ok {
		t.Fatal("Wrong type in New")
	}
	if _, ok := accountStorage.NewList().(*tenancy.AccountList); !ok {
		t.Fatal("Wrong type in NewList")
	}
}

func TestGetAccount(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := testingutil.NewFakeClient(scheme, &configv1alpha1.Account{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	})
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	accountStorage := NewAccountREST(fakeClient, scheme).(*accountREST)
	test, err := accountStorage.Get(userCtx, "test", &metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	testAccount, ok := test.(*tenancy.Account)
	if !ok {
		t.Fatalf("returned account is not a tenancy account")
	} else if testAccount.Name != "test" {
		t.Fatalf("expected account with name test, got %s", testAccount.Name)
	}
}

func TestListAccount(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := testingutil.NewFakeClient(scheme,
		&configv1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &configv1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
				Labels: map[string]string{
					"testlabel": "test",
				},
			},
		}, &configv1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test3",
				Labels: map[string]string{
					"testlabel": "test",
				},
			},
		}, &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test",
				UID:             "123",
				ResourceVersion: "1",
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:           []string{"*"},
					APIGroups:       []string{tenancy.SchemeGroupVersion.Group},
					Resources:       []string{"*"},
					ResourceNames:   []string{"test", "test2"},
					NonResourceURLs: []string{"*"},
				},
			},
			AggregationRule: nil,
		})
	ctx := context.TODO()
	userCtx := withRequestInfo(request.WithUser(ctx, &user.DefaultInfo{Name: "foo"}), "list", "")
	accountStorage := NewAccountREST(fakeClient, scheme).(*accountREST)

	// Get empty list
	obj, err := accountStorage.List(userCtx, &metainternalversion.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	accountList, ok := obj.(*tenancy.AccountList)
	if !ok {
		t.Fatalf("Expected account list, got %#+v", obj)
	} else if len(accountList.Items) != 0 {
		t.Fatalf("Expected empty account list, got %d items", len(accountList.Items))
	}

	// Allow user to see 2 accounts
	fakeClient.Create(context.TODO(), clusterBinding)

	obj, err = accountStorage.List(userCtx, &metainternalversion.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	accountList, ok = obj.(*tenancy.AccountList)
	if !ok {
		t.Fatalf("Expected account list, got %#+v", obj)
	} else if len(accountList.Items) != 2 {
		t.Fatalf("Expected account list with 2 items, got %d items", len(accountList.Items))
	}

	// Filter list by label selector
	selector, err := labels.Parse("testlabel=test")
	if err != nil {
		t.Fatal(err)
	}
	obj, err = accountStorage.List(userCtx, &metainternalversion.ListOptions{LabelSelector: selector})
	if err != nil {
		t.Fatal(err)
	}
	accountList, ok = obj.(*tenancy.AccountList)
	if !ok {
		t.Fatalf("Expected account list, got %#+v", obj)
	} else if len(accountList.Items) != 1 || accountList.Items[0].Name != "test2" {
		t.Fatalf("Expected account list with 1 items, got %d items", len(accountList.Items))
	}
}

func TestCreateAccount(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := testingutil.NewFakeClient(scheme,
		&configv1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &configv1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
			},
		})
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	accountStorage := NewAccountREST(fakeClient, scheme).(*accountREST)

	// Allow us to create the account
	// fakeAuthCache.UserAccounts["foo"] = []string{"*"}
	_, err := accountStorage.Create(userCtx, &tenancy.Account{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test3",
		},
	}, fakeCreateValidation, &metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Expected no error but got %v", err)
	}

	// Try to create an account that already exists
	_, err = accountStorage.Create(userCtx, &tenancy.Account{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test3",
		},
	}, fakeCreateValidation, &metav1.CreateOptions{})
	if err != nil {
		if kerrors.IsAlreadyExists(err) == false {
			t.Fatalf("Expected already exists but got %v", err)
		}
	}
}

func TestAccountUpdate(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := testingutil.NewFakeClient(scheme,
		&configv1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &configv1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
			},
		})
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	accountStorage := NewAccountREST(fakeClient, scheme).(*accountREST)

	newObj, updated, err := accountStorage.Update(userCtx, "test", &fakeUpdater{out: &tenancy.Account{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "123456",
			Labels: map[string]string{
				"Updated": "true",
			},
		},
	}}, fakeCreateValidation, fakeUpdateValidation, false, &metav1.UpdateOptions{})
	if err != nil || updated == false {
		t.Fatalf("Expected no error, got %v", err)
	}
	if newObj.(*tenancy.Account).Labels["Updated"] != "true" {
		t.Fatal("Unexpected object returned")
	}
}

func TestAccountDelete(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := testingutil.NewFakeClient(scheme,
		&configv1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &configv1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
			},
		})
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	accountStorage := NewAccountREST(fakeClient, scheme).(*accountREST)

	_, deleted, err := accountStorage.Delete(userCtx, "test", fakeDeleteValidation, &metav1.DeleteOptions{})
	if err != nil || deleted == false {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func fakeCreateValidation(ctx context.Context, obj runtime.Object) error {
	return nil
}
func fakeUpdateValidation(ctx context.Context, obj, old runtime.Object) error {
	return nil
}
func fakeDeleteValidation(ctx context.Context, obj runtime.Object) error {
	return nil
}

type fakeUpdater struct {
	out runtime.Object
}

func (f *fakeUpdater) Preconditions() *metav1.Preconditions {
	return nil
}
func (f *fakeUpdater) UpdatedObject(ctx context.Context, oldObj runtime.Object) (newObj runtime.Object, err error) {
	return f.out, nil
}

func withRequestInfo(ctx context.Context, verb string, name string) context.Context {
	return request.WithRequestInfo(ctx, &request.RequestInfo{
		IsResourceRequest: true,
		Path:              "/apis/" + tenancy.SchemeGroupVersion.Group + "/" + tenancyv1alpha1.SchemeGroupVersion.Version,
		Verb:              verb,
		APIPrefix:         "",
		APIGroup:          tenancyv1alpha1.SchemeGroupVersion.Group,
		APIVersion:        tenancy.SchemeGroupVersion.Version,
		Namespace:         "",
		Resource:          "accounts",
		Subresource:       "",
		Name:              name,
	})
}
