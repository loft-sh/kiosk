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

package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/go-logr/logr"
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	resyncPeriod = time.Hour * 10
)

// EnqueueSubject is the function that enqueues a subject to the work queue
type EnqueueSubject func(string)

// Cache is used to retrieve and cache mappings between users / groups and namespaces and accounts
type Cache interface {
	GetAccountsForUser(user user.Info, verb string) ([]string, error)
	GetNamespacesForUser(user user.Info, verb string) ([]string, error)

	GetAccounts(ctx context.Context, accounts []string) ([]*configv1alpha1.Account, error)
	GetNamespaces(ctx context.Context, namespaces []string) ([]*corev1.Namespace, error)

	Run(stop <-chan struct{})
}

type authCache struct {
	client client.Client
	cache  ctrlcache.Cache

	accessor              Accessor
	allowedNamespaceStore cache.ThreadSafeStore
	allowedAccountStore   cache.ThreadSafeStore

	accountInformer            ctrlcache.Informer
	roleInformer               ctrlcache.Informer
	roleBindingInformer        ctrlcache.Informer
	clusterRoleInformer        ctrlcache.Informer
	clusterRoleBindingInformer ctrlcache.Informer

	// Subjects that need to be synchronized
	queue workqueue.RateLimitingInterface
	log   logr.Logger
}

// Allowed holds the allowed resources for a certain subject
type Allowed struct {
	View   []string
	Create []string
	Update []string
	Delete []string
}

// NewAuthCache creates a new auth cache
func NewAuthCache(client client.Client, ctrlCache ctrlcache.Cache, log logr.Logger) (Cache, error) {
	// Get informers
	accountInformer, err := ctrlCache.GetInformer(&configv1alpha1.Account{})
	if err != nil {
		return nil, err
	}
	roleInformer, err := ctrlCache.GetInformer(&rbacv1.Role{})
	if err != nil {
		return nil, err
	}
	roleBindingInformer, err := ctrlCache.GetInformer(&rbacv1.RoleBinding{})
	if err != nil {
		return nil, err
	}
	clusterRoleInformer, err := ctrlCache.GetInformer(&rbacv1.ClusterRole{})
	if err != nil {
		return nil, err
	}
	clusterRoleBindingInformer, err := ctrlCache.GetInformer(&rbacv1.ClusterRoleBinding{})
	if err != nil {
		return nil, err
	}

	a := &authCache{
		client:                client,
		cache:                 ctrlCache,
		accessor:              &accessor{client: client},
		allowedNamespaceStore: cache.NewThreadSafeStore(map[string]cache.IndexFunc{}, map[string]cache.Index{}),
		allowedAccountStore:   cache.NewThreadSafeStore(map[string]cache.IndexFunc{}, map[string]cache.Index{}),

		accountInformer:            accountInformer,
		roleInformer:               roleInformer,
		roleBindingInformer:        roleBindingInformer,
		clusterRoleInformer:        clusterRoleInformer,
		clusterRoleBindingInformer: clusterRoleBindingInformer,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		log:   log,
	}

	a.registerEventHandler()
	registerIndices(ctrlCache)

	return a, nil
}

func (a *authCache) registerEventHandler() {
	// Account
	a.accountInformer.AddEventHandlerWithResyncPeriod(&AccountHandler{
		enqueue: a.enqueueSubject,
	}, resyncPeriod)

	// Role Binding
	a.roleBindingInformer.AddEventHandlerWithResyncPeriod(&RoleBindingHandler{
		enqueue: a.enqueueSubject,
	}, resyncPeriod)

	// Role
	a.roleInformer.AddEventHandlerWithResyncPeriod(&RoleHandler{
		client:  a.client,
		enqueue: a.enqueueSubject,
	}, resyncPeriod)

	// Cluster Role Binding
	a.clusterRoleBindingInformer.AddEventHandlerWithResyncPeriod(&ClusterRoleBindingHandler{
		enqueue: a.enqueueSubject,
	}, resyncPeriod)

	// Cluster Role
	a.clusterRoleInformer.AddEventHandlerWithResyncPeriod(&ClusterRoleHandler{
		client:  a.client,
		enqueue: a.enqueueSubject,
	}, resyncPeriod)
}

func (a *authCache) waitForCacheSync() error {
	err := wait.PollImmediate(100*time.Millisecond, time.Minute, func() (bool, error) {
		if !a.accountInformer.HasSynced() || !a.roleInformer.HasSynced() || !a.roleBindingInformer.HasSynced() || !a.clusterRoleInformer.HasSynced() || !a.clusterRoleBindingInformer.HasSynced() {
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		log.Println("Waiting for cache sync failed: " + err.Error())
	}

	return err
}

// TODO: Is this necessary and/or is there a better way of doing this?
func (a *authCache) waitForCache() error {
	err := wait.PollImmediate(100*time.Millisecond, time.Minute, func() (bool, error) {
		if a.queue.Len() > 0 {
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		log.Println("Waiting for cache sync failed: " + err.Error())
	}

	return err
}

// GetAccounts is a convienience method to retrieve the objects from the given account names
func (a *authCache) GetAccounts(ctx context.Context, accounts []string) ([]*configv1alpha1.Account, error) {
	if len(accounts) == 0 {
		return nil, nil
	}

	retList := []*configv1alpha1.Account{}

	// Should we get all?
	if accounts[0] == rbacv1.ResourceAll {
		accountList := &configv1alpha1.AccountList{}
		err := a.client.List(ctx, accountList)
		if err != nil {
			return nil, err
		}

		for _, account := range accountList.Items {
			accountCopy := account
			retList = append(retList, &accountCopy)
		}
	} else {
		for _, account := range accounts {
			accountCopy := &configv1alpha1.Account{}
			err := a.client.Get(ctx, types.NamespacedName{Name: account}, accountCopy)
			if err != nil {
				if kerrors.IsNotFound(err) {
					continue
				}

				return nil, err
			}

			retList = append(retList, accountCopy)
		}
	}

	return retList, nil
}

// GetNamespaces is a convienience method to retrieve the objects from the given namespace names
func (a *authCache) GetNamespaces(ctx context.Context, namespaces []string) ([]*corev1.Namespace, error) {
	if len(namespaces) == 0 {
		return nil, nil
	}

	retList := []*corev1.Namespace{}

	// Should we get all?
	if namespaces[0] == rbacv1.ResourceAll {
		namespaceList := &corev1.NamespaceList{}
		err := a.client.List(ctx, namespaceList)
		if err != nil {
			return nil, err
		}

		for _, namespace := range namespaceList.Items {
			namespaceCopy := namespace
			retList = append(retList, &namespaceCopy)
		}
	} else {
		for _, namespace := range namespaces {
			namespaceCopy := &corev1.Namespace{}
			err := a.client.Get(ctx, types.NamespacedName{Name: namespace}, namespaceCopy)
			if err != nil {
				if kerrors.IsNotFound(err) {
					continue
				}

				return nil, err
			}

			retList = append(retList, namespaceCopy)
		}
	}

	return retList, nil
}

func (a *authCache) enqueueSubject(subject string) {
	a.queue.Add(&subject)
}

func (a *authCache) invalidateSubjectNamespaceCache(subject string) error {
	ctx := context.Background()
	viewNamespaces, err := a.accessor.RetrieveAllowedNamespaces(ctx, subject, "get")
	if err != nil {
		return err
	}
	createNamespaces, err := a.accessor.RetrieveAllowedNamespaces(ctx, subject, "create")
	if err != nil {
		return err
	}
	updateNamespaces, err := a.accessor.RetrieveAllowedNamespaces(ctx, subject, "update")
	if err != nil {
		return err
	}
	deleteNamespaces, err := a.accessor.RetrieveAllowedNamespaces(ctx, subject, "delete")
	if err != nil {
		return err
	}

	// If the user has no allowed namespaces we delete him from the cache
	a.addToStore(a.allowedNamespaceStore, subject, &Allowed{
		View:   viewNamespaces,
		Create: createNamespaces,
		Update: updateNamespaces,
		Delete: deleteNamespaces,
	})
	return nil
}

func (a *authCache) invalidateSubjectAccountCache(subject string) error {
	ctx := context.Background()
	viewAccounts, err := a.accessor.RetrieveAllowedAccounts(ctx, subject, "get")
	if err != nil {
		return err
	}
	createAccounts, err := a.accessor.RetrieveAllowedAccounts(ctx, subject, "create")
	if err != nil {
		return err
	}
	updateAccounts, err := a.accessor.RetrieveAllowedAccounts(ctx, subject, "update")
	if err != nil {
		return err
	}
	deleteAccounts, err := a.accessor.RetrieveAllowedAccounts(ctx, subject, "delete")
	if err != nil {
		return err
	}

	// If the user has no allowed accounts we delete him from the cache
	a.addToStore(a.allowedAccountStore, subject, &Allowed{
		View:   viewAccounts,
		Create: createAccounts,
		Update: updateAccounts,
		Delete: deleteAccounts,
	})
	return nil
}

func (a *authCache) addToStore(store cache.ThreadSafeStore, subject string, allowed *Allowed) {
	if len(allowed.View) == 0 && len(allowed.Create) == 0 && len(allowed.Update) == 0 && len(allowed.Delete) == 0 {
		store.Delete(subject)
		return
	}

	store.Add(subject, allowed)
}

func (a *authCache) Run(stopCh <-chan struct{}) {
	err := a.waitForCacheSync()
	if err != nil {
		a.log.Error(err, "run auth cache")
		return
	}

	// Execute the change loop
	wait.Until(a.runProcessCacheChanges, 1*time.Second, stopCh)
}

func (a *authCache) runProcessCacheChanges() {
	for a.processCacheChange() {
	}
}

func (a *authCache) processCacheChange() bool {
	subject, quit := a.queue.Get()
	if quit {
		return false
	}

	defer a.queue.Done(subject)
	subjectStr, ok := subject.(*string)
	if !ok {
		a.log.Error(fmt.Errorf("Object %v is not a string pointer", subject), "process cache change")
		return true
	}

	err := a.invalidateSubjectNamespaceCache(*subjectStr)
	if err != nil {
		a.log.Error(err, "invalidate subject "+*subjectStr+" namespace cache")
		return true
	}

	err = a.invalidateSubjectAccountCache(*subjectStr)
	if err != nil {
		a.log.Error(err, "invalidate subject "+*subjectStr+" account cache")
		return true
	}

	return true
}

func (a *authCache) getAllowedFromStore(subject string, store cache.ThreadSafeStore) (*Allowed, error) {
	if n, ok := store.Get(subject); ok {
		return n.(*Allowed), nil
	}

	return &Allowed{}, nil
}

func (a *authCache) GetAccountsForUser(user user.Info, verb string) ([]string, error) {
	return a.getAllowedFor(user, verb, a.allowedAccountStore)
}

func (a *authCache) GetNamespacesForUser(user user.Info, verb string) ([]string, error) {
	return a.getAllowedFor(user, verb, a.allowedNamespaceStore)
}

func (a *authCache) getAllowedFor(user user.Info, verb string, store cache.ThreadSafeStore) ([]string, error) {
	// Wait till the queue is empty
	err := a.waitForCache()
	if err != nil {
		return nil, err
	}

	// Gather subjects
	retNames := map[string]bool{}
	subjects := []string{UserPrefix + user.GetName()}
	for _, group := range user.GetGroups() {
		subjects = append(subjects, GroupPrefix+group)
	}

	for _, subject := range subjects {
		allowed, err := a.getAllowedFromStore(subject, store)
		if err != nil {
			return nil, err
		}

		list := []string{}
		if verb == "get" || verb == "list" || verb == "watch" {
			list = allowed.View
		} else if verb == "create" {
			list = allowed.Create
		} else if verb == "update" {
			list = allowed.Update
		} else if verb == "delete" {
			list = allowed.Delete
		} else {
			return nil, errors.New("Verb is unrecognized: " + verb + ", must be one of: list,watch,get,create,update,delete")
		}

		for _, v := range list {
			retNames[v] = true
		}

		// Check if we can access all
		if _, ok := retNames[rbacv1.ResourceAll]; ok {
			return []string{rbacv1.ResourceAll}, nil
		}
	}

	retArray := make([]string, 0, len(retNames))
	for key := range retNames {
		if key == "" {
			continue
		}

		retArray = append(retArray, key)
	}

	return retArray, nil
}
