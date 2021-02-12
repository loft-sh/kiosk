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
	"github.com/loft-sh/kiosk/kube/plugin/pkg/auth/authorizer/rbac"
	configv1alpha1 "github.com/loft-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/loft-sh/kiosk/pkg/apis/tenancy"
	"github.com/loft-sh/kiosk/pkg/apis/tenancy/validation"
	"github.com/loft-sh/kiosk/pkg/apiserver/registry/util"
	"github.com/loft-sh/kiosk/pkg/authorization"
	"github.com/loft-sh/kiosk/pkg/constants"
	"github.com/loft-sh/kiosk/pkg/util/loghelper"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	authorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/filters"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type spaceStorage struct {
	authorizer authorizer.Authorizer
	scheme     *runtime.Scheme
	filter     authorization.FilteredLister
	client     client.Client
}

// NewSpaceREST creates a new space storage that implements the rest interface
func NewSpaceREST(cachedClient client.Client, uncachedClient client.Client, scheme *runtime.Scheme) rest.Storage {
	ruleClient := authorization.NewRuleClient(cachedClient)
	authorizer := rbac.New(ruleClient, ruleClient, ruleClient, ruleClient)
	return &spaceStorage{
		client:     uncachedClient,
		authorizer: authorizer,
		scheme:     scheme,
		filter:     authorization.NewFilteredLister(uncachedClient, authorizer),
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

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

func (r *spaceStorage) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	var table metav1.Table
	fn := func(obj runtime.Object) error {
		space, ok := obj.(*tenancy.Space)
		if !ok {
			return fmt.Errorf("cannot convert to space: %#+v", obj)
		}

		table.Rows = append(table.Rows, metav1.TableRow{
			Cells:  []interface{}{space.GetName(), space.Spec.Account, space.GetCreationTimestamp().Time.UTC().Format(time.RFC3339)},
			Object: runtime.RawExtension{Object: obj},
		})
		return nil
	}
	switch {
	case meta.IsListType(object):
		if err := meta.EachListItem(object, fn); err != nil {
			return nil, err
		}
	default:
		if err := fn(object); err != nil {
			return nil, err
		}
	}
	if m, err := meta.ListAccessor(object); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.SelfLink = m.GetSelfLink()
		table.Continue = m.GetContinue()
		table.RemainingItemCount = m.GetRemainingItemCount()
	} else {
		if m, err := meta.CommonAccessor(object); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
			table.SelfLink = m.GetSelfLink()
		}
	}
	if opt, ok := tableOptions.(*metav1.TableOptions); !ok || !opt.NoHeaders {
		table.ColumnDefinitions = []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "Owner", Type: "string", Description: "The account that owns this space"},
			{Name: "Created At", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
		}
	}
	return &table, nil
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

	// check if user can create namespaces
	a, err := filters.GetAuthorizerAttributes(ctx)
	if err != nil {
		return nil, err
	}

	// Check if user can access account and create space
	var account *configv1alpha1.Account
	if space.Spec.Account == "" {
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
			err := r.client.List(ctx, namespaceList, client.MatchingLabels{constants.SpaceLabelAccount: account.Name})
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

	// print some log information
	loghelper.Infof("create space %s for user %s", space.Name, a.GetUser().GetName())

	// Create the target namespace
	namespace := ConvertSpace(space)
	if account != nil {
		namespace.Annotations[constants.SpaceAnnotationInitializing] = "true"
		err := r.client.Create(ctx, namespace, &client.CreateOptions{
			Raw: options,
		})
		if err != nil {
			loghelper.Infof("error creating namespace %s for user %s: %v", namespace.Name, a.GetUser().GetName(), err)
			return nil, err
		}

		// Create the default space templates and role binding
		loghelper.Infof("initialize space %s for user %s", namespace.Name, a.GetUser().GetName())
		err = r.initializeSpace(ctx, namespace, account)
		if err != nil {
			// we have to use a background context here, because it might
			// be possible that the user is cancelling the request
			spaceErr := r.client.Delete(context.Background(), namespace)
			if spaceErr != nil {
				loghelper.Infof("error deleting namespace %s after creation: %v", namespace.Name, spaceErr)
			}

			loghelper.Infof("error initializing space %s for user %s: %v", namespace.Name, a.GetUser().GetName(), err)
			return nil, err
		}

		// wait until we get access
		err = r.waitForAccess(ctx, a.GetUser(), namespace)
		if err != nil {
			// if this happens it is kind of weird, but its not a reason to return an error and abort the request
			loghelper.Infof("error waiting for access to namespace %s for user %s: %v", namespace.Name, a.GetUser().GetName(), err)
		}
	} else {
		err := r.client.Create(ctx, namespace, &client.CreateOptions{
			Raw: options,
		})
		if err != nil {
			return nil, err
		}
	}

	loghelper.Infof("successfully created space %s for user %s", namespace.Name, a.GetUser().GetName())
	return ConvertNamespace(namespace), nil
}

func (r *spaceStorage) waitForAccess(ctx context.Context, user user.Info, namespace *corev1.Namespace) error {
	a := &authorizer.AttributesRecord{
		User:            user,
		Verb:            "get",
		Namespace:       namespace.Name,
		APIGroup:        corev1.SchemeGroupVersion.Group,
		APIVersion:      corev1.SchemeGroupVersion.Version,
		Resource:        "namespaces",
		Name:            namespace.Name,
		ResourceRequest: true,
	}

	// here we wait until the authorizer tells us that the account can get the space
	return wait.PollImmediate(time.Second, time.Second*5, func() (bool, error) {
		decision, _, err := r.authorizer.Authorize(ctx, a)
		if err != nil {
			return false, err
		}

		return decision == authorizer.DecisionAllow, nil
	})
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
	err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
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
	originalNamespace := namespace.DeepCopy()
	delete(namespace.Annotations, constants.SpaceAnnotationInitializing)
	return r.client.Patch(ctx, namespace, client.MergeFrom(originalNamespace))
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
		return nil, false, fmt.Errorf("new object is not a space")
	}

	namespace := ConvertSpace(newSpace)
	err = r.client.Update(ctx, namespace, &client.UpdateOptions{
		Raw: options,
	})
	if err != nil {
		return nil, false, err
	}

	return ConvertNamespace(namespace), true, nil
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

	// we have to use a background context here, because it might
	// be possible that the user is cancelling the request and we want
	// to fully delete the namespace or otherwise there might be left overs
	err = r.client.Delete(context.Background(), namespace, &client.DeleteOptions{
		Raw: options,
	})
	if err != nil {
		return nil, false, err
	}

	return ConvertNamespace(namespace), true, nil
}
