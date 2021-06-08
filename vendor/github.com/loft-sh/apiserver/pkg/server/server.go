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
	"flag"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/loft-sh/apiserver/pkg/builders"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog"
	openapi "k8s.io/kube-openapi/pkg/common"
)

type StartOptions struct {
	Apis       []*builders.APIGroupBuilder
	Authorizer authorizer.Authorizer

	GetOpenAPIDefinitions openapi.GetOpenAPIDefinitions
	Version               *version.Info

	TweakServerOptions func(options *ServerOptions)
	TweakServerConfig  func(config *genericapiserver.RecommendedConfig) error

	StopChan            <-chan struct{}
	DisableCommandFlags bool
}

func StartAPIServer(opts *StartOptions) error {
	if opts.StopChan == nil {
		opts.StopChan = genericapiserver.SetupSignalHandler()
	}

	cmd, _ := newAPIServerCommand(opts, opts.StopChan)
	if opts.DisableCommandFlags == false {
		cmd.Flags().AddFlagSet(pflag.CommandLine)
	}
	if err := cmd.Execute(); err != nil {
		return err
	}

	return nil
}

func newAPIServerCommand(opts *StartOptions, stopChan <-chan struct{}) (*cobra.Command, *ServerOptions) {
	o := newAPIServerOptions(opts.Apis)
	o.GetOpenAPIDefinitions = opts.GetOpenAPIDefinitions

	// adjust the server config
	if opts.TweakServerOptions != nil {
		opts.TweakServerOptions(o)
	}

	cmd := &cobra.Command{
		Short: "Launch an API server",
		Long:  "Launch an API server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunServer(opts.Version, stopChan, opts.Authorizer, opts.TweakServerConfig); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)
	feature.DefaultMutableFeatureGate.AddFlag(flags)
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	flags.AddGoFlagSet(klogFlags)
	return cmd, o
}

func newAPIServerOptions(b []*builders.APIGroupBuilder) *ServerOptions {
	versions := []schema.GroupVersion{}
	for _, b := range b {
		versions = append(versions, b.GetLegacyCodec()...)
	}

	builders.Codecs = serializer.NewCodecFactory(builders.Scheme, func(options *serializer.CodecFactoryOptions) {
		options.Strict = true
	})
	o := &ServerOptions{
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			"",
			builders.Codecs.LegacyCodec(versions...),
		),
		APIBuilders: b,
	}

	// we don't use etcd
	o.RecommendedOptions.Etcd = nil
	o.RecommendedOptions.Admission = genericoptions.NewAdmissionOptions()
	o.RecommendedOptions.Admission.DefaultOffPlugins = sets.String{lifecycle.PluginName: sets.Empty{}}

	o.RecommendedOptions.Authorization.RemoteKubeConfigFileOptional = true
	o.RecommendedOptions.Authentication.RemoteKubeConfigFileOptional = true
	return o
}

func applyOptions(config *genericapiserver.Config, applyTo ...func(*genericapiserver.Config) error) error {
	for _, fn := range applyTo {
		if err := fn(config); err != nil {
			return err
		}
	}

	return nil
}
