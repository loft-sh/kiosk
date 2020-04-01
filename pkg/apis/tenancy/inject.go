package tenancy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Scheme will be injected during startup and then passed to the rest storages
var Scheme *runtime.Scheme

// Client will be injected during startup and then passed to the rest storages
var Client client.Client
