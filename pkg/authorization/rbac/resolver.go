/*
Copyright 2016 The Kubernetes Authors.
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

package rbac

import (
	"context"
	"fmt"
	"github.com/loft-sh/kiosk/pkg/constants"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
)

type DefaultRuleResolver struct {
	ListAll bool
	Client client.Client
}

func NewDefaultRuleResolver(client client.Client) *DefaultRuleResolver {
	return &DefaultRuleResolver{Client: client}
}

func describeSubject(s *rbacv1.Subject, bindingNamespace string) string {
	switch s.Kind {
	case rbacv1.ServiceAccountKind:
		if len(s.Namespace) > 0 {
			return fmt.Sprintf("%s %q", s.Kind, s.Name+"/"+s.Namespace)
		}
		return fmt.Sprintf("%s %q", s.Kind, s.Name+"/"+bindingNamespace)
	default:
		return fmt.Sprintf("%s %q", s.Kind, s.Name)
	}
}

type clusterRoleBindingDescriber struct {
	binding *rbacv1.ClusterRoleBinding
	subject *rbacv1.Subject
}

func (d *clusterRoleBindingDescriber) String() string {
	return fmt.Sprintf("ClusterRoleBinding %q of %s %q to %s",
		d.binding.Name,
		d.binding.RoleRef.Kind,
		d.binding.RoleRef.Name,
		describeSubject(d.subject, ""),
	)
}

type roleBindingDescriber struct {
	binding *rbacv1.RoleBinding
	subject *rbacv1.Subject
}

func (d *roleBindingDescriber) String() string {
	return fmt.Sprintf("RoleBinding %q of %s %q to %s",
		d.binding.Name+"/"+d.binding.Namespace,
		d.binding.RoleRef.Kind,
		d.binding.RoleRef.Name,
		describeSubject(d.subject, d.binding.Namespace),
	)
}

func (r *DefaultRuleResolver) VisitRulesFor(ctx context.Context, user user.Info, namespace string, visitor func(source fmt.Stringer, rule *rbacv1.PolicyRule, err error) bool) {
	if clusterRoleBindings, err := r.listClusterRoleBindings(ctx, user); err != nil {
		if !visitor(nil, nil, err) {
			return
		}
	} else {
		sourceDescriber := &clusterRoleBindingDescriber{}
		for _, clusterRoleBinding := range clusterRoleBindings {
			subjectIndex, applies := appliesTo(user, clusterRoleBinding.Subjects, "")
			if !applies {
				continue
			}
			rules, err := r.GetRoleReferenceRules(ctx, clusterRoleBinding.RoleRef, "")
			if err != nil {
				if !visitor(nil, nil, err) {
					return
				}
				continue
			}
			sourceDescriber.binding = clusterRoleBinding
			sourceDescriber.subject = &clusterRoleBinding.Subjects[subjectIndex]
			for i := range rules {
				if !visitor(sourceDescriber, &rules[i], nil) {
					return
				}
			}
		}
	}

	if len(namespace) > 0 {
		if roleBindings, err := r.listRoleBindings(ctx, user, namespace); err != nil {
			if !visitor(nil, nil, err) {
				return
			}
		} else {
			sourceDescriber := &roleBindingDescriber{}
			for _, roleBinding := range roleBindings {
				subjectIndex, applies := appliesTo(user, roleBinding.Subjects, namespace)
				if !applies {
					continue
				}
				rules, err := r.GetRoleReferenceRules(ctx, roleBinding.RoleRef, namespace)
				if err != nil {
					if !visitor(nil, nil, err) {
						return
					}
					continue
				}
				sourceDescriber.binding = roleBinding
				sourceDescriber.subject = &roleBinding.Subjects[subjectIndex]
				for i := range rules {
					if !visitor(sourceDescriber, &rules[i], nil) {
						return
					}
				}
			}
		}
	}
}

func (r *DefaultRuleResolver) listClusterRoleBindings(ctx context.Context, user user.Info) ([]*rbacv1.ClusterRoleBinding, error) {
	retBindings := []*rbacv1.ClusterRoleBinding{}
	if r.ListAll {
		roleBindings := &rbacv1.ClusterRoleBindingList{}
		err := r.Client.List(ctx, roleBindings)
		if err != nil {
			return nil, err
		}

		for _, crb := range roleBindings.Items {
			cpy := crb
			retBindings = append(retBindings, &cpy)
		}

		return retBindings, nil
	}
	
	subjects := []string{"user:" + user.GetName()}
	for _, group := range user.GetGroups() {
		subjects = append(subjects, "group:"+group)
	}

	for _, subj := range subjects {
		clusterRoleBindings := &rbacv1.ClusterRoleBindingList{}
		err := r.Client.List(ctx, clusterRoleBindings, client.MatchingFields{constants.IndexBySubjects: subj})
		if err != nil {
			return nil, err
		}

		for _, crb := range clusterRoleBindings.Items {
			cpy := crb
			retBindings = append(retBindings, &cpy)
		}
	}

	return retBindings, nil
}

func (r *DefaultRuleResolver) listRoleBindings(ctx context.Context, user user.Info, namespace string) ([]*rbacv1.RoleBinding, error) {
	retBindings := []*rbacv1.RoleBinding{}
	if r.ListAll {
		roleBindings := &rbacv1.RoleBindingList{}
		err := r.Client.List(ctx, roleBindings, client.InNamespace(namespace))
		if err != nil {
			return nil, err
		}

		for _, crb := range roleBindings.Items {
			cpy := crb
			retBindings = append(retBindings, &cpy)
		}
		
		return retBindings, nil
	}
	
	subjects := []string{"user:" + user.GetName()}
	for _, group := range user.GetGroups() {
		subjects = append(subjects, "group:"+group)
	}

	for _, subj := range subjects {
		roleBindings := &rbacv1.RoleBindingList{}
		err := r.Client.List(ctx, roleBindings, client.InNamespace(namespace), client.MatchingFields{constants.IndexBySubjects: subj})
		if err != nil {
			return nil, err
		}

		for _, crb := range roleBindings.Items {
			cpy := crb
			retBindings = append(retBindings, &cpy)
		}
	}

	return retBindings, nil
}

// GetRoleReferenceRules attempts to resolve the RoleBinding or ClusterRoleBinding.
func (r *DefaultRuleResolver) GetRoleReferenceRules(ctx context.Context, roleRef rbacv1.RoleRef, bindingNamespace string) ([]rbacv1.PolicyRule, error) {
	switch roleRef.Kind {
	case "Role":
		role := &rbacv1.Role{}
		err := r.Client.Get(ctx, types.NamespacedName{Namespace: bindingNamespace, Name: roleRef.Name}, role)
		if err != nil {
			return nil, err
		}
		return role.Rules, nil

	case "ClusterRole":
		clusterRole := &rbacv1.ClusterRole{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: roleRef.Name}, clusterRole)
		if err != nil {
			return nil, err
		}
		return clusterRole.Rules, nil

	default:
		return nil, fmt.Errorf("unsupported role reference kind: %q", roleRef.Kind)
	}
}

// appliesTo returns whether any of the bindingSubjects applies to the specified subject,
// and if true, the index of the first subject that applies
func appliesTo(user user.Info, bindingSubjects []rbacv1.Subject, namespace string) (int, bool) {
	for i, bindingSubject := range bindingSubjects {
		if appliesToUser(user, bindingSubject, namespace) {
			return i, true
		}
	}
	return 0, false
}

func appliesToUser(user user.Info, subject rbacv1.Subject, namespace string) bool {
	switch subject.Kind {
	case rbacv1.UserKind:
		return user.GetName() == subject.Name

	case rbacv1.GroupKind:
		return has(user.GetGroups(), subject.Name)

	case rbacv1.ServiceAccountKind:
		// default the namespace to namespace we're working in if its available.  This allows rolebindings that reference
		// SAs in th local namespace to avoid having to qualify them.
		saNamespace := namespace
		if len(subject.Namespace) > 0 {
			saNamespace = subject.Namespace
		}
		if len(saNamespace) == 0 {
			return false
		}
		// use a more efficient comparison for RBAC checking
		return serviceaccount.MatchesUsername(saNamespace, subject.Name, user.GetName())
	default:
		return false
	}
}

func has(set []string, ele string) bool {
	for _, s := range set {
		if s == ele {
			return true
		}
	}
	return false
}

// ConvertSubject converts the given subject into an unqiue id string
func ConvertSubject(namespace string, subject *rbacv1.Subject) string {
	switch subject.Kind {
	case rbacv1.UserKind:
		return "user:" + subject.Name

	case rbacv1.GroupKind:
		return "group:" + subject.Name

	case rbacv1.ServiceAccountKind:
		saNamespace := namespace
		if len(subject.Namespace) > 0 {
			saNamespace = subject.Namespace
		}
		if len(saNamespace) == 0 {
			return ""
		}

		return "user:" + serviceaccount.MakeUsername(saNamespace, subject.Name)
	default:
	}

	return ""
}
