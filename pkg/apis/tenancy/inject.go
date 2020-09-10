package tenancy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Scheme will be injected during startup and then passed to the rest storages
var Scheme *runtime.Scheme

// CachedClient will be injected during startup and then passed to the rest storages
var CachedClient client.Client

// UncachedClient will be injected during startup and then passed to the rest storages
var UncachedClient client.Client
