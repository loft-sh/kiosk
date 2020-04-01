package controllers

import (
	"context"
	"github.com/ghodss/yaml"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"testing"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"

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
	account         *configv1alpha1.Account
	ownedNamespaces []*corev1.Namespace

	expectedAccountStatus *configv1alpha1.AccountStatus
}

func TestAccountController(t *testing.T) {
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
			expectedAccountStatus: &configv1alpha1.AccountStatus{
				Namespaces: []configv1alpha1.AccountNamespaceStatus{
					configv1alpha1.AccountNamespaceStatus{
						Name: "test",
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
	}
}



type eventHandlerTestCase struct {
	name string

	event interface{}

	expectedItems []interface{}
}

func TestEventHandler(t *testing.T) {
	testCases := []eventHandlerTestCase{
		{
			name:  "Create without meta",
			event: event.CreateEvent{},
		},
		{
			name: "Update with same accounts",
			event: event.UpdateEvent{
				MetaOld: &corev1.Namespace{
					ObjectMeta: v1.ObjectMeta{
						Name: "ns1",
						Labels: map[string]string{
							constants.SpaceLabelAccount: "someOwner",
						},
					},
				},
				MetaNew: &corev1.Namespace{
					ObjectMeta: v1.ObjectMeta{
						Name: "ns1",
						Labels: map[string]string{
							constants.SpaceLabelAccount: "someOwner",
						},
					},
				},
			},
		},
		{
			name: "Update with different accounts",
			event: event.UpdateEvent{
				MetaOld: &corev1.Namespace{
					ObjectMeta: v1.ObjectMeta{
						Name: "ns1",
						Labels: map[string]string{
							constants.SpaceLabelAccount: "someOwnerOld",
						},
					},
				},
				MetaNew: &corev1.Namespace{
					ObjectMeta: v1.ObjectMeta{
						Name: "ns1",
						Labels: map[string]string{
							constants.SpaceLabelAccount: "someOwnerNew",
						},
					},
				},
			},
			expectedItems: []interface{}{
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "someOwnerOld",
					},
				},
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "someOwnerNew",
					},
				},
			},
		},
		{
			name: "Delete without annotations",
			event: event.DeleteEvent{
				Meta: &corev1.Pod{},
			},
		},
		{
			name: "Generic with annotation",
			event: event.GenericEvent{
				Meta: &corev1.Namespace{
					ObjectMeta: v1.ObjectMeta{
						Labels: map[string]string{
							constants.SpaceLabelAccount: "someOwner",
						},
					},
				},
			},
			expectedItems: []interface{}{
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "someOwner",
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		rateLimiter := &fakeRateLimiter{
			RateLimitingInterface: workqueue.NewRateLimitingQueue(nil),
		}
		testHandler := &NamespaceEventHandler{}

		switch e := testCase.event.(type) {
		case event.CreateEvent:
			testHandler.Create(e, rateLimiter)
		case event.UpdateEvent:
			testHandler.Update(e, rateLimiter)
		case event.DeleteEvent:
			testHandler.Delete(e, rateLimiter)
		case event.GenericEvent:
			testHandler.Generic(e, rateLimiter)
		default:
			t.Fatalf("Unknown type %T", e)
		}

		itemsAsYaml, err := yaml.Marshal(rateLimiter.items)
		assert.NilError(t, err, "Error parsing items in testCase %s", testCase.name)
		expectedAsYaml, err := yaml.Marshal(testCase.expectedItems)
		assert.NilError(t, err, "Error parsing expectation in testCase %s", testCase.name)
		assert.Equal(t, string(itemsAsYaml), string(expectedAsYaml), "Unexpected items in testCase %s", testCase.name)
	}
}

type fakeRateLimiter struct {
	workqueue.RateLimitingInterface
	items []interface{}
}

func (f *fakeRateLimiter) Add(item interface{}) {
	f.items = append(f.items, item)
}

