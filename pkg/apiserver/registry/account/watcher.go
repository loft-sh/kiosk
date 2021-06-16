package account

import (
	"context"
	configv1alpha1 "github.com/loft-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/loft-sh/kiosk/pkg/apis/tenancy"
	kioskwatch "github.com/loft-sh/kiosk/pkg/watch"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog"
	"sync"
)

var _ kioskwatch.Watcher = &watcher{}

type watcher struct {
	userInfo      user.Info
	labelSelector labels.Selector
	authorizer    authorizer.Authorizer

	result  chan watch.Event
	stopped bool
	sync.RWMutex
}

func (w *watcher) Observe(event watch.Event) {
	configAccount, ok := event.Object.(*configv1alpha1.Account)
	if !ok {
		return
	}

	tenancyAccount, err := ConvertConfigAccount(configAccount)
	if err != nil {
		klog.Infof("Error converting config account: %v", err)
		return
	}

	event.Object = tenancyAccount
	decision, _, err := w.authorizer.Authorize(context.Background(), authorizer.AttributesRecord{
		User:            w.userInfo,
		ResourceRequest: true,
		Verb:            "get",
		APIGroup:        tenancy.SchemeGroupVersion.Group,
		APIVersion:      "v1alpha1",
		Resource:        "accounts",
		Name:            tenancyAccount.Name,
	})
	if err != nil || decision != authorizer.DecisionAllow {
		return
	}

	// check label selector
	if w.labelSelector != nil && w.labelSelector.Matches(labels.Set(tenancyAccount.Labels)) == false {
		return
	}

	// send event
	w.RLock()
	defer w.RUnlock()

	w.result <- event
}

func (w *watcher) Stop() {
	w.Lock()
	defer w.Unlock()
	if !w.stopped {
		klog.Infof("Stop watching accounts for " + w.userInfo.GetName())
		kioskwatch.AccountRegistry.Unsubscribe(w)
		close(w.result)
		w.stopped = true
	}
}

func (w *watcher) ResultChan() <-chan watch.Event {
	return w.result
}
