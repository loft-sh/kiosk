package apiservice

import (
	"context"
	"io/ioutil"

	tenancyv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/tenancy/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/util/certhelper"
	"github.com/kiosk-sh/kiosk/pkg/util/clienthelper"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EnsureAPIService makes sure the apiservice is up and correct
func EnsureAPIService(ctx context.Context, client client.Client) error {
	service := &apiregistrationv1.APIService{}
	name := tenancyv1alpha1.SchemeGroupVersion.Version + "." + tenancyv1alpha1.SchemeGroupVersion.Group
	err := client.Get(ctx, types.NamespacedName{Name: name}, service)
	if err != nil {
		if kerrors.IsNotFound(err) == false {
			return err
		}

		service.Name = name
		err = prepareAPIService(service)
		if err != nil {
			return err
		}

		return client.Create(ctx, service)
	}

	err = prepareAPIService(service)
	if err != nil {
		return err
	}

	return client.Update(ctx, service)
}

func prepareAPIService(service *apiregistrationv1.APIService) error {
	caBundleData, err := ioutil.ReadFile(filepath.Join(certhelper.APIServiceCertFolder, "ca.crt"))
	if err != nil {
		return err
	}

	namespace, err := clienthelper.CurrentNamespace()
	if err != nil {
		return err
	}

	service.Spec = apiregistrationv1.APIServiceSpec{
		Service: &apiregistrationv1.ServiceReference{
			Namespace: namespace,
			Name:      certhelper.APIServiceName,
		},
		Group:                tenancyv1alpha1.SchemeGroupVersion.Group,
		Version:              tenancyv1alpha1.SchemeGroupVersion.Version,
		CABundle:             caBundleData,
		GroupPriorityMinimum: 10000,
		VersionPriority:      1000,
	}

	delete(service.Annotations, "cert-manager.io/inject-ca-from")
	return nil
}
