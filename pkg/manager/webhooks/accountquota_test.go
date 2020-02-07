package webhooks

import (
	"testing"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/util/convert"
	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type handleTestCase struct {
	operation       v1beta1.Operation
	accountQuota    *configv1alpha1.AccountQuota
	accountQuotaOld *configv1alpha1.AccountQuota

	isDenied bool
}

func setType(obj runtime.Object) runtime.Object {
	scheme := testingutil.NewScheme()
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		panic(err)
	}

	// Set the type correctly because we are to lazy to set it in the test
	accessor, err := meta.TypeAccessor(obj)
	if err != nil {
		panic(err)
	}
	accessor.SetAPIVersion(gvk.GroupVersion().String())
	accessor.SetKind(gvk.Kind)

	return obj
}

func TestHandle(t *testing.T) {
	tests := map[string]*handleTestCase{
		"Is no update": &handleTestCase{
			operation: "TEST",
		},
		"Is update": &handleTestCase{
			accountQuota: &configv1alpha1.AccountQuota{
				ObjectMeta: v1.ObjectMeta{Name: "test"},
				Spec:       configv1alpha1.AccountQuotaSpec{Account: "test"},
			},
			accountQuotaOld: &configv1alpha1.AccountQuota{
				ObjectMeta: v1.ObjectMeta{Name: "test"},
				Spec:       configv1alpha1.AccountQuotaSpec{Account: "test"},
			},
			operation: "UPDATE",
			isDenied:  false,
		},
		"newQuota is different than oldQuota": &handleTestCase{
			accountQuota: &configv1alpha1.AccountQuota{
				ObjectMeta: v1.ObjectMeta{Name: "test2"},
				Spec:       configv1alpha1.AccountQuotaSpec{Account: "test2"},
			},
			accountQuotaOld: &configv1alpha1.AccountQuota{
				ObjectMeta: v1.ObjectMeta{Name: "test"},
				Spec:       configv1alpha1.AccountQuotaSpec{Account: "test"},
			},
			operation: "UPDATE",
			isDenied:  true,
		},
	}

	scheme := testingutil.NewScheme()
	decoder, err := admission.NewDecoder(scheme)
	if err != nil {
		t.Fatal(err)
	}

	for testName, test := range tests {
		qv := &AccountQuotaValidator{
			Scheme: scheme,
		}
		qv.InjectDecoder(decoder)

		req := admission.Request{
			v1beta1.AdmissionRequest{
				Operation: test.operation,
			},
		}

		if test.accountQuota != nil {
			setType(test.accountQuota)
			bs, err := convert.RuntimeObjectToBytes(test.accountQuota)
			if err != nil {
				t.Fatalf("Test %s: %s", testName, err)
			}
			req.AdmissionRequest.Object.Raw = bs
		}

		if test.accountQuotaOld != nil {
			setType(test.accountQuotaOld)
			bs, err := convert.RuntimeObjectToBytes(test.accountQuotaOld)
			if err != nil {
				t.Fatalf("Test %s: %s", testName, err)
			}
			req.OldObject.Raw = bs
		}

		resp := qv.Handle(nil, req)
		if !resp.Allowed && !test.isDenied {
			t.Fatalf("Test %s: got error but did not expect it", testName)
		}
		if resp.Allowed && test.isDenied {
			t.Fatalf("Test %s: got no error but expected it", testName)
		}
	}
}
