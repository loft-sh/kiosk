package watch

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kubecache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

var (
	AccountRegistry   = NewRegistry()
	NamespaceRegistry = NewRegistry()
)

type Registry interface {
	Subscribe(watcher Watcher)
	Unsubscribe(watcher Watcher)
	Start(ctx context.Context, cache cache.Cache, obj client.Object) error
}

type Watcher interface {
	Observe(event watch.Event)
}

func NewRegistry() Registry {
	return &registry{watcher: []Watcher{}}
}

type registry struct {
	watcherMutex sync.RWMutex
	watcher      []Watcher
}

func (r *registry) Subscribe(watcher Watcher) {
	r.watcherMutex.Lock()
	defer r.watcherMutex.Unlock()

	r.watcher = append(r.watcher, watcher)
}

func (r *registry) Unsubscribe(watcher Watcher) {
	r.watcherMutex.Lock()
	defer r.watcherMutex.Unlock()

	newWatcher := []Watcher{}
	for _, w := range r.watcher {
		if w != watcher {
			newWatcher = append(newWatcher, w)
		}
	}

	r.watcher = newWatcher
}

func (r *registry) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}

	r.watcherMutex.RLock()
	defer r.watcherMutex.RUnlock()

	for _, w := range r.watcher {
		w.Observe(watch.Event{
			Type:   watch.Added,
			Object: runtimeObj,
		})
	}
}

func (r *registry) OnUpdate(oldObj, newObj interface{}) {
	runtimeObj, ok := newObj.(runtime.Object)
	if !ok {
		return
	}

	r.watcherMutex.RLock()
	defer r.watcherMutex.RUnlock()

	for _, w := range r.watcher {
		w.Observe(watch.Event{
			Type:   watch.Modified,
			Object: runtimeObj,
		})
	}
}

func (r *registry) OnDelete(obj interface{}) {
	if deletedFinalStateUnknown, ok := obj.(kubecache.DeletedFinalStateUnknown); ok {
		obj = deletedFinalStateUnknown.Obj
	}

	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}

	r.watcherMutex.RLock()
	defer r.watcherMutex.RUnlock()

	for _, w := range r.watcher {
		w.Observe(watch.Event{
			Type:   watch.Deleted,
			Object: runtimeObj,
		})
	}
}

func (r *registry) Start(ctx context.Context, cache cache.Cache, obj client.Object) error {
	inf, err := cache.GetInformer(ctx, obj)
	if err != nil {
		return err
	}

	inf.AddEventHandler(r)
	return nil
}
