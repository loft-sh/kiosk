package secret

import (
	"context"
	"github.com/loft-sh/kiosk/pkg/util/certhelper"
	"github.com/loft-sh/kiosk/pkg/util/clienthelper"
	"github.com/pkg/errors"
	"io/ioutil"
	"k8s.io/klog"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// WebhookCertSecretName is the name of the kiosk webhook certificate
	WebhookCertSecretName = "kiosk-webhook-cert"

	// APIServiceCertSecretName is the name of the kiosk api service certificate
	APIServiceCertSecretName = "kiosk-api-service-cert"
)

func EnsureCertSecrets(ctx context.Context, client client.Client) error {
	// get current namespace
	namespace, err := clienthelper.CurrentNamespace()
	if err != nil {
		return err
	}

	// make sure the namespace exists
	err = client.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})
	if err != nil && kerrors.IsAlreadyExists(err) == false {
		return err
	}

	// ensure webhook secret
	err = ensureSecret(ctx, client, namespace, WebhookCertSecretName, certhelper.WebhookCertFolder, certhelper.GenerateWebhookCertificate)
	if err != nil {
		return errors.Wrap(err, "ensure webhook cert")
	}

	// ensure webhook secret
	err = ensureSecret(ctx, client, namespace, APIServiceCertSecretName, certhelper.APIServiceCertFolder, certhelper.GenerateAPIServiceCertificate)
	if err != nil {
		return errors.Wrap(err, "ensure api service cert")
	}

	return nil
}

func ensureSecret(ctx context.Context, client client.Client, namespace string, certSecretName string, certFolder string, createCert func() error) error {
	secret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: certSecretName}, secret)
	if err != nil {
		if kerrors.IsNotFound(err) == false {
			return err
		}

		// Fallthrough
	} else if isSecretValid(secret) {
		return getDataFromSecret(secret, certFolder)
	} else {
		err = client.Delete(ctx, secret)
		if err != nil {
			return err
		}
	}

	// create a new secret
	err = createCert()
	if err != nil {
		return err
	}

	// read the data
	data := map[string][]byte{}
	for _, file := range []string{"ca.crt", "tls.crt", "tls.key"} {
		out, err := ioutil.ReadFile(filepath.Join(certFolder, file))
		if err != nil {
			return err
		}

		data[file] = out
	}

	// create secret
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certSecretName,
			Namespace: namespace,
		},
		Data: data,
		Type: corev1.SecretTypeTLS,
	}
	err = client.Create(ctx, secret)
	if err != nil {
		// someone was faster here, this can happen if we run with leader election on
		// and another instance created the secret faster than we did, in this case
		// we just retrieve the secret again and continue
		if kerrors.IsAlreadyExists(err) {
			klog.Infof("secret %s/%s already exists, will retry to get the data from it", namespace, certSecretName)
			return ensureSecret(ctx, client, namespace, certSecretName, certFolder, createCert)
		}

		return err
	}

	return nil
}

func getDataFromSecret(secret *corev1.Secret, certFolder string) error {
	err := os.MkdirAll(certFolder, 0755)
	if err != nil {
		return err
	}

	for _, file := range []string{"ca.crt", "tls.crt", "tls.key"} {
		err := ioutil.WriteFile(filepath.Join(certFolder, file), secret.Data[file], 0666)
		if err != nil {
			return err
		}
	}

	return nil
}

func isSecretValid(secret *corev1.Secret) bool {
	if secret.Data == nil {
		return false
	} else if secret.Type != corev1.SecretTypeTLS {
		return false
	} else if secret.Data["ca.crt"] == nil || secret.Data["tls.crt"] == nil || secret.Data["tls.key"] == nil {
		return false
	}

	return true
}
