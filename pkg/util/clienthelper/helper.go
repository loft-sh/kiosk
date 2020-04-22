package clienthelper

import (
	"context"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func OwnerOf(obj runtime.Object) ([]metav1.OwnerReference, error) {
	metaAccessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	return metaAccessor.GetOwnerReferences(), nil
}

func OwningDeployment(ctx context.Context, client client.Client) (*metav1.OwnerReference, error) {
	namespace, err := CurrentNamespace()
	if err != nil {
		return nil, err
	}

	currentPod := &corev1.Pod{}
	err = client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: CurrentPod()}, currentPod)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	ownerReferences, err := OwnerOf(currentPod)
	if err != nil {
		return nil, err
	} else if len(ownerReferences) != 1 {
		return nil, nil
	} else if ownerReferences[0].APIVersion != appsv1.SchemeGroupVersion.String() || ownerReferences[0].Kind != "ReplicaSet" {
		return nil, nil
	}

	currentReplicaSet := &appsv1.ReplicaSet{}
	err = client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ownerReferences[0].Name}, currentReplicaSet)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	ownerReferences, err = OwnerOf(currentReplicaSet)
	if err != nil {
		return nil, err
	} else if len(ownerReferences) != 1 {
		return nil, nil
	} else if ownerReferences[0].APIVersion != appsv1.SchemeGroupVersion.String() || ownerReferences[0].Kind != "Deployment" {
		return nil, nil
	}

	return &ownerReferences[0], nil
}

func CurrentPod() string {
	return os.Getenv("HOSTNAME")
}

func CurrentNamespace() (string, error) {
	namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}

	return string(namespace), nil
}

func CreateWithOwner(ctx context.Context, client client.Client, object runtime.Object, owner metav1.Object, scheme *runtime.Scheme) error {
	accessor, err := meta.Accessor(object)
	if err != nil {
		return err
	}
	typeAccessor, err := meta.TypeAccessor(object)
	if err != nil {
		return err
	}

	// Set owner controller
	err = ctrl.SetControllerReference(owner, accessor, scheme)
	if err != nil {
		return err
	}

	err = client.Create(ctx, object)
	if err != nil {
		return err
	}

	klog.Info("created " + typeAccessor.GetKind() + "  " + accessor.GetName())
	return nil
}
