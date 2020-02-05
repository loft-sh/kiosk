package account

import (
	"context"
	fakeauth "github.com/kiosk-sh/kiosk/pkg/apiserver/auth/testing"
	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"testing"
)

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

}
