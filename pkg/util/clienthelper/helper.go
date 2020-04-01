package clienthelper

import (
	"context"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

	klog.V(3).Info("created " + typeAccessor.GetKind() + "  " + accessor.GetName())
	return nil
}
