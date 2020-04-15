package util

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

func ChangeAttributesResource(a authorizer.Attributes, groupVersionResource schema.GroupVersionResource, namespace string) authorizer.Attributes {
	return authorizer.AttributesRecord{
		User:            a.GetUser(),
		Verb:            a.GetVerb(),
		Name:            a.GetName(),
		Namespace:       namespace,
		APIGroup:        groupVersionResource.Group,
		APIVersion:      groupVersionResource.Version,
		Resource:        groupVersionResource.Resource,
		Subresource:     a.GetSubresource(),
		ResourceRequest: a.IsResourceRequest(),
		// TODO: change path as well
		Path: a.GetPath(),
	}
}
