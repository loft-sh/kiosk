package crd

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Builder is the interface to build crds from types
type Builder interface {
	CreateCRDs(ctx context.Context, types ...*TypeDefinition) (map[*TypeDefinition]*apiextensionsv1beta1.CustomResourceDefinition, error)
}

// NewBuilder creates a new crd builder
func NewBuilder(client client.Client) Builder {
	return &builder{
		client: client,
	}
}

type builder struct {
	client client.Client
}

func (b *builder) CreateCRDs(ctx context.Context, types ...*TypeDefinition) (map[*TypeDefinition]*apiextensionsv1beta1.CustomResourceDefinition, error) {
	typesStatus := map[*TypeDefinition]*apiextensionsv1beta1.CustomResourceDefinition{}

	ready, err := GetReadyCRDs(ctx, b.client)
	if err != nil {
		return nil, err
	}

	for _, t := range types {
		crd, err := b.createCRD(ctx, t, ready)
		if err != nil {
			return nil, err
		}
		typesStatus[t] = crd
	}

	ready, err = GetReadyCRDs(ctx, b.client)
	if err != nil {
		return nil, err
	}

	for t, crd := range typesStatus {
		if readyCrd, ok := ready[crd.Name]; ok {
			typesStatus[t] = readyCrd
		} else {
			if err := b.waitCRD(ctx, crd.Name, t, typesStatus); err != nil {
				return nil, err
			}
		}
	}

	return typesStatus, nil
}

func (b *builder) waitCRD(ctx context.Context, crdName string, t *TypeDefinition, typesStatus map[*TypeDefinition]*apiextensionsv1beta1.CustomResourceDefinition) error {
	klog.Infof("Waiting for CRD %s to become available", crdName)
	defer klog.Infof("Done waiting for CRD %s to become available", crdName)

	first := true
	return wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
		if !first {
			logrus.Infof("Waiting for CRD %s to become available", crdName)
		}
		first = false

		crd := &apiextensionsv1beta1.CustomResourceDefinition{}
		err := b.client.Get(ctx, types.NamespacedName{Name: crdName}, crd)
		if err != nil {
			return false, err
		}

		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1beta1.Established:
				if cond.Status == apiextensionsv1beta1.ConditionTrue {
					typesStatus[t] = crd
					return true, err
				}
			case apiextensionsv1beta1.NamesAccepted:
				if cond.Status == apiextensionsv1beta1.ConditionFalse {
					klog.Infof("Name conflict on %s: %v\n", crdName, cond.Reason)
				}
			}
		}

		return false, ctx.Err()
	})
}

func (b *builder) createCRD(ctx context.Context, t *TypeDefinition, ready map[string]*apiextensionsv1beta1.CustomResourceDefinition) (*apiextensionsv1beta1.CustomResourceDefinition, error) {
	name := strings.ToLower(t.Plural + "." + t.GVK.Group)
	crd, ok := ready[name]
	if ok {
		return crd, nil
	}

	crd = &apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   t.GVK.Group,
			Version: t.GVK.Version,
			Scope:   t.Scope,
			Subresources: &apiextensionsv1beta1.CustomResourceSubresources{
				Status: &apiextensionsv1beta1.CustomResourceSubresourceStatus{},
			},
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural:   t.Plural,
				Singular: t.Singular,
				Kind:     t.GVK.Kind,
			},
		},
	}

	klog.Infof("Creating CRD %s", name)
	err := b.client.Create(ctx, crd)
	if kerrors.IsAlreadyExists(err) {
		return crd, nil
	}

	return crd, err
}

func GetReadyCRDs(ctx context.Context, client client.Client) (map[string]*apiextensionsv1beta1.CustomResourceDefinition, error) {
	list := &apiextensionsv1beta1.CustomResourceDefinitionList{}

	// List existing custom resource definitions
	err := client.List(ctx, list)
	if err != nil {
		return nil, err
	}

	result := map[string]*apiextensionsv1beta1.CustomResourceDefinition{}
	for i, crd := range list.Items {
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1beta1.Established:
				if cond.Status == apiextensionsv1beta1.ConditionTrue {
					result[crd.Name] = &list.Items[i]
				}
			}
		}
	}

	return result, nil
}
