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

package space

import (
	"context"
	"errors"
	"fmt"
	"github.com/kiosk-sh/kiosk/kube/plugin/pkg/auth/authorizer/rbac"
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy/validation"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/registry/util"
	"github.com/kiosk-sh/kiosk/pkg/authorization"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	authorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/filters"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type spaceStorage struct {
	authorizer authorizer.Authorizer
	scheme     *runtime.Scheme
	filter     authorization.FilteredLister
	client     client.Client
}

// NewSpaceREST creates a new space storage that implements the rest interface
func NewSpaceREST(client client.Client, scheme *runtime.Scheme) rest.Storage {
	ruleClient := authorization.NewRuleClient(client)
	authorizer := rbac.New(ruleClient, ruleClient, ruleClient, ruleClient)
	return &spaceStorage{
		client:     client,
		authorizer: authorizer,
		scheme:     scheme,
		filter:     authorization.NewFilteredLister(client, authorizer),
	}
}

var _ = rest.Scoper(&spaceStorage{})

func (r *spaceStorage) NamespaceScoped() bool {
	return false
}

var _ = rest.Storage(&spaceStorage{})

func (r *spaceStorage) New() runtime.Object {
	return &tenancy.Space{}
}

var _ = rest.Lister(&spaceStorage{})

func (r *spaceStorage) NewList() runtime.Object {
	return &tenancy.SpaceList{}
}

func (r *spaceStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	namespaces := &corev1.NamespaceList{}
	_, err := r.filter.List(ctx, namespaces, corev1.SchemeGroupVersion.WithResource("namespaces"), options)
	if err != nil {
		return nil, err
	}

	spaceList := &tenancy.SpaceList{
		Items: []tenancy.Space{},
	}
	for _, n := range namespaces.Items {
		spaceList.Items = append(spaceList.Items, *ConvertNamespace(&n))
	}

	return spaceList, nil
}

var _ = rest.Getter(&spaceStorage{})

func (r *spaceStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	a, err := filters.GetAuthorizerAttributes(ctx)
	if err != nil {
		return nil, err
	}

	decision, _, err := r.authorizer.Authorize(ctx, util.ChangeAttributesResource(a, corev1.SchemeGroupVersion.WithResource("namespaces"), name))
	if err != nil {
		return nil, err
	} else if decision != authorizer.DecisionAllow {
		return nil, kerrors.NewNotFound(tenancy.SchemeGroupVersion.WithResource("space").GroupResource(), name)
		// return nil, kerrors.NewForbidden(tenancy.SchemeGroupVersion.WithResource("space").GroupResource(), name, errors.New(reason))
	}

	namespaceObj := &corev1.Namespace{}
	err = r.client.Get(ctx, types.NamespacedName{Name: name}, namespaceObj)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, kerrors.NewNotFound(tenancy.Resource("spaces"), name)
		}

		return nil, err
	}

	return ConvertNamespace(namespaceObj), nil
}

var _ = rest.Creater(&spaceStorage{})

func (r *spaceStorage) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("couldn't find user in request")
	}

	space, ok := obj.(*tenancy.Space)
	if !ok {
		return nil, fmt.Errorf("not a space: %#v", obj)
	}

	// Validation phase
	rest.FillObjectMetaSystemFields(&space.ObjectMeta)
	errs := validation.ValidateSpace(space)
	if len(errs) > 0 {
		return nil, kerrors.NewInvalid(tenancy.Kind("Space"), space.Name, errs)
	}
	if err := createValidation(ctx, obj); err != nil {
		return nil, err
	}

	// Check if user can access account and create space
	var account *configv1alpha1.Account
	if space.Spec.Account == "" {
		// check if user can create namespaces
		a, err := filters.GetAuthorizerAttributes(ctx)
		if err != nil {
			return nil, err
		}

		decision, _, err := r.authorizer.Authorize(ctx, util.ChangeAttributesResource(a, corev1.SchemeGroupVersion.WithResource("namespaces"), space.Name))
		if err != nil {
			return nil, err
		} else if decision != authorizer.DecisionAllow {
			return nil, kerrors.NewBadRequest("spec.account is required")
		}
	} else {
		account = &configv1alpha1.Account{}
		err := r.client.Get(ctx, types.NamespacedName{Name: space.Spec.Account}, account)
		if err != nil {
			return nil, err
		}

		// check if user is part of account
		if util.IsUserPartOfAccount(userInfo, account) == false {
			// check if user can create namespaces
			a, err := filters.GetAuthorizerAttributes(ctx)
			if err != nil {
				return nil, err
			}

			decision, _, err := r.authorizer.Authorize(ctx, util.ChangeAttributesResource(a, corev1.SchemeGroupVersion.WithResource("namespaces"), space.Name))
			if err != nil {
				return nil, err
			} else if decision != authorizer.DecisionAllow {
				return nil, kerrors.NewForbidden(tenancy.SchemeGroupVersion.WithResource("space").GroupResource(), space.Name, errors.New(util.ForbiddenMessage(a)))
			}
		}
	}

	// Check if account is at limit
	if account != nil {
		if account.Spec.Space.Limit != nil {
			namespaceList := &corev1.NamespaceList{}
			err := r.client.List(ctx, namespaceList, client.MatchingFields{constants.IndexByAccount: account.Name})
			if err != nil {
				return nil, err
			}

			if len(namespaceList.Items) >= *account.Spec.Space.Limit {
				return nil, kerrors.NewForbidden(tenancy.Resource("spaces"), space.Name, fmt.Errorf("space limit of %d reached for account %s", *account.Spec.Space.Limit, account.Name))
			}
		}

		// Apply namespace annotations & labels
		if account.Spec.Space.SpaceTemplate.Labels != nil {
			if space.ObjectMeta.Labels == nil {
				space.ObjectMeta.Labels = map[string]string{}
			}
			for k, v := range account.Spec.Space.SpaceTemplate.Labels {
				space.ObjectMeta.Labels[k] = v
			}
		}
		if account.Spec.Space.SpaceTemplate.Annotations != nil {
			if space.ObjectMeta.Annotations == nil {
				space.ObjectMeta.Annotations = map[string]string{}
			}
			for k, v := range account.Spec.Space.SpaceTemplate.Annotations {
				space.ObjectMeta.Annotations[k] = v
			}
		}
	}

	// Create the target namespace
	namespace := ConvertSpace(space)
	if len(options.DryRun) == 0 && account != nil {
		namespace.Annotations[constants.SpaceAnnotationInitializing] = "true"
		err := r.client.Create(ctx, namespace, &client.CreateOptions{
			Raw: options,
		})
		if err != nil {
			return nil, err
		}

		// Create the default space templates and role binding
		err = r.initializeSpace(ctx, namespace, account)
		if err != nil {
			r.client.Delete(ctx, namespace)
			return nil, err
		}
	} else {
		err := r.client.Create(ctx, namespace, &client.CreateOptions{
			Raw: options,
		})
		if err != nil {
			return nil, err
		}
	}

	// Wait till we have the namespace in the cache
	err := r.waitForAccess(ctx, namespace.Name)
	if err != nil {
		return nil, err
	}

	return ConvertNamespace(namespace), nil
}

func (r *spaceStorage) initializeSpace(ctx context.Context, namespace *corev1.Namespace, account *configv1alpha1.Account) error {
	// Create template instances
	templateInstances := []*configv1alpha1.TemplateInstance{}
	for _, instSpec := range account.Spec.Space.TemplateInstances {
		if instSpec.Spec.Template == "" {
			continue
		}

		templateInstance := &configv1alpha1.TemplateInstance{
			ObjectMeta: instSpec.ObjectMeta,
			Spec:       instSpec.Spec,
		}

		templateInstance.Namespace = namespace.Name
		if templateInstance.Name == "" && templateInstance.GenerateName == "" {
			templateInstance.GenerateName = instSpec.Spec.Template + "-"
		}

		err := r.client.Create(ctx, templateInstance)
		if err != nil {
			return err
		}

		templateInstances = append(templateInstances, templateInstance)
	}

	// Wait for template instances to be deployed
	backoff := retry.DefaultBackoff
	backoff.Steps = 8
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		// Get the template instances
		instanceList := &configv1alpha1.TemplateInstanceList{}
		err := r.client.List(ctx, instanceList, client.InNamespace(namespace.Name))
		if err != nil {
			return false, err
		}

		// Check if instances are ready
		for _, inst := range templateInstances {
			found := false
			for _, realInst := range instanceList.Items {
				if inst.Name == realInst.Name {
					found = true
					if realInst.Status.Status == configv1alpha1.TemplateInstanceDeploymentStatusPending {
						return false, nil
					} else if realInst.Status.Status == configv1alpha1.TemplateInstanceDeploymentStatusFailed {
						return false, fmt.Errorf("TemplateInstance '%s' failed with %s: %s", realInst.Name, realInst.Status.Reason, realInst.Status.Message)
					}
				}
			}

			if !found {
				return false, nil
			}
		}

		// if we reach this point we found all instances and they are deployed
		return true, nil
	})
	if err != nil {
		return err
	}

	// Create role binding
	err = util.CreateRoleBinding(ctx, r.client, namespace.Name, account, r.scheme)
	if err != nil {
		return err
	}

	// Update namespace intialization
	for true {
		delete(namespace.Annotations, constants.SpaceAnnotationInitializing)
		err = r.client.Update(ctx, namespace)
		if err != nil {
			if kerrors.IsConflict(err) {
				// re get namespace to avoid conflict errors
				err = r.client.Get(ctx, types.NamespacedName{Name: namespace.Name}, namespace)
				if err != nil {
					return err
				}

				continue
			}

			return err
		}

		break
	}

	return nil
}

// waitForAccess blocks until the namespace is created and in our cache
func (r *spaceStorage) waitForAccess(ctx context.Context, namespace string) error {
	backoff := retry.DefaultBackoff
	backoff.Steps = 6 // this effectively waits for 6-ish seconds
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := r.client.Get(ctx, types.NamespacedName{Name: namespace}, &corev1.Namespace{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				return false, nil
			}

			return false, err
		}

		return true, nil
	})

	return err
}

var _ = rest.Updater(&spaceStorage{})
var _ = rest.CreaterUpdater(&spaceStorage{})

func (r *spaceStorage) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	a, err := filters.GetAuthorizerAttributes(ctx)
	if err != nil {
		return nil, false, err
	}

	decision, _, err := r.authorizer.Authorize(ctx, util.ChangeAttributesResource(a, corev1.SchemeGroupVersion.WithResource("namespaces"), name))
	if err != nil {
		return nil, false, err
	} else if decision != authorizer.DecisionAllow {
		return nil, false, kerrors.NewForbidden(tenancy.SchemeGroupVersion.WithResource("space").GroupResource(), name, errors.New(util.ForbiddenMessage(a)))
	}

	oldObj, err := r.Get(ctx, name, nil)
	if err != nil {
		return nil, false, err
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}

	newSpace, ok := newObj.(*tenancy.Space)
	if !ok {
		return nil, false, fmt.Errorf("New object is not a space")
	}

	namespace := ConvertSpace(newSpace)
	err = r.client.Update(ctx, namespace, &client.UpdateOptions{
		Raw: options,
	})
	if err != nil {
		return nil, false, err
	}

	return newSpace, true, nil
}

var _ = rest.GracefulDeleter(&spaceStorage{})

func (r *spaceStorage) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	a, err := filters.GetAuthorizerAttributes(ctx)
	if err != nil {
		return nil, false, err
	}

	decision, _, err := r.authorizer.Authorize(ctx, util.ChangeAttributesResource(a, corev1.SchemeGroupVersion.WithResource("namespaces"), name))
	if err != nil {
		return nil, false, err
	} else if decision != authorizer.DecisionAllow {
		return nil, false, kerrors.NewForbidden(tenancy.SchemeGroupVersion.WithResource("space").GroupResource(), name, errors.New(util.ForbiddenMessage(a)))
	}

	namespace := &corev1.Namespace{}
	err = r.client.Get(ctx, types.NamespacedName{Name: name}, namespace)
	if err != nil {
		return nil, false, err
	}

	err = r.client.Delete(ctx, namespace, &client.DeleteOptions{
		Raw: options,
	})
	if err != nil {
		return nil, false, err
	}

	return ConvertNamespace(namespace), true, nil
}
