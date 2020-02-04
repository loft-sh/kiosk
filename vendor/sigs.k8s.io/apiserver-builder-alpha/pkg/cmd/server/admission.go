package server

import (
	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/rest"
)

var (
	AggregatedAdmissionInitializerGetter func(config *rest.Config) (admission.PluginInitializer, genericapiserver.PostStartHookFunc)
	AggregatedAdmissionPlugins           = make(map[string]admission.Interface)
)
