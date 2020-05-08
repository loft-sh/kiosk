package helm

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/util"
	"github.com/kiosk-sh/kiosk/pkg/util/convert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Helm defines a public interface for executing a template command
type Helm interface {
	Template(client client.Client, name, namespace string, config *configv1alpha1.HelmConfiguration) ([]*unstructured.Unstructured, error)
}

type helm struct {
	runner runFunc
}

type runFunc func(args []string) (string, error)

// NewHelmRunner creates a new helm runner
func NewHelmRunner() Helm {
	return &helm{
		runner: run,
	}
}

// Template executes a helm template command
func (h *helm) Template(client client.Client, name, namespace string, config *configv1alpha1.HelmConfiguration) ([]*unstructured.Unstructured, error) {
	releaseName := config.ReleaseName
	if releaseName == "" {
		releaseName = name
	}

	args := []string{"template", releaseName}
	if config.Chart.Repository == nil {
		return nil, fmt.Errorf("no helm repository given")
	}
	if config.Chart.Repository.Name == "" {
		return nil, fmt.Errorf("chart name cannot be empty")
	}
	if config.Chart.Repository.RepoURL == "" {
		return nil, fmt.Errorf("repository url cannot be empty")
	}

	args = append(args, config.Chart.Repository.Name)
	args = append(args, "--repo", config.Chart.Repository.RepoURL)
	args = append(args, "--namespace", namespace)
	if config.Chart.Repository.Version != "" {
		args = append(args, "--version", config.Chart.Repository.Version)
	}

	if config.Chart.Repository.Username != nil {
		username, err := readSecret(client, config.Chart.Repository.Username)
		if err != nil {
			return nil, fmt.Errorf("error reading username secret: %v", err)
		}

		args = append(args, "--username", username)
	}
	if config.Chart.Repository.Password != nil {
		password, err := readSecret(client, config.Chart.Repository.Password)
		if err != nil {
			return nil, fmt.Errorf("error reading password secret: %v", err)
		}

		args = append(args, "--password", password)
	}

	// Add set values
	for _, v := range config.SetValues {
		if v.ForceString {
			args = append(args, "--set-string", v.Name+"="+v.Value)
		} else {
			args = append(args, "--set", v.Name+"="+v.Value)
		}
	}

	// Add raw values
	if config.Values != "" {
		file, err := ioutil.TempFile("", "values-*.yaml")
		if err != nil {
			return nil, err
		}
		p := file.Name()
		defer func() { _ = os.Remove(p) }()
		err = ioutil.WriteFile(p, []byte(config.Values), 0644)
		if err != nil {
			return nil, err
		}

		args = append(args, "--values", p)
	}

	args = append(args, "--include-crds")
	out, err := h.runner(args)
	if err != nil {
		return nil, err
	}

	return convert.StringToUnstructuredArray(out)
}

func readSecret(client client.Client, secretRef *configv1alpha1.HelmSecretRef) (string, error) {
	if secretRef.Name == "" {
		return "", fmt.Errorf("secret name must be specifed")
	}
	if secretRef.Namespace == "" {
		return "", fmt.Errorf("secret namespace must be specified")
	}
	if secretRef.Key == "" {
		return "", fmt.Errorf("secret key must be specified")
	}

	secret := &corev1.Secret{}
	err := client.Get(context.Background(), types.NamespacedName{Name: secretRef.Name, Namespace: secretRef.Namespace}, secret)
	if err != nil {
		return "", err
	}
	if secret.Data == nil {
		return "", fmt.Errorf("secret %s/%s data is empty", secretRef.Namespace, secretRef.Name)
	}

	data, ok := secret.Data[secretRef.Key]
	if !ok {
		return "", fmt.Errorf("couldn't find key '%s' in secret %s/%s", secretRef.Key, secretRef.Namespace, secretRef.Name)
	}

	return string(data), nil
}

func run(args []string) (string, error) {
	return util.Output("./helm", args...)
}
