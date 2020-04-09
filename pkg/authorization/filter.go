package authorization

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/filters"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
)

type FilteredLister interface {
	List(ctx context.Context, list runtime.Object, groupVersion schema.GroupVersionResource, options *metainternalversion.ListOptions) (runtime.Object, error)
}

func NewFilteredLister(client client.Client, authorizer authorizer.Authorizer) FilteredLister {
	return &filter{
		client:     client,
		authorizer: authorizer,
	}
}

type filter struct {
	client     client.Client
	authorizer authorizer.Authorizer
}

func (f *filter) List(ctx context.Context, list runtime.Object, groupVersionResource schema.GroupVersionResource, options *metainternalversion.ListOptions) (runtime.Object, error) {
	a, err := filters.GetAuthorizerAttributes(ctx)
	if err != nil {
		return nil, err
	}

	err = f.client.List(ctx, list, &client.ListOptions{
		LabelSelector: options.LabelSelector,
		// TODO: support this, since it is currently not supported by the underlying cache implementation
		// FieldSelector: options.FieldSelector,
		Namespace: a.GetNamespace(),
		Limit:     options.Limit,
		Continue:  options.Continue,
	})
	if err != nil {
		return nil, err
	}

	objs, err := meta.ExtractList(list)
	if err != nil {
		return nil, err
	}

	nameAsNamespace := groupVersionResource.Group == corev1.SchemeGroupVersion.Group && groupVersionResource.Version == corev1.SchemeGroupVersion.Version && groupVersionResource.Resource == "namespaces"
	if len(objs) > 0 {
		attributes := authorizer.AttributesRecord{
			User:            a.GetUser(),
			Verb:            "get",
			Namespace:       a.GetNamespace(),
			APIGroup:        groupVersionResource.Group,
			APIVersion:      groupVersionResource.Version,
			Resource:        groupVersionResource.Resource,
			Subresource:     a.GetSubresource(),
			ResourceRequest: a.IsResourceRequest(),
			Path:            a.GetPath(),
		}

		newObjs := []runtime.Object{}
		for _, obj := range objs {
			m, err := meta.Accessor(obj)
			if err != nil {
				return nil, err
			}

			attributes.Name = m.GetName()
			if nameAsNamespace {
				attributes.Namespace = attributes.Name
			}
			// TODO: change because group version is different?
			attributes.Path = a.GetPath() + "/" + m.GetName()

			d, _, err := f.authorizer.Authorize(ctx, attributes)
			if err != nil {
				return nil, err
			} else if d == authorizer.DecisionAllow {
				newObjs = append(newObjs, obj)
			}
		}

		sort.Slice(newObjs, func(i int, j int) bool {
			ni, _ := meta.Accessor(newObjs[i])
			nj, _ := meta.Accessor(newObjs[j])
			return ni.GetName() < nj.GetName()
		})

		err = meta.SetList(list, newObjs)
		if err != nil {
			return nil, err
		}
	}

	return list, nil
}
