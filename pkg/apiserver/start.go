/*
Copyright 2016 The Kubernetes Authors.

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

package apiserver

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/kiosk-sh/kiosk/pkg/util/certhelper"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/server"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	openapi "k8s.io/kube-openapi/pkg/common"
	"sigs.k8s.io/apiserver-builder-alpha/pkg/apiserver"
	"sigs.k8s.io/apiserver-builder-alpha/pkg/builders"
	"sigs.k8s.io/apiserver-builder-alpha/pkg/validators"
)

var GetOpenApiDefinition openapi.GetOpenAPIDefinitions

type ServerOptions struct {
	RecommendedOptions     *genericoptions.RecommendedOptions
	APIBuilders            []*builders.APIGroupBuilder
	InsecureServingOptions *genericoptions.DeprecatedInsecureServingOptionsWithLoopback

	PrintBearerToken bool
	PrintOpenapi     bool
	RunDelegatedAuth bool
	BearerToken      string
	PostStartHooks   []PostStartHook
}

type PostStartHook struct {
	Fn   genericapiserver.PostStartHookFunc
	Name string
}

type StartOptions struct {
	EtcdPath         string
	Apis             []*builders.APIGroupBuilder
	Openapidefs      openapi.GetOpenAPIDefinitions
	Title            string
	Version          string
	TweakConfigFuncs []func(apiServer *apiserver.Config) error

	//FlagConfigFunc handles user-defined flags
	FlagConfigFuncs []func(*cobra.Command) error
}

// StartApiServer starts an apiserver hosting the provider apis and openapi definitions.
func StartApiServer(etcdPath string, apis []*builders.APIGroupBuilder, openapidefs openapi.GetOpenAPIDefinitions,
	title, version string, tweakConfigFuncs ...func(apiServer *apiserver.Config) error) error {
	return StartApiServerWithOptions(&StartOptions{
		EtcdPath: etcdPath,
		Apis:     apis,

		Openapidefs:      openapidefs,
		Title:            title,
		Version:          version,
		TweakConfigFuncs: tweakConfigFuncs,
	})
}

func StartApiServerWithOptions(opts *StartOptions) error {
	GetOpenApiDefinition = opts.Openapidefs

	signalCh := genericapiserver.SetupSignalHandler()
	// To disable providers, manually specify the list provided by getKnownProviders()
	cmd, _ := NewCommandStartServer(opts.EtcdPath, os.Stdout, os.Stderr, opts.Apis, signalCh,
		opts.Title, opts.Version, opts.TweakConfigFuncs...)

	errors := []error{}
	for _, ff := range opts.FlagConfigFuncs {
		if err := ff(cmd); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) != 0 {
		return utilerrors.NewAggregate(errors)
	}

	cmd.Flags().AddFlagSet(pflag.CommandLine)
	if err := cmd.Execute(); err != nil {
		return err
	}

	return nil
}

func NewServerOptions(etcdPath, title, version string, b []*builders.APIGroupBuilder) *ServerOptions {
	versions := []schema.GroupVersion{}
	for _, b := range b {
		versions = append(versions, b.GetLegacyCodec()...)
	}

	o := &ServerOptions{
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			etcdPath,
			builders.Codecs.LegacyCodec(versions...),
			genericoptions.NewProcessInfo(title, version),
		),
		APIBuilders:      b,
		RunDelegatedAuth: false,
	}

	// We don't use etcd
	o.RecommendedOptions.Etcd = nil
	o.RecommendedOptions.Admission = genericoptions.NewAdmissionOptions()
	o.RecommendedOptions.Admission.DefaultOffPlugins = sets.String{lifecycle.PluginName: sets.Empty{}}

	o.RecommendedOptions.SecureServing.ServerCert.CertKey.CertFile = filepath.Join(certhelper.APIServiceCertFolder, "tls.crt")
	o.RecommendedOptions.SecureServing.ServerCert.CertKey.KeyFile = filepath.Join(certhelper.APIServiceCertFolder, "tls.key")
	o.RecommendedOptions.SecureServing.BindPort = 8443

	o.RecommendedOptions.Authorization.RemoteKubeConfigFileOptional = true
	o.RecommendedOptions.Authentication.RemoteKubeConfigFileOptional = true
	o.InsecureServingOptions = func() *genericoptions.DeprecatedInsecureServingOptionsWithLoopback {
		o := genericoptions.DeprecatedInsecureServingOptions{}
		return o.WithLoopback()
	}()

	return o
}

// NewCommandStartMaster provides a CLI handler for 'start master' command
func NewCommandStartServer(etcdPath string, out, errOut io.Writer, builders []*builders.APIGroupBuilder,
	stopCh <-chan struct{}, title, version string, tweakConfigFuncs ...func(apiServer *apiserver.Config) error) (*cobra.Command, *ServerOptions) {
	o := NewServerOptions(etcdPath, title, version, builders)

	// for pluginName := range AggregatedAdmissionPlugins {
	//	o.RecommendedOptions.Admission.RecommendedPluginOrder = append(o.RecommendedOptions.Admission.RecommendedPluginOrder, pluginName)
	// }

	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	// Support overrides
	cmd := &cobra.Command{
		Short: "Launch an API server",
		Long:  "Launch an API server",
		RunE: func(c *cobra.Command, args []string) error {

			// TODO: remove it after upgrading to 1.13+
			// Sync the glog and klog flags.
			klogFlags.VisitAll(func(f *flag.Flag) {
				goFlag := flag.CommandLine.Lookup(f.Name)
				if goFlag != nil {
					goFlag.Value.Set(f.Value.String())
				}
			})

			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}

			if err := o.RunServer(stopCh, title, version, tweakConfigFuncs...); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&o.PrintBearerToken, "print-bearer-token", false, "Print a curl command with the bearer token to test the server")
	flags.BoolVar(&o.PrintOpenapi, "print-openapi", false, "Print the openapi json and exit")
	flags.BoolVar(&o.RunDelegatedAuth, "delegated-auth", false, "Setup delegated auth")
	o.RecommendedOptions.AddFlags(flags)
	o.InsecureServingOptions.AddFlags(flags)

	feature.DefaultMutableFeatureGate.AddFlag(flags)

	klog.InitFlags(klogFlags)
	flags.AddGoFlagSet(klogFlags)

	return cmd, o
}

func (o ServerOptions) Validate(args []string) error {
	return nil
}

func (o *ServerOptions) Complete() error {
	return nil
}

func applyOptions(config *genericapiserver.Config, applyTo ...func(*genericapiserver.Config) error) error {
	for _, fn := range applyTo {
		if err := fn(config); err != nil {
			return err
		}
	}

	return nil
}

func (o ServerOptions) Config(tweakConfigFuncs ...func(config *apiserver.Config) error) (*apiserver.Config, error) {
	// switching pagination according to the feature-gate
	// o.RecommendedOptions.Etcd.StorageConfig.Paging = feature.DefaultFeatureGate.Enabled(features.APIListChunking)

	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, nil); err != nil {

		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(builders.Codecs)

	// TODO(yue9944882): for backward-compatibility, a loopback client is optional in the server. But if the client is
	//  missing, server will have to lose the following additional functionalities:
	//  - 	all admission controllers: almost all admission controllers relies on injecting loopback client or loopback
	//  	informers.
	//  -	delegated authentication/authorization: the server will not be able to request kube-apiserver for delegated
	//		authn/authz apis.
	loopbackClientOptional := true
	loopbackKubeConfig, kubeInformerFactory, err := o.buildLoopback()
	if loopbackClientOptional {
		if err != nil {
			klog.Warningf("attempting to instantiate loopback client but failed: %v", err)
		} else {
			serverConfig.LoopbackClientConfig = loopbackKubeConfig
			serverConfig.SharedInformerFactory = kubeInformerFactory
		}
	} else {
		if err != nil {
			return nil, err
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
				loopbackKubeConfig,
				kubeInformerFactory,
				o.RecommendedOptions.ProcessInfo,
				nil,
			)
		},
		o.RecommendedOptions.Features.ApplyTo,
	)
	if err != nil {
		return nil, err
	}

	var insecureServingInfo *genericapiserver.DeprecatedInsecureServingInfo
	if err := o.InsecureServingOptions.ApplyTo(&insecureServingInfo, &serverConfig.LoopbackClientConfig); err != nil {
		return nil, err
	}
	config := &apiserver.Config{
		RecommendedConfig:   serverConfig,
		InsecureServingInfo: insecureServingInfo,
		PostStartHooks:      make(map[string]genericapiserver.PostStartHookFunc),
	}

	o.RecommendedOptions.Authentication.ApplyTo(&serverConfig.Authentication, serverConfig.Config.SecureServing, serverConfig.Config.OpenAPIConfig)
	o.RecommendedOptions.Authorization.ApplyTo(&serverConfig.Authorization)

	for _, tweakConfigFunc := range tweakConfigFuncs {
		if err := tweakConfigFunc(config); err != nil {
			return nil, err
		}
	}
	return config, nil
}

func (o *ServerOptions) buildLoopback() (*rest.Config, informers.SharedInformerFactory, error) {
	var loopbackConfig *rest.Config
	var err error
	// TODO(yue9944882): protobuf serialization?
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

func (o *ServerOptions) RunServer(stopCh <-chan struct{}, title, version string, tweakConfigFuncs ...func(apiserver *apiserver.Config) error) error {
	aggregatedAPIServerConfig, err := o.Config(tweakConfigFuncs...)
	if err != nil {
		return err
	}
	genericConfig := &aggregatedAPIServerConfig.RecommendedConfig.Config

	if o.PrintBearerToken {
		klog.Infof("Serving on loopback...")
		klog.Infof("\n\n********************************\nTo test the server run:\n"+
			"curl -k -H \"Authorization: Bearer %s\" %s\n********************************\n\n",
			genericConfig.LoopbackClientConfig.BearerToken,
			genericConfig.LoopbackClientConfig.Host)
	}
	o.BearerToken = genericConfig.LoopbackClientConfig.BearerToken

	for _, provider := range o.APIBuilders {
		aggregatedAPIServerConfig.AddApi(provider)
	}

	aggregatedAPIServerConfig.Init()

	genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(GetOpenApiDefinition, openapinamer.NewDefinitionNamer(builders.Scheme))
	genericConfig.OpenAPIConfig.Info.Title = title
	genericConfig.OpenAPIConfig.Info.Version = version

	genericServer, err := aggregatedAPIServerConfig.Complete().New()
	if err != nil {
		return err
	}

	for _, h := range o.PostStartHooks {
		if err := genericServer.GenericAPIServer.AddPostStartHook(h.Name, h.Fn); err != nil {
			return err
		}
	}

	s := genericServer.GenericAPIServer.PrepareRun()
	err = validators.OpenAPI.SetSchema(readOpenapi(genericConfig.LoopbackClientConfig.BearerToken, genericServer.GenericAPIServer.Handler))
	if o.PrintOpenapi {
		fmt.Printf("%s", validators.OpenAPI.OpenAPI)
		os.Exit(0)
	}
	if err != nil {
		return err
	}

	if aggregatedAPIServerConfig.InsecureServingInfo != nil {
		fmt.Println("Starting in insecure mode")

		handler := s.GenericAPIServer.UnprotectedHandler()
		handler = genericapifilters.WithAudit(handler, genericConfig.AuditBackend, genericConfig.AuditPolicyChecker, genericConfig.LongRunningFunc)
		handler = genericapifilters.WithAuthentication(handler, server.InsecureSuperuser{}, nil, nil)
		handler = genericfilters.WithCORS(handler, genericConfig.CorsAllowedOriginList, nil, nil, nil, "true")
		handler = genericfilters.WithTimeoutForNonLongRunningRequests(handler, genericConfig.LongRunningFunc, genericConfig.RequestTimeout)
		handler = genericfilters.WithMaxInFlightLimit(handler, genericConfig.MaxRequestsInFlight, genericConfig.MaxMutatingRequestsInFlight, genericConfig.LongRunningFunc)
		handler = genericapifilters.WithRequestInfo(handler, server.NewRequestInfoResolver(genericConfig))
		handler = genericfilters.WithPanicRecovery(handler)
		if err := aggregatedAPIServerConfig.InsecureServingInfo.Serve(handler, genericConfig.RequestTimeout, stopCh); err != nil {
			return err
		}
	}

	return s.Run(stopCh)
}

func readOpenapi(bearerToken string, handler *genericapiserver.APIServerHandler) string {
	req, err := http.NewRequest("GET", "/openapi/v2", nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", bearerToken))
	if err != nil {
		panic(fmt.Errorf("Could not create openapi request %v", err))
	}
	resp := &BufferedResponse{}
	handler.ServeHTTP(resp, req)
	return resp.String()
}

type BufferedResponse struct {
	bytes.Buffer
}

func (BufferedResponse) Header() http.Header { return http.Header{} }
func (BufferedResponse) WriteHeader(int)     {}
