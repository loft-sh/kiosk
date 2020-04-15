package controllers

import (
	"testing"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	"github.com/kiosk-sh/kiosk/pkg/util"
	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"
	"github.com/kiosk-sh/kiosk/pkg/util/testing/ptr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"gotest.tools/assert"
)

type addManagerIndicesTestCase struct {
	name string

	key string
	in  runtime.Object

	expected []string
}

func TestAddManagerIndices(t *testing.T) {
	testCases := []addManagerIndicesTestCase{
		addManagerIndicesTestCase{
			name: "Empty AccountQuota",
			key:  constants.IndexByAccount,
			in: &configv1alpha1.AccountQuota{
				Spec: configv1alpha1.AccountQuotaSpec{},
			},
		},
		addManagerIndicesTestCase{
			name: "AccountQuota with account",
			key:  constants.IndexByAccount,
			in: &configv1alpha1.AccountQuota{
				Spec: configv1alpha1.AccountQuotaSpec{
					Account: "myAccount",
				},
			},
			expected: []string{"myAccount"},
		},
		addManagerIndicesTestCase{
			name: "Namespace without annotations",
			key:  constants.IndexByAccount,
			in:   &corev1.Namespace{},
		},
		addManagerIndicesTestCase{
			name: "Namespace without annotations",
			key:  constants.IndexByAccount,
			in: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						constants.SpaceLabelAccount: "myAccount2",
					},
				},
			},
			expected: []string{"myAccount2"},
		},
		addManagerIndicesTestCase{
			name: "Empty owner",
			key:  constants.IndexByAccount,
			in:   &rbacv1.RoleBinding{},
		},
		addManagerIndicesTestCase{
			name: "Owner with name",
			key:  constants.IndexByAccount,
			in: &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "noController",
							APIVersion: apiGVStr,
							Kind:       "Account",
							Controller: ptr.Bool(false),
						},
						{
							Name:       "myAccount3",
							APIVersion: apiGVStr,
							Kind:       "Account",
							Controller: ptr.Bool(true),
						},
					},
				},
			},
			expected: []string{"myAccount3"},
		},
		addManagerIndicesTestCase{
			name: "Template instace",
			key:  constants.IndexByTemplate,
			in: &configv1alpha1.TemplateInstance{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: configv1alpha1.TemplateInstanceSpec{
					Template: "test",
				},
			},
			expected: []string{"test"},
		},
	}

	fakeIndexer := &fakeIndexer{
		scheme: testingutil.NewScheme(),
	}

	err := AddManagerIndices(fakeIndexer)
	assert.NilError(t, err, "Unexpected error adding indices")

	for _, testCase := range testCases {
		out, err := fakeIndexer.GetIndexValues(testCase.in, testCase.key)

		assert.NilError(t, err, "Unexpected error in testCase %s", testCase.name)
		assert.Assert(t, util.StringsEqual(out, testCase.expected), "Unexpected output in testCase %s", testCase.name)
	}
}

type fakeIndexer struct {
	scheme  *runtime.Scheme
	indices map[schema.GroupVersionKind]map[string]client.IndexerFunc
}

func (fi *fakeIndexer) GetIndexValues(obj runtime.Object, field string) ([]string, error) {
	gvk, err := apiutil.GVKForObject(obj, fi.scheme)
	if err != nil {
		return nil, err
	}

	return fi.indices[gvk][field](obj), nil
}

func (fi *fakeIndexer) IndexField(obj runtime.Object, field string, extractValue client.IndexerFunc) error {
	gvk, err := apiutil.GVKForObject(obj, fi.scheme)
	if err != nil {
		return err
	}
	if fi.indices == nil {
		fi.indices = map[schema.GroupVersionKind]map[string]client.IndexerFunc{}
	}
	if fi.indices[gvk] == nil {
		fi.indices[gvk] = map[string]client.IndexerFunc{}
	}

	fi.indices[gvk][field] = extractValue
	return nil
}
