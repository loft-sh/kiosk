package helm

import (
	"context"
	"io/ioutil"
	"strings"
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
	expectedArgs  string
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
			name:      "template-name",
			namespace: "test-namespace",
			config: &configv1alpha1.HelmConfiguration{
				Values: "test",
				Chart: configv1alpha1.HelmChart{
					Repository: &configv1alpha1.HelmChartRepository{
						Name:    "test-name",
						RepoURL: "test-repourl",
						Password: &configv1alpha1.HelmSecretRef{
							Name:      "test-name",
							Namespace: "test-namespace",
							Key:       "test-password",
						},
						Username: &configv1alpha1.HelmSecretRef{
							Name:      "test-name",
							Namespace: "test-namespace",
							Key:       "test-username",
						},
					},
				},
				SetValues: []configv1alpha1.HelmSetValue{
					configv1alpha1.HelmSetValue{
						Name:  "set-value-name",
						Value: "set-value-value",
					},
				},
			},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"test-username": []byte("test-username"),
					"test-password": []byte("test-password"),
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
			},
			expectedArgs:  "template template-name test-name --repo test-repourl --namespace test-namespace --username test-username --password test-password --set set-value-name=set-value-value --values",
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

			if test.expectedArgs != "" {
				valuesFileName := args[len(args)-2]
				bs, err := ioutil.ReadFile(valuesFileName)
				if err != nil {
					t.Fatalf("Test %s: could not read values file: %s", testName, err)
				}
				if string(bs) != test.config.Values {
					t.Fatalf("Test %s: expected file content: %s, but got: %s", testName, test.config.Values, string(bs))
				}
			}

			return "", nil
		}

		if test.secret != nil {
			client.Create(context.TODO(), test.secret)
		}

		_, err := h.Template(client, test.name, test.namespace, test.config)
		if test.expectedError && err == nil {
			t.Fatalf("Test %s: expected error but got nil", testName)
		} else if !test.expectedError && err != nil {
			t.Fatalf("Test %s: expected no error but got %v", testName, err)
		}

		if test.expectedArgs != "" {
			if strings.Index(strings.Join(retArgs, " "), test.expectedArgs) == -1 {
				t.Fatalf("Test %s: expected args: %v to be in: %v", testName, test.expectedArgs, retArgs)
			}
		}
	}
}
