package parameters

import (
	"errors"
	"github.com/loft-sh/kiosk/pkg/util/testing/ptr"
	"reflect"
	"testing"
)

type testCase struct {
	input   string
	replace ReplaceVarFn
	output  interface{}
	err     *string
}

func TestParse(t *testing.T) {
	testCases := map[string]*testCase{
		"Single Replace": &testCase{
			input:   " test abc ${Test} ",
			replace: func(value string) (string, error) { return "test", nil },
			output:  " test abc test ",
		},
		"Single Replace 2": &testCase{
			input:   "${Test}",
			replace: func(value string) (string, error) { return "${Test}", nil },
			output:  "${Test}",
		},
		"Var Name 2": &testCase{
			input: " test abc ${Test} ",
			replace: func(value string) (string, error) {
				if value != "Test" {
					return "", errors.New("unexpected var name")
				}
				return "test", nil
			},
			output: " test abc test ",
		},
		"Var Name": &testCase{
			input: " test abc ${Test} ",
			replace: func(value string) (string, error) {
				if value != "Test" {
					return "", errors.New("unexpected var name")
				}
				return "test", nil
			},
			output: " test abc test ",
		},
		"Single Escape": &testCase{
			input:   " test abc $${Test} ",
			replace: func(value string) (string, error) { return "", errors.New("Shouldn't match at all") },
			output:  " test abc ${Test} ",
		},
		"Single Escape 2": &testCase{
			input:   " test abc $$${Test} ",
			replace: func(value string) (string, error) { return "", errors.New("Shouldn't match at all") },
			output:  " test abc $${Test} ",
		},
		"Multiple Replace": &testCase{
			input:   " test ${ABC}${Test} abc $${Test}${Test} ",
			replace: func(value string) (string, error) { return "test", nil },
			output:  " test testtest abc ${Test}test ",
		},
		"Multiple Replace 2": &testCase{
			input:   "${Test}${Test}${Test}",
			replace: func(value string) (string, error) { return value, nil },
			output:  "TestTestTest",
		},
		"Return integer": &testCase{
			input:   "${integer}",
			replace: func(value string) (string, error) { return "1", nil },
			output:  "1",
		},
		"Return bool": &testCase{
			input:   "${bool}",
			replace: func(value string) (string, error) { return "true", nil },
			output:  "true",
		},
		"Return error": &testCase{
			input:   "${bool}",
			replace: func(value string) (string, error) { return "", errors.New("Test Error") },
			err:     ptr.String("Test Error"),
		},
		"No match": &testCase{
			input:   "Test",
			replace: func(value string) (string, error) { return "", errors.New("Test Error") },
			output:  "Test",
		},
		"Integer": &testCase{
			input:   "${{Test}}",
			replace: func(value string) (string, error) { return "1", nil },
			output:  float64(1),
		},
		"Bool": &testCase{
			input:   "${{Test}}",
			replace: func(value string) (string, error) { return "false", nil },
			output:  false,
		},
	}

	// Run test cases
	for key, value := range testCases {
		out, err := ParseString(value.input, value.replace)
		if err != nil {
			if value.err != nil {
				if *value.err != err.Error() {
					t.Fatalf("Test %s failed: unexpected error message: expected %s - got %s", key, *value.err, err.Error())
				}
			} else {
				t.Fatalf("Test %s failed: %v", key, err)
			}
		} else if out != value.output {
			t.Fatalf("Test %s failed: expected %v (%v) - got %v (%v)", key, value.output, reflect.TypeOf(value.output), out, reflect.TypeOf(out))
		}
	}
}
