package events

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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
						Annotations: map[string]string{
							tenancy.SpaceAnnotationAccount: "someOwner",
						},
					},
				},
				MetaNew: &corev1.Namespace{
					ObjectMeta: v1.ObjectMeta{
						Name: "ns1",
						Annotations: map[string]string{
							tenancy.SpaceAnnotationAccount: "someOwner",
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
						Annotations: map[string]string{
							tenancy.SpaceAnnotationAccount: "someOwnerOld",
						},
					},
				},
				MetaNew: &corev1.Namespace{
					ObjectMeta: v1.ObjectMeta{
						Name: "ns1",
						Annotations: map[string]string{
							tenancy.SpaceAnnotationAccount: "someOwnerNew",
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
						Annotations: map[string]string{
							tenancy.SpaceAnnotationAccount: "someOwner",
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
