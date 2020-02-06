package helm

import (
	"context"
	"testing"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	testingutil "github.com/kiosk-sh/kiosk/pkg/util/testing"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type templateTestCase struct {
	name          string
	namespace     string
	config        *configv1alpha1.HelmConfiguration
	secret        *corev1.Secret
	expectedError bool
	expectedArgs  *[]string
}

func TestTemplate(t *testing.T) {
	tests := map[string]*templateTestCase{
		"Repository is nil": &templateTestCase{
			name:      "test",
			namespace: "test",
			config: &configv1alpha1.HelmConfiguration{
				Chart: configv1alpha1.HelmChart{
					Repository: nil,
				},
			},
			expectedError: true,
		},
		"Repository name is empty": &templateTestCase{
			name:      "test",
			namespace: "test",
			config: &configv1alpha1.HelmConfiguration{
				Chart: configv1alpha1.HelmChart{
					Repository: &configv1alpha1.HelmChartRepository{
						Name: "",
					},
				},
			},
			expectedError: true,
		},
		"Repository repoURL is empty": &templateTestCase{
			name:      "test",
			namespace: "test",
			config: &configv1alpha1.HelmConfiguration{
				Chart: configv1alpha1.HelmChart{
					Repository: &configv1alpha1.HelmChartRepository{
						Name:    "test",
						RepoURL: "",
					},
				},
			},
			expectedError: true,
		},
		"ReadSecret name is empty": &templateTestCase{
			name:      "test",
			namespace: "test",
			config: &configv1alpha1.HelmConfiguration{
				Chart: configv1alpha1.HelmChart{
					Repository: &configv1alpha1.HelmChartRepository{
						Name:     "test",
						RepoURL:  "test",
						Username: &configv1alpha1.HelmSecretRef{},
					},
				},
			},
			expectedError: true,
		},
		"ReadSecret goes through": &templateTestCase{
			name:      "test",
			namespace: "test",
			config: &configv1alpha1.HelmConfiguration{
				Chart: configv1alpha1.HelmChart{
					Repository: &configv1alpha1.HelmChartRepository{
						Name:    "test",
						RepoURL: "test",
						Password: &configv1alpha1.HelmSecretRef{
							Name:      "test",
							Namespace: "test",
							Key:       "test",
						},
						Username: &configv1alpha1.HelmSecretRef{
							Name:      "test",
							Namespace: "test",
							Key:       "test",
						},
					},
				},
			},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"test": []byte{},
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			},
			expectedError: false,
		},
		"Add raw values": &templateTestCase{
			name:      "test",
			namespace: "test",
			config: &configv1alpha1.HelmConfiguration{
				Values: "test",
				Chart: configv1alpha1.HelmChart{
					Repository: &configv1alpha1.HelmChartRepository{
						Name:    "test-name",
						RepoURL: "test",
						Password: &configv1alpha1.HelmSecretRef{
							Name:      "test",
							Namespace: "test",
							Key:       "test",
						},
						Username: &configv1alpha1.HelmSecretRef{
							Name:      "test",
							Namespace: "test",
							Key:       "test",
						},
					},
				},
			},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"test": []byte{},
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			},
			expectedArgs:  &[]string{"--repo", "--namespace", "test-name"},
			expectedError: false,
		},
	}

	scheme := testingutil.NewScheme()
	for testName, test := range tests {
		client := testingutil.NewFakeClient(scheme)
		retArgs := []string{}

		h := NewHelmRunner().(*helm)
		h.runner = func(args []string) (string, error) {
			retArgs = args
			return "", nil
		}

		if test.secret != nil {
			client.Create(context.TODO(), test.secret)
		}

		_, err := h.Template(client, test.name, test.namespace, test.config)
		if test.expectedError && err == nil {
			t.Fatalf("Test %s: expected error but got nil", testName)
		} else if !test.expectedError && err != nil {
			t.Fatalf("Test %s: expexted no error but got %v", testName, err)
		}

		if test.expectedArgs != nil {
			for _, e := range *test.expectedArgs {
				if !contains(retArgs, e) {
					t.Fatalf("Test %s: expected args: %v to be in: %v", testName, test.expectedArgs, retArgs)
				}
			}
		}
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
