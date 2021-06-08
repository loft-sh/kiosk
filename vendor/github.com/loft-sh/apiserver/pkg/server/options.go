package apiserver

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/loft-sh/apiserver/pkg/admission"
	"github.com/loft-sh/apiserver/pkg/apiserver"
	"github.com/loft-sh/apiserver/pkg/builders"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	openapi "k8s.io/kube-openapi/pkg/common"
	"net/http"
	"net/url"
)

type ServerOptions struct {
	RecommendedOptions *genericoptions.RecommendedOptions
	APIBuilders        []*builders.APIGroupBuilder

	GetOpenAPIDefinitions openapi.GetOpenAPIDefinitions
	DisableWebhooks       bool
}

func (o *ServerOptions) Validate(args []string) error {
	return nil
}

func (o *ServerOptions) Complete() error {
	return nil
}

func (o *ServerOptions) GenericConfig(tweakConfig func(config *genericapiserver.RecommendedConfig) error) (*genericapiserver.RecommendedConfig, error) {
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, nil); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(builders.Codecs)
	loopbackKubeConfig, kubeInformerFactory, err := o.buildLoopback()
	if err != nil {
		klog.Warningf("attempting to instantiate loopback client but failed: %v", err)
	} else {
		serverConfig.LoopbackClientConfig = loopbackKubeConfig
		serverConfig.SharedInformerFactory = kubeInformerFactory
	}

	// admission webhooks
	if o.DisableWebhooks == false && serverConfig.LoopbackClientConfig != nil {
		proxyTransport := createNodeDialer()
		admissionConfig := &admission.Config{
			ExternalInformers:    kubeInformerFactory,
			LoopbackClientConfig: serverConfig.LoopbackClientConfig,
		}

		serviceResolver := buildServiceResolver(serverConfig.LoopbackClientConfig.Host, kubeInformerFactory)
		pluginInitializers, admissionPostStartHook, err := admissionConfig.New(proxyTransport, serverConfig.EgressSelector, serviceResolver)
		if err != nil {
			return nil, fmt.Errorf("failed to create admission plugin initializer: %v", err)
		}
		if err := serverConfig.AddPostStartHook("start-kube-apiserver-admission-initializer", admissionPostStartHook); err != nil {
			return nil, err
		}

		err = o.RecommendedOptions.Admission.ApplyTo(
			&serverConfig.Config,
			kubeInformerFactory,
			serverConfig.LoopbackClientConfig,
			utilfeature.DefaultFeatureGate,
			pluginInitializers...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize admission: %v", err)
		}
	}

	err = applyOptions(
		&serverConfig.Config,
		// o.RecommendedOptions.Etcd.ApplyTo,
		func(cfg *genericapiserver.Config) error {
			return o.RecommendedOptions.SecureServing.ApplyTo(&cfg.SecureServing, &cfg.LoopbackClientConfig)
		},
		func(cfg *genericapiserver.Config) error {
			return o.RecommendedOptions.Audit.ApplyTo(
				&serverConfig.Config,
			)
		},
		o.RecommendedOptions.Features.ApplyTo,
	)
	if err != nil {
		return nil, err
	}

	_ = o.RecommendedOptions.Authentication.ApplyTo(&serverConfig.Authentication, serverConfig.Config.SecureServing, serverConfig.Config.OpenAPIConfig)
	_ = o.RecommendedOptions.Authorization.ApplyTo(&serverConfig.Authorization)
	if tweakConfig != nil {
		if err := tweakConfig(serverConfig); err != nil {
			return nil, err
		}
	}

	return serverConfig, nil
}

func (o *ServerOptions) RunServer(APIServerVersion *version.Info, stopCh <-chan struct{}, authorizer authorizer.Authorizer, tweakServerConfig func(config *genericapiserver.RecommendedConfig) error) error {
	aggregatedAPIServerConfig, err := o.GenericConfig(tweakServerConfig)
	if err != nil {
		return err
	}

	// set the basics
	genericConfig := &aggregatedAPIServerConfig.Config
	genericConfig.Version = APIServerVersion
	genericConfig.Authorization.Authorizer = authorizer

	// set open api
	genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(o.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(builders.Scheme))
	genericConfig.OpenAPIConfig.Info.Title = "Api"
	genericConfig.OpenAPIConfig.Info.Version = "v0"
	if genericConfig.LongRunningFunc == nil {
		genericConfig.LongRunningFunc = genericfilters.BasicLongRunningRequestCheck(
			sets.NewString("watch", "proxy"),
			sets.NewString("attach", "exec", "proxy", "log", "portforward"),
		)
	}

	// create a new server
	genericServer, err := apiserver.NewServer(aggregatedAPIServerConfig, o.APIBuilders)
	if err != nil {
		return err
	}

	s := genericServer.GenericAPIServer.PrepareRun()
	return s.Run(stopCh)
}

func (o *ServerOptions) buildLoopback() (*rest.Config, informers.SharedInformerFactory, error) {
	var loopbackConfig *rest.Config
	var err error

	if len(o.RecommendedOptions.CoreAPI.CoreAPIKubeconfigPath) == 0 {
		klog.Infof("loading in-cluster loopback client...")
		loopbackConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, nil, err
		}
	} else {
		klog.Infof("loading out-of-cluster loopback client according to `--kubeconfig` settings...")
		loopbackConfig, err = clientcmd.BuildConfigFromFlags("", o.RecommendedOptions.CoreAPI.CoreAPIKubeconfigPath)
		if err != nil {
			return nil, nil, err
		}
	}

	loopbackClient, err := kubernetes.NewForConfig(loopbackConfig)
	if err != nil {
		return nil, nil, err
	}
	kubeInformerFactory := informers.NewSharedInformerFactory(loopbackClient, 0)
	return loopbackConfig, kubeInformerFactory, nil
}

type BufferedResponse struct {
	bytes.Buffer
}

func (BufferedResponse) Header() http.Header { return http.Header{} }
func (BufferedResponse) WriteHeader(int)     {}

func createNodeDialer() *http.Transport {
	// Setup nodeTunneler if needed
	var proxyDialerFn utilnet.DialFunc

	// Proxying to pods and services is IP-based... don't expect to be able to verify the hostname
	proxyTLSClientConfig := &tls.Config{InsecureSkipVerify: true}
	proxyTransport := utilnet.SetTransportDefaults(&http.Transport{
		DialContext:     proxyDialerFn,
		TLSClientConfig: proxyTLSClientConfig,
	})
	return proxyTransport
}

func buildServiceResolver(hostname string, informer informers.SharedInformerFactory) webhook.ServiceResolver {
	var serviceResolver webhook.ServiceResolver
	serviceResolver = aggregatorapiserver.NewClusterIPServiceResolver(
		informer.Core().V1().Services().Lister(),
	)

	// resolve kubernetes.default.svc locally
	if localHost, err := url.Parse(hostname); err == nil {
		serviceResolver = aggregatorapiserver.NewLoopbackServiceResolver(serviceResolver, localHost)
	}
	return serviceResolver
}
