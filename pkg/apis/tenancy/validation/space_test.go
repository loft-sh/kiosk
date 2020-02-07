package validation

import (
	"testing"

	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type testSpaceValidation struct {
	in    *tenancy.Space
	inOld *tenancy.Space

	valid bool
}

func TestSpaceValidation(t *testing.T) {
	tests := map[string]*testSpaceValidation{
		"Invalid space": &testSpaceValidation{
			valid: false,
			in: &tenancy.Space{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testABC",
				},
			},
		},
		"Valid space": &testSpaceValidation{
			valid: true,
			in: &tenancy.Space{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
		},
		"Invalid space update": &testSpaceValidation{
			valid: false,
			inOld: &tenancy.Space{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						"test": "test",
					},
					Labels: map[string]string{
						"test2": "test",
					},
				},
			},
			in: &tenancy.Space{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						"test": "test",
					},
					Labels: map[string]string{
						"test": "test",
					},
				},
			},
		},
	}

	for testName, test := range tests {
		var errs field.ErrorList
		if test.inOld != nil {
			errs = ValidateSpaceUpdate(test.in, test.inOld)
		} else {
			errs = ValidateSpace(test.in)
		}

		if test.valid && len(errs) > 0 {
			t.Fatalf("Test %s: expected space valid, but got %#+v", testName, errs)
		} else if !test.valid && len(errs) == 0 {
			t.Fatalf("Test %s: expected space invalid, but got no errors", testName)
		}
	}
}
