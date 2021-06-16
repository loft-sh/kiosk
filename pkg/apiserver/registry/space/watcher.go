package space

import (
	"context"
	kioskwatch "github.com/loft-sh/kiosk/pkg/watch"
	corev1 "k8s.io/api/core/v1"
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
	namespace, ok := event.Object.(*corev1.Namespace)
	if !ok {
		return
	}

	space := ConvertNamespace(namespace)
	event.Object = space
	decision, _, err := w.authorizer.Authorize(context.Background(), authorizer.AttributesRecord{
		User:            w.userInfo,
		ResourceRequest: true,
		Verb:            "get",
		APIGroup:        corev1.SchemeGroupVersion.Group,
		APIVersion:      corev1.SchemeGroupVersion.Version,
		Resource:        "namespaces",
		Namespace:       space.Name,
		Name:            space.Name,
	})
	if err != nil || decision != authorizer.DecisionAllow {
		return
	}

	// check label selector
	if w.labelSelector != nil && w.labelSelector.Matches(labels.Set(space.Labels)) == false {
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
		klog.Infof("Stop watching spaces for " + w.userInfo.GetName())
		kioskwatch.NamespaceRegistry.Unsubscribe(w)
		close(w.result)
		w.stopped = true
	}
}

func (w *watcher) ResultChan() <-chan watch.Event {
	return w.result
}
