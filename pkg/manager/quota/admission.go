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
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/kiosk-sh/kiosk/pkg/manager/controllers"
	"github.com/kiosk-sh/kiosk/pkg/util"

	corev1 "k8s.io/api/core/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"

	quota "github.com/kiosk-sh/kiosk/kube/pkg/quota/v1"
	"github.com/kiosk-sh/kiosk/kube/pkg/quota/v1/generic"
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"

	"github.com/kiosk-sh/kiosk/kube/plugin/pkg/admission/resourcequota"
	resourcequotaapi "github.com/kiosk-sh/kiosk/kube/plugin/pkg/admission/resourcequota/apis/resourcequota"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// accountQuotaAdmission implements an admission controller that can enforce accountQuota constraints
type accountQuotaAdmission struct {
	*admission.Handler

	// these are used to create the accessor
	client client.Client
	cache  cache.Cache
	config quota.Configuration

	lockFactory util.LockFactory

	init      sync.Once
	evaluator resourcequota.Evaluator
}

var _ admission.ValidationInterface = &accountQuotaAdmission{}

const (
	timeToWaitForCacheSync = 10 * time.Second
	numEvaluatorThreads    = 10
)

// NewAccountResourceQuota configures an admission controller that can enforce accountQuota constraints
// using the provided registry.  The registry must have the capability to handle group/kinds that
// are persisted by the server this admission controller is intercepting
func NewAccountResourceQuota(ctrlCtx *controllers.Context) admission.ValidationInterface {
	return &accountQuotaAdmission{
		Handler: admission.NewHandler(admission.Create, admission.Update),

		config:      NewQuotaConfiguration(generic.ListerFuncForResourceFunc(ctrlCtx.SharedInformers.ForResource)),
		client:      ctrlCtx.Manager.GetClient(),
		cache:       ctrlCtx.Manager.GetCache(),
		lockFactory: util.NewDefaultLockFactory(),
	}
}

// Admit makes admission decisions while enforcing clusterQuota
func (q *accountQuotaAdmission) Validate(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) (err error) {
	// ignore all operations that correspond to sub-resource actions
	if len(a.GetSubresource()) != 0 {
		return nil
	}
	// ignore cluster level resources
	if len(a.GetNamespace()) == 0 {
		return nil
	}

	if !q.waitForSyncedStore(time.After(timeToWaitForCacheSync)) {
		return admission.NewForbidden(a, errors.New("caches not synchronized"))
	}

	q.init.Do(func() {
		accountQuotaAccessor := newQuotaAccessor(q.client)
		q.evaluator = resourcequota.NewQuotaEvaluator(accountQuotaAccessor, q.config.IgnoredResources(), generic.NewRegistry(q.config.Evaluators()), q.lockAquisition, &resourcequotaapi.Configuration{}, numEvaluatorThreads, utilwait.NeverStop)
	})

	return q.evaluator.Evaluate(a)
}

func (q *accountQuotaAdmission) lockAquisition(quotas []corev1.ResourceQuota) func() {
	locks := []sync.Locker{}

	// acquire the locks in alphabetical order because I'm too lazy to think of something clever
	sort.Sort(ByName(quotas))
	for _, quota := range quotas {
		lock := q.lockFactory.GetLock(quota.Name)
		lock.Lock()
		locks = append(locks, lock)
	}

	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
		}
	}
}

func (q *accountQuotaAdmission) waitForSyncedStore(timeout <-chan time.Time) bool {
	namespaceInformer, err := q.cache.GetInformer(&corev1.Namespace{})
	if err != nil {
		return false
	}

	accountQuotaInformer, err := q.cache.GetInformer(&configv1alpha1.AccountQuota{})
	if err != nil {
		return false
	}

	for !namespaceInformer.HasSynced() || !accountQuotaInformer.HasSynced() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			return namespaceInformer.HasSynced() && accountQuotaInformer.HasSynced()
		}
	}

	return true
}

type ByName []corev1.ResourceQuota

func (v ByName) Len() int           { return len(v) }
func (v ByName) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v ByName) Less(i, j int) bool { return v[i].Name < v[j].Name }
