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

package test

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/rest"
	openapi "k8s.io/kube-openapi/pkg/common"
	"sigs.k8s.io/apiserver-builder-alpha/pkg/builders"
	"sigs.k8s.io/apiserver-builder-alpha/pkg/cmd/server"
)

type TestEnvironment struct {
	StopServer        chan struct{}
	ServerOutput      *io.PipeWriter
	ApiserverCobraCmd *cobra.Command
	ApiserverOptions  *server.ServerOptions
	ApiserverPort     int
	BearerToken       string
	EtcdClientPort    int
	EtcdPeerPort      int
	EtcdPath          string
	EtcdCmd           *exec.Cmd
	Done              bool

	apiserverready chan *rest.Config
	etcdready      chan string
}

func NewTestEnvironment(apis []*builders.APIGroupBuilder, openapidefs openapi.GetOpenAPIDefinitions) *TestEnvironment {
	te := &TestEnvironment{
		EtcdPath:       "/registry/test.kubernetes.io",
		StopServer:     make(chan struct{}),
		etcdready:      make(chan string),
		apiserverready: make(chan *rest.Config),
	}

	te.EtcdClientPort = te.getPort()
	te.EtcdPeerPort = te.getPort()
	te.ApiserverPort = te.getPort()

	_, te.ServerOutput = io.Pipe()
	server.GetOpenApiDefinition = openapidefs
	cmd, options := server.NewCommandStartServer(
		te.EtcdPath,
		te.ServerOutput, te.ServerOutput, apis, te.StopServer, "API", "v0")

	options.RecommendedOptions.SecureServing.BindPort = te.ApiserverPort
	options.RunDelegatedAuth = false
	options.RecommendedOptions.Etcd.StorageConfig.Transport.ServerList = []string{
		fmt.Sprintf("http://localhost:%d", te.EtcdClientPort),
	}
	tmpdir, err := ioutil.TempDir("", "apiserver-test")
	if err != nil {
		panic(fmt.Sprintf("Could not create temp dir for testing: %v", err))
	}
	options.RecommendedOptions.SecureServing.ServerCert = genericoptions.GeneratableKeyCert{
		CertDirectory: tmpdir,
		PairName:      "apiserver",
	}

	// Notify once the apiserver is ready to serve traffic
	options.PostStartHooks = []server.PostStartHook{
		{
			Fn: func(context genericapiserver.PostStartHookContext) error {
				// Let the test know the server is ready
				te.apiserverready <- context.LoopbackClientConfig
				return nil
			},
			Name: "apiserver-ready",
		},
	}

	te.ApiserverCobraCmd = cmd
	te.ApiserverOptions = options

	return te
}

func (te *TestEnvironment) getPort() int {
	l, _ := net.Listen("tcp", ":0")
	defer l.Close()
	println(l.Addr().String())
	pieces := strings.Split(l.Addr().String(), ":")
	i, err := strconv.Atoi(pieces[len(pieces)-1])
	if err != nil {
		panic(err)
	}
	return i
}

// Stop stops a running server
func (te *TestEnvironment) Stop() {
	te.Done = true
	te.StopServer <- struct{}{}
	te.EtcdCmd.Process.Kill()
}

// Start starts a local Kubernetes server and updates te.ApiserverPort with the port it is listening on
func (te *TestEnvironment) Start() *rest.Config {
	go te.startEtcd()

	// Wait for etcd to start
	// TODO: Poll the /health address to wait for etcd to become healthy
	time.Sleep(time.Second * 1)

	go te.startApiserver()

	// Wait for everything to be ready
	loopback := <-te.apiserverready
	<-te.etcdready
	return loopback
}

func (te *TestEnvironment) startApiserver() {
	if err := te.ApiserverCobraCmd.Execute(); err != nil {
		panic(err)
	}
}

// startEtcd starts a new etcd process using a random temp data directory and random free port
func (te *TestEnvironment) startEtcd() {
	dirname, err := ioutil.TempDir("/tmp", "apiserver-test")
	if err != nil {
		panic(err)
	}

	clientAddr := fmt.Sprintf("http://localhost:%d", te.EtcdClientPort)
	peerAddr := fmt.Sprintf("http://localhost:%d", te.EtcdPeerPort)
	cmd := exec.Command(
		"etcd",
		"--data-dir", dirname,
		"--listen-client-urls", clientAddr,
		"--listen-peer-urls", peerAddr,
		"--advertise-client-urls", clientAddr,
	)
	te.EtcdCmd = cmd
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	go te.waitForEtcdReady(stdout)
	go te.waitForEtcdReady(stderr)

	err = cmd.Wait()
	if err != nil && !te.Done {
		panic(err)
	}
}

// waitForEtcdReady notify's read once the etcd instances is ready to receive traffic
func (te *TestEnvironment) waitForEtcdReady(reader io.Reader) {
	started := regexp.MustCompile("serving insecure client requests on (.+), this is strongly discouraged!")
	buffered := bufio.NewReader(reader)
	for {
		l, _, err := buffered.ReadLine()
		if err != nil {
			time.Sleep(time.Second * 5)
		}
		line := string(l)
		if started.MatchString(line) {
			addr := started.FindStringSubmatch(line)[1]
			// etcd is ready
			te.etcdready <- addr
			return
		}
	}
}
