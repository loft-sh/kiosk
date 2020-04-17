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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
	"time"

	"github.com/kiosk-sh/kiosk/pkg/constants"
	"github.com/kiosk-sh/kiosk/pkg/manager/helm"
	"github.com/kiosk-sh/kiosk/pkg/manager/merge"
	"github.com/kiosk-sh/kiosk/pkg/util"
	"github.com/kiosk-sh/kiosk/pkg/util/convert"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	helm           helm.Helm
	newMergeClient newMergeClient
	restMapper     meta.RESTMapper

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
		if kerrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// If not pending and syncing then stop
	if templateInstance.Spec.Sync == false && templateInstance.Status.Status != configv1alpha1.TemplateInstanceDeploymentStatusPending {
		return ctrl.Result{}, nil
	}

	// Get template
	template := &configv1alpha1.Template{}
	err := r.Get(ctx, types.NamespacedName{Name: templateInstance.Spec.Template}, template)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return r.setFailed(ctx, templateInstance, "TemplateNotFound", fmt.Sprintf("The specified template '%s' couldn't be found.", templateInstance.Spec.Template))
		}

		return ctrl.Result{}, err
	}

	// Check if template instance has changed to last deployment
	if templateInstance.Status.TemplateResourceVersion == template.ResourceVersion && templateInstance.Status.Status == configv1alpha1.TemplateInstanceDeploymentStatusDeployed {
		return ctrl.Result{}, nil
	}

	// Try to deploy the template
	return r.deploy(ctx, template, templateInstance, log)
}

func (r *TemplateInstanceReconciler) deploy(ctx context.Context, template *configv1alpha1.Template, templateInstance *configv1alpha1.TemplateInstance, log logr.Logger) (ctrl.Result, error) {
	objects := []*unstructured.Unstructured{}

	// Gather objects from manifest
	if len(template.Resources.Manifests) > 0 {
		for _, manifest := range template.Resources.Manifests {
			objs, err := convert.StringToUnstructuredArray(string(manifest.Raw))
			if err != nil {
				return r.setFailed(ctx, templateInstance, "ErrorConvertingManifest", fmt.Sprintf("Error converting manifest %s: %v", string(manifest.Raw), err))
			}

			objects = append(objects, objs...)
		}
	}

	// Gather objects from helm
	if template.Resources.Helm != nil {
		objs, err := r.helm.Template(r, template.Name, templateInstance.Namespace, template.Resources.Helm)
		if err != nil {
			return r.setFailed(ctx, templateInstance, "ErrorHelm", fmt.Sprintf("Error during helm template: %v", err))
		}

		objects = append(objects, objs...)
	}

	return r.deployObjects(ctx, template, templateInstance, objects, log)
}

func (r *TemplateInstanceReconciler) deployObjects(ctx context.Context, template *configv1alpha1.Template, templateInstance *configv1alpha1.TemplateInstance, objects []*unstructured.Unstructured, log logr.Logger) (ctrl.Result, error) {
	var err error
	now := metav1.Now()
	templateInstance.Status = configv1alpha1.TemplateInstanceStatus{
		Status:                  configv1alpha1.TemplateInstanceDeploymentStatusDeployed,
		TemplateResourceVersion: template.ResourceVersion,
		TemplateManifests:       templateInstance.Status.TemplateManifests,
		LastAppliedAt:           &now,
	}

	// Create manifest string
	manifestsArray := []string{}
	for _, object := range objects {
		object.SetNamespace(templateInstance.Namespace)
		if r.restMapper != nil {
			// check what scope the object has
			groupVersion, err := schema.ParseGroupVersion(object.GetAPIVersion())
			if err != nil {
				return r.setFailed(ctx, templateInstance, "ParseGroupVersion", err.Error())
			}

			mapping, err := r.restMapper.RESTMapping(schema.GroupKind{
				Group: groupVersion.Group,
				Kind:  object.GetKind(),
			})
			if err != nil {
				if meta.IsNoMatchError(err) == false {
					return r.setFailed(ctx, templateInstance, "FindObjectMapping", err.Error())
				}
			}

			// Should set namespace and owner
			if mapping != nil && mapping.Scope != nil && mapping.Scope.Name() == meta.RESTScopeNameNamespace {
				// Set owner controller
				if shouldSetOwner(templateInstance) {
					_ = ctrl.SetControllerReference(templateInstance, object, r.Scheme)
				}
			} else {
				// global scoped objects cannot have namespaced scope resources as owners, so instead of setting the template
				// instance as owner, we set the template as owner instead, so if the complete template is deleted, the
				// resource is deleted as well, which is better than never deleting the resource
				if shouldSetOwner(templateInstance) {
					_ = ctrl.SetControllerReference(template, object, r.Scheme)
				}
			}
		}

		yaml, err := convert.ObjectToYaml(object)
		if err != nil {
			return r.setFailed(ctx, templateInstance, "ObjectToYamlError", err.Error())
		}

		manifestsArray = append(manifestsArray, string(yaml))
	}

	manifests := strings.Join(manifestsArray, "\n---\n")

	// Retrieve old manifests
	oldManifests := ""
	if templateInstance.Status.TemplateManifests != "" {
		oldManifests, err = util.Uncompress(templateInstance.Status.TemplateManifests)
		if err != nil {
			return r.setFailed(ctx, templateInstance, "DecompressOldManifests", err.Error())
		}
	}

	// Apply the manifests
	if oldManifests != manifests {
		err = r.newMergeClient().Merge(oldManifests, manifests, true)
		if err != nil {
			return r.setFailed(ctx, templateInstance, "ApplyManifests", err.Error())
		}
	}

	compressed, err := util.Compress(manifests)
	if err != nil {
		return r.setFailed(ctx, templateInstance, "CompressManifests", err.Error())
	}

	templateInstance.Status.TemplateManifests = compressed
	err = r.Status().Update(ctx, templateInstance)
	if err != nil {
		return r.setFailed(ctx, templateInstance, "ErrorUpdatingStatus", fmt.Sprintf("Couldn't update template instance status: %v", err))
	}

	return ctrl.Result{}, nil
}

func shouldSetOwner(templateInstance *configv1alpha1.TemplateInstance) bool {
	return templateInstance.Annotations == nil || templateInstance.Annotations[configv1alpha1.TemplateInstanceNoOwnerAnnotation] != "true"
}

func (r *TemplateInstanceReconciler) setFailed(ctx context.Context, templateInstance *configv1alpha1.TemplateInstance, reason, message string) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Template instance %s/%s failed: %s", templateInstance.Namespace, templateInstance.Name, message))
	templateInstance.Status = configv1alpha1.TemplateInstanceStatus{
		Status:                  configv1alpha1.TemplateInstanceDeploymentStatusFailed,
		Reason:                  reason,
		Message:                 message,
		TemplateManifests:       templateInstance.Status.TemplateManifests,
		TemplateResourceVersion: templateInstance.Status.TemplateResourceVersion,
	}

	return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Minute}, r.Status().Update(ctx, templateInstance)
}

type templateMapper struct {
	client client.Client

	Log logr.Logger
}

func (t *templateMapper) Map(obj handler.MapObject) []reconcile.Request {
	templateInstanceList := &configv1alpha1.TemplateInstanceList{}
	err := t.client.List(context.TODO(), templateInstanceList, client.MatchingFields{constants.IndexByTemplate: obj.Meta.GetName()})
	if err != nil {
		t.Log.Info("Template instance list failed: " + err.Error())
	}

	requests := []reconcile.Request{}
	for _, i := range templateInstanceList.Items {
		// Don't reconcile instances that don't sync
		if !i.Spec.Sync {
			continue
		}

		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      i.Name,
				Namespace: i.Namespace,
			},
		})
	}

	return requests
}

type newMergeClient func() merge.Interface

// SetupWithManager adds the controller to the manager
func (r *TemplateInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.helm = helm.NewHelmRunner()
	r.newMergeClient = func() merge.Interface { return merge.New(nil) }
	r.restMapper = mgr.GetRESTMapper()

	return ctrl.NewControllerManagedBy(mgr).
		Watches(&source.Kind{Type: &configv1alpha1.Template{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: &templateMapper{client: r, Log: r.Log}}).
		For(&configv1alpha1.TemplateInstance{}).
		Complete(r)
}
