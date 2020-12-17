package testing

import (
	"context"
	"fmt"
	"strings"
	"sync"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// NewScheme creates a new scheme
func NewScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = configv1alpha1.AddToScheme(scheme)
	_ = tenancy.AddToScheme(scheme)
	return scheme
}

// NewFakeClient creates a new fake client for the standard schema
func NewFakeClient(scheme *runtime.Scheme, objs ...runtime.Object) *FakeIndexClient {
	return &FakeIndexClient{
		Client:  fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build(),
		scheme:  scheme,
		indexes: map[schema.GroupVersionKind]map[string]map[string][]runtime.Object{},
	}
}

// NewFakeCache creates a new fake cache
func NewFakeCache(scheme *runtime.Scheme) *FakeInformers {
	return &FakeInformers{
		Scheme: scheme,
	}
}

// NewFakeMapper creates a new fake mapper
func NewFakeMapper(scheme *runtime.Scheme) meta.RESTMapper {
	return meta.NewDefaultRESTMapper(scheme.PreferredVersionAllGroups())
}

type FakeIndexClient struct {
	client.Client

	clientLock sync.Mutex
	scheme     *runtime.Scheme
	indexes    map[schema.GroupVersionKind]map[string]map[string][]runtime.Object
}

func (fc *FakeIndexClient) SetIndexValue(gvk schema.GroupVersionKind, index string, value string, objs []runtime.Object) {
	fc.clientLock.Lock()
	defer fc.clientLock.Unlock()
	if fc.indexes[gvk] == nil {
		fc.indexes[gvk] = map[string]map[string][]runtime.Object{}
	}
	if fc.indexes[gvk][index] == nil {
		fc.indexes[gvk][index] = map[string][]runtime.Object{}
	}

	fc.indexes[gvk][index][value] = objs
}

func (fc *FakeIndexClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	fc.clientLock.Lock()
	defer fc.clientLock.Unlock()

	return fc.Client.Create(ctx, obj, opts...)
}

func (fc *FakeIndexClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	fc.clientLock.Lock()
	defer fc.clientLock.Unlock()

	return fc.Client.Delete(ctx, obj, opts...)
}

func (fc *FakeIndexClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	fc.clientLock.Lock()
	defer fc.clientLock.Unlock()

	return fc.Client.Update(ctx, obj, opts...)
}

func (fc *FakeIndexClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	fc.clientLock.Lock()
	defer fc.clientLock.Unlock()

	return fc.Client.Patch(ctx, obj, patch, opts...)
}

func (fc *FakeIndexClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	fc.clientLock.Lock()
	defer fc.clientLock.Unlock()

	return fc.Client.DeleteAllOf(ctx, obj, opts...)
}

func (fc *FakeIndexClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	fc.clientLock.Lock()
	defer fc.clientLock.Unlock()

	return fc.Client.Get(ctx, key, obj)
}

func (fc *FakeIndexClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	fc.clientLock.Lock()
	defer fc.clientLock.Unlock()

	gvk, err := apiutil.GVKForObject(list, fc.scheme)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(gvk.Kind, "List") {
		return fmt.Errorf("non-list type %T (kind %q) passed as output", list, gvk)
	}

	// we need the non-list GVK, so chop off the "List" from the end of the kind
	gvk.Kind = gvk.Kind[:len(gvk.Kind)-4]

	// Check if we want to list by an index
	for _, opt := range opts {
		matchingFields, ok := opt.(client.MatchingFields)
		if !ok {
			continue
		}

		// Check if we have a value for that
		// TODO: Improve that it works for multiple matching fields
		for k, v := range matchingFields {
			if fc.indexes[gvk] == nil {
				return nil
			}
			if fc.indexes[gvk][k] == nil {
				return nil
			}
			if fc.indexes[gvk][k][v] == nil {
				return nil
			}
			err := meta.SetList(list, fc.indexes[gvk][k][v])
			if err != nil {
				return err
			}

			return nil
		}
	}

	return fc.Client.List(ctx, list, opts...)
}
