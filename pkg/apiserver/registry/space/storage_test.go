package space

import (
	"testing"

	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	fakeauth "github.com/kiosk-sh/kiosk/pkg/apiserver/auth/testing"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"

	"context"
)

func TestBasic(t *testing.T) {
	spaceStorage := &spaceStorage{}

	if spaceStorage.NamespaceScoped() == true {
		t.Fatal("Expected cluster scope")
	}
	if _, ok := spaceStorage.New().(*tenancy.Space); !ok {
		t.Fatal("Wrong type in New")
	}
	if _, ok := spaceStorage.NewList().(*tenancy.SpaceList); !ok {
		t.Fatal("Wrong type in NewList")
	}
}

func TestGetSpace(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := testingutil.NewFakeClient(scheme, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	})
	fakeAuthCache := fakeauth.NewFakeAuthCache()
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	spaceStorage := NewSpaceStorage(fakeClient, fakeAuthCache, scheme).(*spaceStorage)

	// We are not allowed to retrieve it so this should return a not found
	_, err := spaceStorage.Get(userCtx, "test", &metav1.GetOptions{})
	if err == nil || kerrors.IsNotFound(err) == false {
		t.Fatalf("Expected not found error, got %v", err)
	}

	// Change the auth cache that allows us to retrieve the account
	fakeAuthCache.UserNamespaces["foo"] = []string{"test"}

	// We are not allowed to retrieve it so this should return a not found
	test, err := spaceStorage.Get(userCtx, "test", &metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	testSpace, ok := test.(*tenancy.Space)
	if !ok {
		t.Fatalf("returned space is not a tenancy space")
	} else if testSpace.Name != "test" {
		t.Fatalf("expected space with name test, got %s", testSpace.Name)
	}
}

func TestListSpaces(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := testingutil.NewFakeClient(scheme,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
				Labels: map[string]string{
					"testlabel": "test",
				},
			},
		}, &corev1.Namespace{
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
	spaceStorage := NewSpaceStorage(fakeClient, fakeAuthCache, scheme).(*spaceStorage)

	// Get empty list
	obj, err := spaceStorage.List(userCtx, &metainternalversion.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	spaceList, ok := obj.(*tenancy.SpaceList)
	if !ok {
		t.Fatalf("Expected space list, got %#+v", obj)
	} else if len(spaceList.Items) != 0 {
		t.Fatalf("Expected empty space list, got %d items", len(spaceList.Items))
	}

	// Allow user to see 2 namespaces
	fakeAuthCache.UserNamespaces["foo"] = []string{"test", "test2"}

	obj, err = spaceStorage.List(userCtx, &metainternalversion.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	spaceList, ok = obj.(*tenancy.SpaceList)
	if !ok {
		t.Fatalf("Expected space list, got %#+v", obj)
	} else if len(spaceList.Items) != 2 {
		t.Fatalf("Expected space list with 2 items, got %d items", len(spaceList.Items))
	}

	// Filter list by label selector
	selector, err := labels.Parse("testlabel=test")
	if err != nil {
		t.Fatal(err)
	}
	obj, err = spaceStorage.List(userCtx, &metainternalversion.ListOptions{LabelSelector: selector})
	if err != nil {
		t.Fatal(err)
	}
	spaceList, ok = obj.(*tenancy.SpaceList)
	if !ok {
		t.Fatalf("Expected space list, got %#+v", obj)
	} else if len(spaceList.Items) != 1 || spaceList.Items[0].Name != "test2" {
		t.Fatalf("Expected space list with 1 items, got %d items", len(spaceList.Items))
	}
}

func TestCreateSpace(t *testing.T) {
	spaceLimit := 2
	scheme := testingutil.NewScheme()
	fakeClient := testingutil.NewFakeClient(scheme,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
			},
		}, &configv1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: configv1alpha1.AccountSpec{
				SpaceLimit: &spaceLimit,
			},
		})
	fakeAuthCache := fakeauth.NewFakeAuthCache()
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	spaceStorage := NewSpaceStorage(fakeClient, fakeAuthCache, scheme).(*spaceStorage)

	// Try to create if we are not allowed to
	_, err := spaceStorage.Create(userCtx, &tenancy.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test3",
		},
	}, fakeCreateValidation, &metav1.CreateOptions{})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	// Allow us to create the space
	fakeAuthCache.UserAccounts["foo"] = []string{"test"}

	// Create a space with account
	_, err = spaceStorage.Create(userCtx, &tenancy.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test3",
		},
		Spec: tenancy.SpaceSpec{
			Account: "test",
		},
	}, fakeCreateValidation, &metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Expected no error but got %v", err)
	}

	// Create a space without account
	_, err = spaceStorage.Create(userCtx, &tenancy.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test4",
		},
	}, fakeCreateValidation, &metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Expected no error but got %v", err)
	}

	// Set index value
	fakeClient.SetIndexValue(corev1.SchemeGroupVersion.WithKind("Namespace"), constants.IndexByAccount, "test", []runtime.Object{
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
			},
		},
	})

	// Create a space that would exceed the limit
	_, err = spaceStorage.Create(userCtx, &tenancy.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test5",
		},
	}, fakeCreateValidation, &metav1.CreateOptions{})
	if err == nil || kerrors.IsForbidden(err) == false {
		t.Fatalf("Expected forbidden but got %v", err)
	}

	fakeAuthCache.UserAccounts["foo"] = []string{}
	fakeAuthCache.UserNamespaces["foo"] = []string{"test6"}

	// Create a space without account again
	_, err = spaceStorage.Create(userCtx, &tenancy.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test6",
		},
	}, fakeCreateValidation, &metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Expected no error but got %v", err)
	}

	fakeAuthCache.UserNamespaces["foo"] = []string{"*"}

	// Try to create an space that already exists
	_, err = spaceStorage.Create(userCtx, &tenancy.Space{
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

func TestSpaceUpdate(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := testingutil.NewFakeClient(scheme,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
			},
		})
	fakeAuthCache := fakeauth.NewFakeAuthCache()
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	spaceStorage := NewSpaceStorage(fakeClient, fakeAuthCache, scheme).(*spaceStorage)

	_, updated, err := spaceStorage.Update(userCtx, "test", &fakeUpdater{out: &tenancy.Space{
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

	// Allow namespace update
	fakeAuthCache.UserNamespaces["foo"] = []string{"*"}

	_, updated, err = spaceStorage.Update(userCtx, "test", &fakeUpdater{out: &tenancy.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "123456",
			Labels: map[string]string{
				"Updated": "true",
			},
		},
	}}, fakeCreateValidation, fakeUpdateValidation, false, &metav1.UpdateOptions{})
	if err == nil || kerrors.IsInvalid(err) == false {
		t.Fatalf("Expected invalid error, got %v", err)
	}
}

func TestSpaceDelete(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := testingutil.NewFakeClient(scheme,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
			},
		})
	fakeAuthCache := fakeauth.NewFakeAuthCache()
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	spaceStorage := NewSpaceStorage(fakeClient, fakeAuthCache, scheme).(*spaceStorage)

	_, deleted, err := spaceStorage.Delete(userCtx, "test", fakeDeleteValidation, &metav1.DeleteOptions{})
	if err == nil || kerrors.IsForbidden(err) == false || deleted == true {
		t.Fatalf("Expected forbidden error, got %v", err)
	}

	// Allow account delete
	fakeAuthCache.UserNamespaces["foo"] = []string{"test"}

	_, deleted, err = spaceStorage.Delete(userCtx, "test", fakeDeleteValidation, &metav1.DeleteOptions{})
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
