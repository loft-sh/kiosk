package metahelper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func ReplaceManagedFieldsApiVersion(obj metav1.Object, gv schema.GroupVersion) {
	managedFields := obj.GetManagedFields()
	apiVersion := gv.String()
	for i := range managedFields {
		managedFields[i].APIVersion = apiVersion
	}
	obj.SetManagedFields(managedFields)
}
