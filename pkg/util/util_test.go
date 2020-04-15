package util

import (
	"github.com/kiosk-sh/kiosk/pkg/constants"
	"testing"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type runTestCase struct {
	name string

	command string
	args    []string

	expectedErr bool
}

func TestRun(t *testing.T) {
	testCases := []runTestCase{
		{
			name:        "Error in command",
			expectedErr: true,
		},
		{
			name:    "Successful command",
			command: "echo",
			args:    []string{"hello"},
		},
	}

	for _, testCase := range testCases {
		err := Run(testCase.command, testCase.args...)

		if testCase.expectedErr == false {
			assert.NilError(t, err, "Unexpected error in testCase %s", testCase.name)
		} else if err == nil {
			t.Fatalf("No error in testCase %s", testCase.name)
		}
	}
}

type outputTestCase struct {
	name string

	command string
	args    []string

	expectedErr    bool
	expectedOutput string
}

func TestOutput(t *testing.T) {
	testCases := []outputTestCase{
		{
			name:        "Error in command",
			expectedErr: true,
		},
		{
			name:           "Successful command",
			command:        "echo",
			args:           []string{"hello"},
			expectedOutput: "hello\n",
		},
	}

	for _, testCase := range testCases {
		output, err := Output(testCase.command, testCase.args...)

		if testCase.expectedErr == false {
			assert.NilError(t, err, "Unexpected error in testCase %s", testCase.name)
		} else if err == nil {
			t.Fatalf("No error in testCase %s", testCase.name)
		}

		assert.Equal(t, output, testCase.expectedOutput, "Unexpected output in testCase %s", testCase.name)
	}
}

type getAccountFromNamespaceTestCase struct {
	name string

	labels map[string]string

	expectedAccount string
}

func TestGetAccountFromNamespace(t *testing.T) {
	testCases := []getAccountFromNamespaceTestCase{
		{
			name: "No annotations",
		},
		{
			name: "Get account",
			labels: map[string]string{
				constants.SpaceLabelAccount: "myAccount",
			},
			expectedAccount: "myAccount",
		},
	}

	for _, testCase := range testCases {
		account := GetAccountFromNamespace(&corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Labels: testCase.labels,
			},
		})
		assert.Equal(t, account, testCase.expectedAccount, "Unexpected account in testCase %s", testCase.name)
	}
}

type isNamespaceInitializingTestCase struct {
	name string

	annotations map[string]string

	expected bool
}

func TestIsNamespaceInitializing(t *testing.T) {
	testCases := []isNamespaceInitializingTestCase{
		{
			name: "No annotations",
		},
		{
			name: "It is initializing",
			annotations: map[string]string{
				constants.SpaceAnnotationInitializing: "true",
			},
			expected: true,
		},
	}

	for _, testCase := range testCases {
		result := IsNamespaceInitializing(&corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Annotations: testCase.annotations,
			},
		})
		assert.Equal(t, result, testCase.expected, "Unexpected result in testCase %s", testCase.name)
	}
}

type stringsEqualTestCase struct {
	name string

	arr1 []string
	arr2 []string

	expected bool
}

func TestStringsEqual(t *testing.T) {
	testCases := []stringsEqualTestCase{
		{
			name:     "Unequal length",
			arr1:     []string{""},
			expected: false,
		},
		{
			name:     "Unequal content",
			arr1:     []string{"inBoth", "arr1Exclusive"},
			arr2:     []string{"arr2Exclusive", "inBoth"},
			expected: false,
		},
		{
			name:     "Unequal number of insances",
			arr1:     []string{"OnceInArr1", "TwiceInArr1", "TwiceInArr1"},
			arr2:     []string{"OnceInArr1", "OnceInArr1", "TwiceInArr1"},
			expected: false,
		},
		{
			name:     "Equal but wrong order",
			arr1:     []string{"a", "a", "b", "c"},
			arr2:     []string{"c", "a", "b", "a"},
			expected: true,
		},
	}

	for _, testCase := range testCases {
		result := StringsEqual(testCase.arr1, testCase.arr2)
		assert.Equal(t, result, testCase.expected, "Unexpected result in testCase %s", testCase.name)
	}
}
