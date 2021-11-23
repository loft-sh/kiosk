package space

import (
	tenancyv1alpha1 "github.com/loft-sh/kiosk/pkg/apis/tenancy/v1alpha1"
	"github.com/loft-sh/kiosk/pkg/authorization"
	"github.com/loft-sh/kiosk/pkg/authorization/rbac"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	"github.com/loft-sh/kiosk/pkg/apis/tenancy"
	"github.com/loft-sh/kiosk/pkg/constants"
	testingutil "github.com/loft-sh/kiosk/pkg/util/testing"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime"

	configv1alpha1 "github.com/loft-sh/kiosk/pkg/apis/config/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"

	"context"
)

var clusterAdminBinding = &rbacv1.ClusterRoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test",
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

func NewTestSpaceREST(fakeClient client.Client, scheme *runtime.Scheme) *spaceStorage {
	a := &rbac.RBACAuthorizer{AuthorizationRuleResolver: &rbac.DefaultRuleResolver{
		ListAll: true,
		Client: fakeClient,
	}}
	return &spaceStorage{
		client:     fakeClient,
		authorizer: a,
		scheme:     scheme,
		filter:     authorization.NewFilteredLister(fakeClient, a),
	}
}

func clientWithDefaultRoles(scheme *runtime.Scheme, objs ...runtime.Object) *testingutil.FakeIndexClient {
	objs = append(objs, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:           []string{"*"},
				APIGroups:       []string{"*"},
				Resources:       []string{"*"},
				NonResourceURLs: []string{"*"},
			},
		},
		AggregationRule: nil,
	})

	return testingutil.NewFakeClient(scheme, objs...)
}

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
	fakeClient := clientWithDefaultRoles(scheme, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	})
	ctx := context.TODO()
	userCtx := request.WithUser(ctx, &user.DefaultInfo{Name: "foo"})
	spaceStorage := NewTestSpaceREST(fakeClient, scheme)

	// We are not allowed to retrieve it so this should return a not found
	_, err := spaceStorage.Get(withRequestInfo(userCtx, "get", "test"), "test", &metav1.GetOptions{})
	if err == nil || kerrors.IsNotFound(err) == false {
		t.Fatalf("Expected not found error, got %v", err)
	}

	// make user cluster admin
	err = fakeClient.Create(context.TODO(), clusterAdminBinding.DeepCopy())
	if err != nil {
		t.Fatal(err)
	}

	// We are not allowed to retrieve it so this should return a not found
	test, err := spaceStorage.Get(withRequestInfo(userCtx, "get", "test"), "test", &metav1.GetOptions{})
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
	ctx := context.TODO()
	userCtx := withRequestInfo(request.WithUser(ctx, &user.DefaultInfo{Name: "foo"}), "list", "")
	spaceStorage := NewTestSpaceREST(fakeClient, scheme)

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

	// create role for 2 spaces
	err = fakeClient.Create(context.TODO(), &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:           []string{"*"},
				APIGroups:       []string{"*"},
				Resources:       []string{"*"},
				ResourceNames:   []string{"test", "test2"},
				NonResourceURLs: []string{"*"},
			},
		},
		AggregationRule: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = fakeClient.Create(context.TODO(), clusterAdminBinding.DeepCopy())
	if err != nil {
		t.Fatal(err)
	}

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
	fakeClient := clientWithDefaultRoles(scheme,
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
				Space: configv1alpha1.AccountSpace{
					Limit: &spaceLimit,
					SpaceTemplate: configv1alpha1.AccountSpaceTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"Test": "Test",
							},
						},
					},
				},
			},
		})
	ctx := context.TODO()
	userCtx := withRequestInfo(request.WithUser(ctx, &user.DefaultInfo{Name: "foo"}), "create", "")
	spaceStorage := NewTestSpaceREST(fakeClient, scheme)

	// Try to create if we are not allowed to
	_, err := spaceStorage.Create(userCtx, &tenancy.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test3",
		},
	}, fakeCreateValidation, &metav1.CreateOptions{})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	// Get old account
	oldAccount := &configv1alpha1.Account{}
	err = fakeClient.Get(ctx, client.ObjectKey{Name: "test"}, oldAccount)
	if err != nil {
		t.Fatal(err)
	}
	
	// Set index value
	test := "test"
	newAccount := &configv1alpha1.Account{
		ObjectMeta: oldAccount.ObjectMeta,
		Spec: configv1alpha1.AccountSpec{
			Subjects: []rbacv1.Subject{
				{
					Kind:     "User",
					APIGroup: rbacv1.GroupName,
					Name:     "foo",
				},
			},
			Space: configv1alpha1.AccountSpace{
				Limit: &spaceLimit,
				ClusterRole: &test,
				SpaceTemplate: configv1alpha1.AccountSpaceTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"Test": "Test",
						},
					},
				},
			},
		},
	}
	fakeClient.SetIndexValue(configv1alpha1.SchemeGroupVersion.WithKind("Account"), constants.IndexBySubjects, "user:foo", []runtime.Object{
		newAccount,
	})
	err = fakeClient.Update(context.TODO(), newAccount)
	if err != nil {
		t.Fatal(err)
	}

	// Create a space with account
	createdObj, err := spaceStorage.Create(userCtx, &tenancy.Space{
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

	createdSpace, ok := createdObj.(*tenancy.Space)
	if !ok {
		t.Fatalf("Expected space, but got: %#+v", createdObj)
	}
	if createdSpace.Annotations["Test"] != "Test" {
		t.Fatalf("Annotations were not set correctly during space init")
	}

	// Create a space without account
	_, err = spaceStorage.Create(userCtx, &tenancy.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test4",
		},
	}, fakeCreateValidation, &metav1.CreateOptions{})
	if err == nil {
		t.Fatalf("Expected error but got no error")
	}
}

func TestSpaceUpdate(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := clientWithDefaultRoles(scheme,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
			},
		})
	ctx := context.TODO()
	userCtx := withRequestInfo(request.WithUser(ctx, &user.DefaultInfo{Name: "foo"}), "update", "test")
	spaceStorage := NewTestSpaceREST(fakeClient, scheme)

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
	err = fakeClient.Create(context.TODO(), clusterAdminBinding.DeepCopy())
	if err != nil {
		t.Fatal(err)
	}

	_, updated, err = spaceStorage.Update(userCtx, "test", &fakeUpdater{out: &tenancy.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"Updated": "true",
			},
		},
	}}, fakeCreateValidation, fakeUpdateValidation, false, &metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestSpaceDelete(t *testing.T) {
	scheme := testingutil.NewScheme()
	fakeClient := clientWithDefaultRoles(scheme,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
			},
		})
	ctx := context.TODO()
	userCtx := withRequestInfo(request.WithUser(ctx, &user.DefaultInfo{Name: "foo"}), "delete", "test")
	spaceStorage := NewTestSpaceREST(fakeClient, scheme)

	_, deleted, err := spaceStorage.Delete(userCtx, "test", fakeDeleteValidation, &metav1.DeleteOptions{})
	if err == nil || kerrors.IsForbidden(err) == false || deleted == true {
		t.Fatalf("Expected forbidden error, got %v", err)
	}

	// Allow account delete
	err = fakeClient.Create(context.TODO(), clusterAdminBinding.DeepCopy())
	if err != nil {
		t.Fatal(err)
	}

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

func withRequestInfo(ctx context.Context, verb string, name string) context.Context {
	return request.WithRequestInfo(ctx, &request.RequestInfo{
		IsResourceRequest: true,
		Path:              "/apis/" + tenancy.SchemeGroupVersion.Group + "/" + tenancyv1alpha1.SchemeGroupVersion.Version,
		Verb:              verb,
		APIPrefix:         "",
		APIGroup:          tenancyv1alpha1.SchemeGroupVersion.Group,
		APIVersion:        tenancy.SchemeGroupVersion.Version,
		Namespace:         "",
		Resource:          "spaces",
		Subresource:       "",
		Name:              name,
	})
}
