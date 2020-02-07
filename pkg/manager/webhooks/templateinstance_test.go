package webhooks

import (
	"testing"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/util/convert"
	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"
	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type handleTemplateInstanceTestCase struct {
	operation           v1beta1.Operation
	templateInstance    *configv1alpha1.TemplateInstance
	templateInstanceOld *configv1alpha1.TemplateInstance

	isDenied bool
}

func TestHandleTemplateInstance(t *testing.T) {
	tests := map[string]*handleTemplateInstanceTestCase{
		"Is no update": &handleTemplateInstanceTestCase{
			operation: "TEST",
		},
		"Is update": &handleTemplateInstanceTestCase{
			templateInstance: &configv1alpha1.TemplateInstance{
				ObjectMeta: v1.ObjectMeta{Name: "test"},
				Spec:       configv1alpha1.TemplateInstanceSpec{Template: "test"},
			},
			templateInstanceOld: &configv1alpha1.TemplateInstance{
				ObjectMeta: v1.ObjectMeta{Name: "test"},
				Spec:       configv1alpha1.TemplateInstanceSpec{Template: "test"},
			},
			operation: "UPDATE",
			isDenied:  false,
		},
		"newQuota is different than oldQuota": &handleTemplateInstanceTestCase{
			templateInstance: &configv1alpha1.TemplateInstance{
				ObjectMeta: v1.ObjectMeta{Name: "test2"},
				Spec:       configv1alpha1.TemplateInstanceSpec{Template: "test2"},
			},
			templateInstanceOld: &configv1alpha1.TemplateInstance{
				ObjectMeta: v1.ObjectMeta{Name: "test"},
				Spec:       configv1alpha1.TemplateInstanceSpec{Template: "test"},
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
		ti := &TemplateInstanceValidator{
			Scheme: scheme,
		}
		ti.InjectDecoder(decoder)

		req := admission.Request{
			v1beta1.AdmissionRequest{
				Operation: test.operation,
			},
		}

		if test.templateInstance != nil {
			setType(test.templateInstance)
			bs, err := convert.RuntimeObjectToBytes(test.templateInstance)
			if err != nil {
				t.Fatalf("Test %s: %s", testName, err)
			}
			req.AdmissionRequest.Object.Raw = bs
		}

		if test.templateInstanceOld != nil {
			setType(test.templateInstanceOld)
			bs, err := convert.RuntimeObjectToBytes(test.templateInstanceOld)
			if err != nil {
				t.Fatalf("Test %s: %s", testName, err)
			}
			req.OldObject.Raw = bs
		}

		resp := ti.Handle(nil, req)
		if !resp.Allowed && !test.isDenied {
			t.Fatalf("Test %s: got error but did not expect it", testName)
		}
		if resp.Allowed && test.isDenied {
			t.Fatalf("Test %s: got no error but expected it", testName)
		}
	}
}
