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

package quota

import (
	"context"
	"time"

	lru "github.com/hashicorp/golang-lru"

	utilquota "github.com/kiosk-sh/kiosk/kube/pkg/quota/v1"
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"

	corev1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	etcd "k8s.io/apiserver/pkg/storage/etcd3"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type accountQuotaAccessor struct {
	client client.Client

	// updatedAccountQuotas holds a cache of quotas that we've updated.  This is used to pull the "really latest" during back to
	// back quota evaluations that touch the same quota doc.  This only works because we can compare etcd resourceVersions
	// for the same resource as integers.  Before this change: 22 updates with 12 conflicts.  after this change: 15 updates with 0 conflicts
	updatedAccountQuotas *lru.Cache
}

// newQuotaAccessor creates an object that conforms to the QuotaAccessor interface to be used to retrieve quota objects.
func newQuotaAccessor(client client.Client) *accountQuotaAccessor {
	updatedCache, err := lru.New(100)
	if err != nil {
		// this should never happen
		panic(err)
	}

	return &accountQuotaAccessor{
		client:               client,
		updatedAccountQuotas: updatedCache,
	}
}

// UpdateQuotaStatus the newQuota coming in will be incremented from the original.  The difference between the original
// and the new is the amount to add to the namespace total, but the total status is the used value itself
func (e *accountQuotaAccessor) UpdateQuotaStatus(newQuota *corev1.ResourceQuota) error {
	accountQuota := &configv1alpha1.AccountQuota{}
	ctx := context.Background()
	err := e.client.Get(ctx, types.NamespacedName{Name: newQuota.Name}, accountQuota)
	if err != nil {
		return err
	}
	accountQuota = e.checkCache(accountQuota)

	// re-assign objectmeta
	// make a copy
	accountQuota = accountQuota.DeepCopy()
	accountQuota.ObjectMeta = newQuota.ObjectMeta
	accountQuota.Namespace = ""

	// determine change in usage
	usageDiff := utilquota.Subtract(newQuota.Status.Used, accountQuota.Status.Total.Used)

	// update aggregate usage
	accountQuota.Status.Total.Used = newQuota.Status.Used

	// update per namespace totals
	oldNamespaceTotals, _ := GetResourceQuotasStatusByNamespace(accountQuota.Status.Namespaces, newQuota.Namespace)
	namespaceTotalCopy := oldNamespaceTotals.DeepCopy()
	newNamespaceTotals := *namespaceTotalCopy
	newNamespaceTotals.Used = utilquota.Add(oldNamespaceTotals.Used, usageDiff)
	InsertResourceQuotasStatus(&accountQuota.Status.Namespaces, configv1alpha1.AccountQuotaStatusByNamespace{
		Namespace: newQuota.Namespace,
		Status:    newNamespaceTotals,
	})

	err = e.client.Status().Update(ctx, accountQuota)
	if err != nil {
		return err
	}

	e.updatedAccountQuotas.Add(accountQuota.Name, accountQuota)
	return nil
}

var etcdVersioner = etcd.APIObjectVersioner{}

// checkCache compares the passed quota against the value in the look-aside cache and returns the newer
// if the cache is out of date, it deletes the stale entry.  This only works because of etcd resourceVersions
// being monotonically increasing integers
func (e *accountQuotaAccessor) checkCache(accountQuota *configv1alpha1.AccountQuota) *configv1alpha1.AccountQuota {
	uncastCachedQuota, ok := e.updatedAccountQuotas.Get(accountQuota.Name)
	if !ok {
		return accountQuota
	}

	cachedQuota := uncastCachedQuota.(*configv1alpha1.AccountQuota)
	if etcdVersioner.CompareResourceVersion(accountQuota, cachedQuota) >= 0 {
		e.updatedAccountQuotas.Remove(accountQuota.Name)
		return accountQuota
	}

	return cachedQuota
}

func (e *accountQuotaAccessor) GetQuotas(namespaceName string) ([]corev1.ResourceQuota, error) {
	accountQuotas, err := e.waitForReadyAccountQuotaNames(namespaceName)
	if err != nil {
		return nil, err
	}

	resourceQuotas := []corev1.ResourceQuota{}
	for _, accountQuota := range accountQuotas {
		accountQuota = e.checkCache(accountQuota)

		// now convert to a ResourceQuota
		convertedQuota := corev1.ResourceQuota{}
		convertedQuota.ObjectMeta = accountQuota.ObjectMeta
		convertedQuota.Namespace = namespaceName
		convertedQuota.Spec = accountQuota.Spec.Quota
		convertedQuota.Status = accountQuota.Status.Total
		resourceQuotas = append(resourceQuotas, convertedQuota)
	}

	return resourceQuotas, nil
}

func (e *accountQuotaAccessor) waitForReadyAccountQuotaNames(namespaceName string) ([]*configv1alpha1.AccountQuota, error) {
	var accountQuotas []*configv1alpha1.AccountQuota
	// wait for a valid mapping cache.  The overall response can be delayed for up to 10 seconds.
	err := utilwait.PollImmediate(100*time.Millisecond, 8*time.Second, func() (bool, error) {
		// if we can't find the namespace yet, just wait for the cache to update.  Requests to non-existent namespaces
		// may hang, but those people are doing something wrong and namespacelifecycle should reject them.
		namespace := &corev1.Namespace{}
		err := e.client.Get(context.Background(), types.NamespacedName{Name: namespaceName}, namespace)
		if kapierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}

		// Check if namespace belongs to an account
		if namespace.Labels == nil || namespace.Labels[constants.SpaceLabelAccount] == "" {
			return true, nil
		}

		// Now list the account quotas for the namespace
		accountName := namespace.Labels[constants.SpaceLabelAccount]
		accountQuotaList := &configv1alpha1.AccountQuotaList{}
		err = e.client.List(context.Background(), accountQuotaList, client.MatchingFields{constants.IndexByAccount: accountName})
		if err != nil {
			return false, err
		}

		accountQuotas = make([]*configv1alpha1.AccountQuota, 0, len(accountQuotaList.Items))
		for _, i := range accountQuotaList.Items {
			cpy := i
			accountQuotas = append(accountQuotas, &cpy)
		}

		// Everything is good we can return to the caller
		return true, nil
	})
	return accountQuotas, err
}
