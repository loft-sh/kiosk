package controllers

import (
	"github.com/kiosk-sh/kiosk/kube/pkg/controller"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Context holds informations for native kubernetes controllers that need access to a sharedindexfactory
type Context struct {
	// The controller runtime manager
	Manager manager.Manager

	// The global stop chan
	StopChan <-chan struct{}

	// SharedInformers is a generic shared informer factory that is used by multiple controller together, informers
	// stored here are different from informers in the controller-runtime manager, which should be preferred if possible
	SharedInformers informers.SharedInformerFactory

	// ObjectOrMetadataInformers creates generic informers for each group version resource
	ObjectOrMetadataInformers controller.InformerFactory

	// InformersStarted is closed as soon as the informer factories were started
	InformersStarted chan struct{}

	// DiscoveryFunc is able to discover available api resources in the cluster
	DiscoveryFunc func() ([]*metav1.APIResourceList, error)
}

// ResyncPeriod returns a function which generates a duration each time it is
// invoked; this is so that multiple controllers don't get into lock-step and all
// hammer the apiserver with list requests simultaneously.
func ResyncPeriod() func() time.Duration {
	duration := 12 * time.Hour

	return func() time.Duration {
		factor := rand.Float64() + 1
		return time.Duration(float64(duration) * factor)
	}
}

// NewControllerContext creates a new controller context
func NewControllerContext(mgr manager.Manager, stopChan <-chan struct{}) *Context {
	kubeConfig := mgr.GetConfig()

	versionedClient := clientset.NewForConfigOrDie(kubeConfig)
	sharedInformers := informers.NewSharedInformerFactory(versionedClient, ResyncPeriod()())

	metadataClient := metadata.NewForConfigOrDie(kubeConfig)
	metadataInformers := metadatainformer.NewSharedInformerFactory(metadataClient, ResyncPeriod()())

	objectOrMetadataInformer := controller.NewInformerFactory(sharedInformers, metadataInformers)
	discoveryFunc := versionedClient.Discovery().ServerPreferredNamespacedResources

	return &Context{
		Manager:                   mgr,
		SharedInformers:           sharedInformers,
		StopChan:                  stopChan,
		ObjectOrMetadataInformers: objectOrMetadataInformer,
		InformersStarted:          make(chan struct{}),
		DiscoveryFunc:             discoveryFunc,
	}
}

// Start starts the informer factories
func (c *Context) Start() {
	// Start the informer factories
	c.SharedInformers.Start(c.StopChan)
	c.ObjectOrMetadataInformers.Start(c.StopChan)
	close(c.InformersStarted)

	select {}
}
