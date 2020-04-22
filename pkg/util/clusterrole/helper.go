package clusterrole

import (
	"context"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SyncClusterRoleBindings(ctx context.Context, client client.Client, clusterRoleBindings []rbacv1.ClusterRoleBinding, clusterRoleName string, subjects []rbacv1.Subject, deleteExcess bool) ([]rbacv1.ClusterRoleBinding, error) {
	// Delete bindings with wrong ref & excess
	bindings := []rbacv1.ClusterRoleBinding{}
	for _, crb := range clusterRoleBindings {
		if crb.RoleRef.Name != clusterRoleName || crb.RoleRef.Kind != "ClusterRole" {
			err := client.Delete(ctx, &crb)
			if err != nil {
				return nil, err
			}

			continue
		}
		if deleteExcess && len(bindings) > 0 {
			err := client.Delete(ctx, &crb)
			if err != nil {
				return nil, err
			}

			continue
		}

		// Update cluster role binding
		if apiequality.Semantic.DeepEqual(crb.Subjects, subjects) == false {
			crb.Subjects = subjects
			err := client.Update(ctx, &crb)
			if err != nil {
				return nil, err
			}

			klog.Info("updated cluster role binding " + crb.Name)
		}

		bindings = append(bindings, crb)
	}

	return bindings, nil
}

func SyncClusterRoles(ctx context.Context, client client.Client, clusterRoles []rbacv1.ClusterRole, rules []rbacv1.PolicyRule, deleteExcess bool) ([]rbacv1.ClusterRole, error) {
	roles := []rbacv1.ClusterRole{}
	for _, clusterRole := range clusterRoles {
		// Delete excess cluster roles
		if deleteExcess && len(roles) > 0 {
			err := client.Delete(ctx, &clusterRole)
			if err != nil {
				return nil, err
			}

			klog.Info("deleted cluster role " + clusterRole.Name)
			continue
		}

		if apiequality.Semantic.DeepEqual(clusterRole.Rules, rules) == false {
			clusterRole.Rules = rules
			err := client.Update(ctx, &clusterRole)
			if err != nil {
				return nil, err
			}

			klog.Info("updated cluster role " + clusterRole.Name)
		}

		roles = append(roles, clusterRole)
	}

	return roles, nil
}
