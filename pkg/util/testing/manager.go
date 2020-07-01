package testing

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ manager.Manager = &FakeManager{}

type FakeManager struct {
	Scheme *runtime.Scheme
	Client client.Client
	Cache  *FakeInformers
	Mapper meta.RESTMapper
}

func NewFakeManager() *FakeManager {
	scheme := NewScheme()
	return &FakeManager{
		Scheme: scheme,
		Client: NewFakeClient(scheme),
		Cache:  NewFakeCache(scheme),
		Mapper: NewFakeMapper(scheme),
	}
}

func (fm *FakeManager) AddMetricsExtraHandler(path string, handler http.Handler) error {
	return nil
}
func (fm *FakeManager) Elected() <-chan struct{} {
	return nil
}
func (fm *FakeManager) Add(manager.Runnable) error {
	return nil
}
func (fm *FakeManager) SetFields(interface{}) error {
	return nil
}
func (fm *FakeManager) AddHealthzCheck(name string, check healthz.Checker) error {
	return nil
}
func (fm *FakeManager) AddReadyzCheck(name string, check healthz.Checker) error {
	return nil
}
func (fm *FakeManager) Start(<-chan struct{}) error {
	return nil
}
func (fm *FakeManager) GetConfig() *rest.Config {
	return nil
}
func (fm *FakeManager) GetScheme() *runtime.Scheme {
	return fm.Scheme
}
func (fm *FakeManager) GetClient() client.Client {
	return fm.Client
}
func (fm *FakeManager) GetFieldIndexer() client.FieldIndexer {
	return fm.Cache
}
func (fm *FakeManager) GetCache() cache.Cache {
	return fm.Cache
}
func (fm *FakeManager) GetEventRecorderFor(name string) record.EventRecorder {
	return nil
}
func (fm *FakeManager) GetRESTMapper() meta.RESTMapper {
	return fm.Mapper
}
func (fm *FakeManager) GetAPIReader() client.Reader {
	return fm.Client
}
func (fm *FakeManager) GetWebhookServer() *webhook.Server {
	return nil
}
