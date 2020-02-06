package account

import (
	"context"

	fakeauth "github.com/kiosk-sh/kiosk/pkg/apiserver/auth/testing"
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

func TestBasic(t *testing.T) {
	accountStorage := &accountStorage{}

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
	fakeAuthCache := fakeauth.NewFakeAuthCache()
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	accountStorage := NewAccountStorage(fakeClient, fakeAuthCache).(*accountStorage)

	// We are not allowed to retrieve it so this should return a not found
	_, err := accountStorage.Get(userCtx, "test", &metav1.GetOptions{})
	if err == nil || kerrors.IsNotFound(err) == false {
		t.Fatalf("Expected not found error, got %v", err)
	}

	// Change the auth cache that allows us to retrieve the account
	fakeAuthCache.UserAccounts["foo"] = []string{"test"}

	// We are not allowed to retrieve it so this should return a not found
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
		})
	fakeAuthCache := fakeauth.NewFakeAuthCache()
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	accountStorage := NewAccountStorage(fakeClient, fakeAuthCache).(*accountStorage)

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
	fakeAuthCache.UserAccounts["foo"] = []string{"test", "test2"}

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
	fakeAuthCache := fakeauth.NewFakeAuthCache()
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	accountStorage := NewAccountStorage(fakeClient, fakeAuthCache).(*accountStorage)

	// Try to create if we are not allowed to
	_, err := accountStorage.Create(userCtx, &tenancy.Account{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test3",
		},
	}, fakeCreateValidation, &metav1.CreateOptions{})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	// Allow us to create the account
	fakeAuthCache.UserAccounts["foo"] = []string{"*"}
	_, err = accountStorage.Create(userCtx, &tenancy.Account{
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
	fakeAuthCache := fakeauth.NewFakeAuthCache()
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	accountStorage := NewAccountStorage(fakeClient, fakeAuthCache).(*accountStorage)

	_, updated, err := accountStorage.Update(userCtx, "test", &fakeUpdater{out: &tenancy.Account{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"Updated": "true",
			},
		},
	}}, fakeCreateValidation, fakeUpdateValidation, false, &metav1.UpdateOptions{})
	if err == nil || kerrors.IsForbidden(err) == false || updated == true {
		t.Fatalf("Expected forbidden error, got %v", err)
	}

	// Allow account update
	fakeAuthCache.UserAccounts["foo"] = []string{"*"}

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
	fakeAuthCache := fakeauth.NewFakeAuthCache()
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	accountStorage := NewAccountStorage(fakeClient, fakeAuthCache).(*accountStorage)

	_, deleted, err := accountStorage.Delete(userCtx, "test", fakeDeleteValidation, &metav1.DeleteOptions{})
	if err == nil || kerrors.IsForbidden(err) == false || deleted == true {
		t.Fatalf("Expected forbidden error, got %v", err)
	}

	// Allow account delete
	fakeAuthCache.UserAccounts["foo"] = []string{"test"}

	_, deleted, err = accountStorage.Delete(userCtx, "test", fakeDeleteValidation, &metav1.DeleteOptions{})
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
