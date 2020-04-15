package authorization

import (
	"context"
	rbacregistryvalidation "github.com/kiosk-sh/kiosk/kube/pkg/registry/rbac/validation"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
)

type RuleClient interface {
	rbacregistryvalidation.RoleGetter
	rbacregistryvalidation.RoleBindingLister
	rbacregistryvalidation.ClusterRoleGetter
	rbacregistryvalidation.ClusterRoleBindingLister
}

type ruleClient struct {
	client client2.Client
}

func NewRuleClient(client client2.Client) RuleClient {
	return &ruleClient{client: client}
}

func (r *ruleClient) GetRole(namespace, name string) (*rbacv1.Role, error) {
	role := &rbacv1.Role{}
	err := r.client.Get(context.Background(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, role)
	if err != nil {
		return nil, err
	}

	return role, nil
}

func (r *ruleClient) ListRoleBindings(namespace string) ([]*rbacv1.RoleBinding, error) {
	list := &rbacv1.RoleBindingList{}
	err := r.client.List(context.Background(), list, client2.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	// convert list
	ret := make([]*rbacv1.RoleBinding, len(list.Items))
	for i, r := range list.Items {
		c := r
		ret[i] = &c
	}

	return ret, nil
}

func (r *ruleClient) GetClusterRole(name string) (*rbacv1.ClusterRole, error) {
	role := &rbacv1.ClusterRole{}
	err := r.client.Get(context.Background(), types.NamespacedName{
		Name: name,
	}, role)
	if err != nil {
		return nil, err
	}

	return role, nil
}

func (r *ruleClient) ListClusterRoleBindings() ([]*rbacv1.ClusterRoleBinding, error) {
	list := &rbacv1.ClusterRoleBindingList{}
	err := r.client.List(context.Background(), list)
	if err != nil {
		return nil, err
	}

	// convert list
	ret := make([]*rbacv1.ClusterRoleBinding, len(list.Items))
	for i, r := range list.Items {
		c := r
		ret[i] = &c
	}

	return ret, nil
}
