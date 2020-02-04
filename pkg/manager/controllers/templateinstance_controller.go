/*
Copyright 2020 DevSpace Technologies Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/kiosk-sh/kiosk/pkg/manager/helm"
	"github.com/kiosk-sh/kiosk/pkg/util/convert"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

// TemplateInstanceReconciler reconciles a template instance object
type TemplateInstanceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for an Account object and makes changes based on the state read
func (r *TemplateInstanceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("templateinstance", req.NamespacedName)

	log.Info("Template Instance reconcile started")

	// Retrieve account
	templateInstance := &configv1alpha1.TemplateInstance{}
	if err := r.Get(ctx, req.NamespacedName, templateInstance); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// If not pending then stop
	if templateInstance.Status.Status != configv1alpha1.TemplateInstanceDeploymentStatusPending {
		return ctrl.Result{}, nil
	}

	// Get template
	template := &configv1alpha1.Template{}
	err := r.Get(ctx, types.NamespacedName{Name: templateInstance.Spec.Template}, template)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return ctrl.Result{}, r.setFailed(ctx, templateInstance, "TemplateNotFound", fmt.Sprintf("The specified template '%s' couldn't be found.", templateInstance.Spec.Template))
		}

		return ctrl.Result{}, err
	}

	// Try to deploy the template
	err = r.deploy(ctx, template, templateInstance, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *TemplateInstanceReconciler) deploy(ctx context.Context, template *configv1alpha1.Template, templateInstance *configv1alpha1.TemplateInstance, log logr.Logger) error {
	objects := []*unstructured.Unstructured{}

	// Gather objects from manifest
	if len(template.Resources.Manifests) > 0 {
		for _, manifest := range template.Resources.Manifests {
			objs, err := convert.YAML(string(manifest.Raw))
			if err != nil {
				return r.setFailed(ctx, templateInstance, "ErrorConvertingManifest", fmt.Sprintf("Error converting manifest %s: %v", string(manifest.Raw), err))
			}

			objects = append(objects, objs...)
		}
	}

	// Gather objects from helm
	if template.Resources.Helm != nil {
		objs, err := helm.Template(r, template.Name, templateInstance.Namespace, template.Resources.Helm)
		if err != nil {
			return r.setFailed(ctx, templateInstance, "ErrorHelm", fmt.Sprintf("Error during helm template: %v", err))
		}

		objects = append(objects, objs...)
	}

	return r.deployObjects(ctx, template, templateInstance, objects, log)
}

func (r *TemplateInstanceReconciler) deployObjects(ctx context.Context, template *configv1alpha1.Template, templateInstance *configv1alpha1.TemplateInstance, objects []*unstructured.Unstructured, log logr.Logger) error {
	now := metav1.Now()
	templateInstance.Status = configv1alpha1.TemplateInstanceStatus{
		Status:                  configv1alpha1.TemplateInstanceDeploymentStatusDeployed,
		Resources:               []configv1alpha1.ResourceStatus{},
		TemplateResourceVersion: template.ResourceVersion,
		LastAppliedAt:           &now,
	}

	// Deploy all objects
	for _, object := range objects {
		object.SetNamespace(templateInstance.Namespace)

		// Get group version
		gv, err := schema.ParseGroupVersion(object.GetAPIVersion())
		if err != nil {
			return r.setFailed(ctx, templateInstance, "ErrorParsingGroupVersion", fmt.Sprintf("Error parsing apiVersion of %s: %v", object.GetName(), err))
		}

		// Create the object
		err = r.Create(ctx, object)
		if err != nil {
			return r.setFailed(ctx, templateInstance, "ErrorCreatingObject", fmt.Sprintf("Error creating object %s: %v", object.GetName(), err))
		}

		templateInstance.Status.Resources = append(templateInstance.Status.Resources, configv1alpha1.ResourceStatus{
			Group:           gv.Group,
			Version:         gv.Version,
			Kind:            object.GetKind(),
			ResourceVersion: object.GetResourceVersion(),
			Name:            object.GetName(),
			Namespace:       object.GetNamespace(),
			UID:             object.GetUID(),
		})
	}

	err := r.Status().Update(ctx, templateInstance)
	if err != nil {
		return r.setFailed(ctx, templateInstance, "ErrorUpdatingStatus", fmt.Sprintf("Couldn't update template instance status: %v", err))
	}

	return nil
}

func (r *TemplateInstanceReconciler) setFailed(ctx context.Context, templateInstance *configv1alpha1.TemplateInstance, reason, message string) error {
	r.Log.Info("Template instance failed: " + message)

	templateInstance.Status = configv1alpha1.TemplateInstanceStatus{
		Status:  configv1alpha1.TemplateInstanceDeploymentStatusFailed,
		Reason:  reason,
		Message: message,
	}

	return r.Status().Update(ctx, templateInstance)
}

type templateMapper struct {
	client client.Client
}

func (t *templateMapper) Map(obj handler.MapObject) []reconcile.Request {
	// TODO: Sync
	return []reconcile.Request{}
}

// SetupWithManager adds the controller to the manager
func (r *TemplateInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(&source.Kind{Type: &configv1alpha1.Template{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: &templateMapper{client: r}}).
		For(&configv1alpha1.TemplateInstance{}).
		Complete(r)
}
