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
	"github.com/ghodss/yaml"
	"github.com/loft-sh/kiosk/pkg/util/loghelper"
	"github.com/loft-sh/kiosk/pkg/util/parameters"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"regexp"
	"strings"
	"time"

	"github.com/loft-sh/kiosk/pkg/constants"
	"github.com/loft-sh/kiosk/pkg/manager/helm"
	"github.com/loft-sh/kiosk/pkg/manager/merge"
	"github.com/loft-sh/kiosk/pkg/util"
	"github.com/loft-sh/kiosk/pkg/util/convert"

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

	configv1alpha1 "github.com/loft-sh/kiosk/pkg/apis/config/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

// TemplateInstanceReconciler reconciles a template instance object
type TemplateInstanceReconciler struct {
	client.Client
	helm           helm.Helm
	newMergeClient newMergeClient
	restMapper     meta.RESTMapper

	Log    loghelper.Logger
	Scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for an Account object and makes changes based on the state read
func (r *TemplateInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := loghelper.NewFromExisting(r.Log, req.Namespace+"/"+req.Name)
	log.Debugf("reconcile started")

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
	return r.deploy(ctx, template, templateInstance)
}

func (r *TemplateInstanceReconciler) deploy(ctx context.Context, template *configv1alpha1.Template, templateInstance *configv1alpha1.TemplateInstance) (ctrl.Result, error) {
	objects := []*unstructured.Unstructured{}

	// check if parameters were filled correctly
	err := validateParameters(template, templateInstance)
	if err != nil {
		return r.setFailed(ctx, templateInstance, "ErrorValidatingParameters", err.Error())
	}

	// Gather objects from manifest
	if len(template.Resources.Manifests) > 0 {
		for _, manifest := range template.Resources.Manifests {
			objs, err := convert.StringToUnstructuredArray(string(manifest.Raw))
			if err != nil {
				return r.setFailed(ctx, templateInstance, "ErrorConvertingManifest", fmt.Sprintf("Error converting manifest %s: %v", string(manifest.Raw), err))
			}

			// replace parameters
			for _, obj := range objs {
				err = r.replaceUnstructuredParameters(ctx, template, templateInstance, obj)
				if err != nil {
					return r.setFailed(ctx, templateInstance, "ErrorReplaceParameters", fmt.Sprintf("Error replacing parameters in manifest: %v", err))
				}
			}

			objects = append(objects, objs...)
		}
	}

	// Gather objects from helm
	if template.Resources.Helm != nil {
		helmOptions := template.Resources.Helm.DeepCopy()

		// replace values if there are any
		helmOptions.Values, err = r.replaceHelmValuesParameters(ctx, template, templateInstance, helmOptions.Values)
		if err != nil {
			return r.setFailed(ctx, templateInstance, "ErrorReplaceParameters", fmt.Sprintf("Error replacing parameters in helm values: %v", err))
		}

		// template the chart
		objs, err := r.helm.Template(r.Client, template.Name, templateInstance.Namespace, helmOptions)
		if err != nil {
			return r.setFailed(ctx, templateInstance, "ErrorHelm", fmt.Sprintf("Error during helm template: %v", err))
		}

		objects = append(objects, objs...)
	}

	return r.deployObjects(ctx, template, templateInstance, objects)
}

func (r *TemplateInstanceReconciler) replaceUnstructuredParameters(ctx context.Context, template *configv1alpha1.Template, templateInstance *configv1alpha1.TemplateInstance, obj *unstructured.Unstructured) error {
	if obj == nil {
		return nil
	}

	err := parameters.WalkStringMap(obj.Object, func(value string) (interface{}, error) {
		return r.replaceVariable(ctx, template, templateInstance, value)
	})
	if err != nil {
		return errors.Wrap(err, "replace parameters")
	}

	return nil
}

func (r *TemplateInstanceReconciler) replaceHelmValuesParameters(ctx context.Context, template *configv1alpha1.Template, templateInstance *configv1alpha1.TemplateInstance, values string) (string, error) {
	if values == "" {
		return values, nil
	}

	valuesObj := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(values), &valuesObj)
	if err != nil {
		return "", errors.Wrap(err, "unmarshal helm values")
	}

	err = parameters.WalkStringMap(valuesObj, func(value string) (interface{}, error) {
		return r.replaceVariable(ctx, template, templateInstance, value)
	})
	if err != nil {
		return "", errors.Wrap(err, "replace parameters")
	}

	// marshal the values back in to a string
	retValues, err := yaml.Marshal(valuesObj)
	if err != nil {
		return "", errors.Wrap(err, "marshal replace helm values")
	}

	return string(retValues), nil
}

func (r *TemplateInstanceReconciler) resolvePredefinedVariable(ctx context.Context, templateInstance *configv1alpha1.TemplateInstance, value string) (bool, string, error) {
	lowerValue := strings.ToLower(value)
	var (
		isNamespace           = lowerValue == "namespace"
		isNamespaceAnnotation = strings.HasPrefix(lowerValue, "namespace.metadata.annotations.")
		isNamespaceLabel      = strings.HasPrefix(lowerValue, "namespace.metadata.labels.")
		isAccount             = lowerValue == "account"
		isAccountAnnotation   = strings.HasPrefix(lowerValue, "account.metadata.annotations.")
		isAccountLabel        = strings.HasPrefix(lowerValue, "account.metadata.labels.")
	)

	// targets namespace?
	if isNamespace {
		return true, templateInstance.Namespace, nil
	} else if isNamespaceAnnotation || isNamespaceLabel {
		namespace := &corev1.Namespace{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: templateInstance.Namespace}, namespace)
		if err != nil {
			return false, "", errors.Wrap(err, "get template instance namespace")
		} else if isNamespaceAnnotation && namespace.Annotations == nil {
			return false, "", errors.Errorf("namespace %s has no annotations, however ${%s} parameter is used", namespace.Name, value)
		} else if isNamespaceLabel && namespace.Labels == nil {
			return false, "", errors.Errorf("namespace %s has no labels, however ${%s} parameter is used", namespace.Name, value)
		}

		var (
			identifier string
			retVal     string
			ok         bool
		)
		if isNamespaceAnnotation {
			identifier = value[len("namespace.metadata.annotations."):]
			retVal, ok = namespace.Annotations[identifier]
		} else {
			identifier = value[len("namespace.metadata.labels."):]
			retVal, ok = namespace.Labels[identifier]
		}
		if !ok {
			return false, "", errors.Errorf("%s not found on namespace %s, however ${%s} parameter is used", identifier, namespace.Name, value)
		}

		return true, retVal, nil
	}

	// targets account?
	if isAccount || isAccountAnnotation || isAccountLabel {
		namespace := &corev1.Namespace{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: templateInstance.Namespace}, namespace)
		if err != nil {
			return false, "", errors.Wrap(err, "get template instance namespace")
		} else if namespace.Labels == nil || namespace.Labels[constants.SpaceLabelAccount] == "" {
			return false, "", errors.Errorf("space is not owned by an account, however ${%s} parameter is used", value)
		}

		accountName := namespace.Labels[constants.SpaceLabelAccount]
		if isAccount {
			return true, accountName, nil
		}

		account := &configv1alpha1.Account{}
		err = r.Client.Get(ctx, types.NamespacedName{Name: accountName}, account)
		if err != nil {
			return false, "", errors.Errorf("account %s that owns namespace %s does not exist, however ${%s} parameter is used", accountName, namespace.Name, value)
		} else if isAccountAnnotation && account.Annotations == nil {
			return false, "", errors.Errorf("account %s has no annotations, however ${%s} parameter is used", account.Name, value)
		} else if isAccountLabel && account.Labels == nil {
			return false, "", errors.Errorf("account %s has no labels, however ${%s} parameter is used", account.Name, value)
		}

		var (
			identifier string
			retVal     string
			ok         bool
		)
		if isAccountAnnotation {
			identifier = value[len("account.metadata.annotations."):]
			retVal, ok = account.Annotations[identifier]
		} else {
			identifier = value[len("account.metadata.labels."):]
			retVal, ok = account.Labels[identifier]
		}
		if !ok {
			return false, "", errors.Errorf("%s not found on account %s, however ${%s} parameter is used", identifier, account.Name, value)
		}

		return true, retVal, nil
	}

	return false, "", nil
}

func (r *TemplateInstanceReconciler) replaceVariable(ctx context.Context, template *configv1alpha1.Template, templateInstance *configv1alpha1.TemplateInstance, value string) (interface{}, error) {
	return parameters.ParseString(value, func(value string) (string, error) {
		// check if predefined variable
		wasPredefined, retValue, err := r.resolvePredefinedVariable(ctx, templateInstance, value)
		if err != nil {
			return "", err
		} else if wasPredefined {
			return retValue, nil
		}

		// check if value is in template instance
		for _, v := range templateInstance.Spec.Parameters {
			if v.Name == value {
				return v.Value, nil
			}
		}

		// check if value is in template
		for _, v := range template.Parameters {
			if v.Name == value {
				return v.Value, nil
			}
		}

		return "${" + value + "}", nil
	})
}

func validateParameters(template *configv1alpha1.Template, templateInstance *configv1alpha1.TemplateInstance) error {
	if len(template.Parameters) > 0 {
		for _, parameter := range template.Parameters {
			if parameter.Name == "" {
				continue
			} else if parameter.Required == false && parameter.Validation == "" {
				continue
			}

			// make sure the template instance filled the value correctly
			found := false
			for _, instanceParameter := range templateInstance.Spec.Parameters {
				if instanceParameter.Name == parameter.Name {
					found = true
					if parameter.Validation != "" {
						matched, err := regexp.MatchString(parameter.Validation, instanceParameter.Value)
						if err != nil {
							return errors.Wrap(err, "match string")
						} else if matched == false {
							return errors.Errorf("parameter %s value %s does not match validation pattern %s", parameter.Name, instanceParameter.Value, parameter.Validation)
						}
					}
				}
			}
			if !found && parameter.Required {
				return errors.Errorf("parameter %s is required but was not found in template instance", parameter.Name)
			}
		}
	}

	for _, instanceParameter := range templateInstance.Spec.Parameters {
		if instanceParameter.Name == "" {
			continue
		}

		found := false
		for _, parameter := range template.Parameters {
			if parameter.Name == instanceParameter.Name {
				found = true
				break
			}
		}
		if !found {
			return errors.Errorf("parameter %s does not exist in template %s", instanceParameter.Name, template.Name)
		}
	}

	return nil
}

func (r *TemplateInstanceReconciler) deployObjects(ctx context.Context, template *configv1alpha1.Template, templateInstance *configv1alpha1.TemplateInstance, objects []*unstructured.Unstructured) (ctrl.Result, error) {
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
	r.Log.Infof("Template instance %s/%s failed: %s", templateInstance.Namespace, templateInstance.Name, message)
	templateInstance.Status = configv1alpha1.TemplateInstanceStatus{
		Status:                  configv1alpha1.TemplateInstanceDeploymentStatusFailed,
		Reason:                  reason,
		Message:                 message,
		TemplateManifests:       templateInstance.Status.TemplateManifests,
		TemplateResourceVersion: templateInstance.Status.TemplateResourceVersion,
	}

	return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Minute}, r.Status().Update(ctx, templateInstance)
}

func mapTemplateInstances(kubeClient client.Client, obj client.Object, log logr.Logger) []reconcile.Request {
	templateInstanceList := &configv1alpha1.TemplateInstanceList{}
	err := kubeClient.List(context.TODO(), templateInstanceList, client.MatchingFields{constants.IndexByTemplate: obj.GetName()})
	if err != nil {
		log.Info("template instance list failed: " + err.Error())
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
		Watches(&source.Kind{Type: &configv1alpha1.Template{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return mapTemplateInstances(r.Client, object, r.Log)
		})).
		For(&configv1alpha1.TemplateInstance{}).
		Complete(r)
}
