package certhelper

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"github.com/kiosk-sh/kiosk/pkg/util/clienthelper"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	// WebhookCertFolder is the folder where the cert for the webhook is stored
	WebhookCertFolder = "/tmp/k8s-webhook-server/serving-certs"
	// WebhookServiceName is the name of the webhook service
	WebhookServiceName = "kiosk"

	// APIServiceCertFolder is the folder where the cert for the api service is stored
	APIServiceCertFolder = "/tmp/k8s-apiserver/serving-certs"
	// APIServiceName is the name of the service for the api service
	APIServiceName = "kiosk-apiservice"
)

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

func writeCert(folder string, cert []byte, key interface{}) error {
	out := &bytes.Buffer{}
	err := pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: cert})
	if err != nil {
		return err
	}

	err = os.MkdirAll(folder, 0755)
	if err != nil {
		return err
	}

	// tls.crt
	err = ioutil.WriteFile(filepath.Join(folder, "tls.crt"), out.Bytes(), 066)
	if err != nil {
		return err
	}

	// ca.crt
	err = ioutil.WriteFile(filepath.Join(folder, "ca.crt"), out.Bytes(), 066)
	if err != nil {
		return err
	}

	out.Reset()

	err = pem.Encode(out, pemBlockForKey(key))
	if err != nil {
		return err
	}

	// tls.key
	err = ioutil.WriteFile(filepath.Join(folder, "tls.key"), out.Bytes(), 066)
	if err != nil {
		return err
	}

	return nil
}

func generateCertificate(folder string, service string) error {
	// check if already exists
	_, err := os.Stat(filepath.Join(folder, "tls.key"))
	if err == nil {
		return nil
	} else if os.IsNotExist(err) == false {
		return err
	}

	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	namespace, err := clienthelper.CurrentNamespace()
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"kiosk"},
		},
		DNSNames: []string{
			fmt.Sprintf("%s.%s.svc", service, namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", service, namespace),
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		SubjectKeyId:          []byte{1, 2, 3, 4, 6},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	cert, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	return writeCert(folder, cert, priv)
}

func WriteCertificates() error {
	err := generateCertificate(WebhookCertFolder, WebhookServiceName)
	if err != nil {
		return err
	}

	return generateCertificate(APIServiceCertFolder, APIServiceName)
}
