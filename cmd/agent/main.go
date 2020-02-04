/*
 * Copyright 2020 The Multicluster-Scheduler Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"flag"
	"log"

	"admiralty.io/multicluster-controller/pkg/cluster"
	mcmgr "admiralty.io/multicluster-controller/pkg/manager"
	"admiralty.io/multicluster-scheduler/pkg/apis"
	configv1alpha2 "admiralty.io/multicluster-scheduler/pkg/apis/config/v1alpha2"
	agentconfig "admiralty.io/multicluster-scheduler/pkg/config/agent"
	"admiralty.io/multicluster-scheduler/pkg/controllers/nodepool"
	"admiralty.io/multicluster-scheduler/pkg/controllers/svcreroute"
	"admiralty.io/multicluster-scheduler/pkg/vk/node"
	"admiralty.io/multicluster-scheduler/pkg/webhooks/proxypod"
	"admiralty.io/multicluster-service-account/pkg/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	vklog "github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/sample-controller/pkg/signals"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// TODO standardize logging

func main() {
	stopCh := signals.SetupSignalHandler()

	agentCfg := agentconfig.New()

	agentClientCfg, _, err := config.ConfigAndNamespaceForContext("")
	if err != nil {
		log.Fatalf("cannot load member cluster config: %v", err)
	}
	log.Printf("Local API server URL: %s\n", agentClientCfg.Host)

	startControllers(stopCh, agentClientCfg)
	startWebhook(stopCh, agentClientCfg, agentCfg.Webhook)
	startVirtualKubelet(stopCh, agentClientCfg)

	<-stopCh
}

func startControllers(stopCh <-chan struct{}, agentClientCfg *rest.Config) {
	m := mcmgr.New()

	agentCluster := cluster.New("local", agentClientCfg, cluster.Options{})
	if err := apis.AddToScheme(agentCluster.GetScheme()); err != nil {
		log.Fatalf("adding APIs to member cluster's scheme: %v", err)
	}

	co, err := nodepool.NewController(agentCluster)
	if err != nil {
		log.Fatalf("cannot create nodepool controller: %v", err)
	}
	m.AddController(co)

	co, err = svcreroute.NewController(agentCluster)
	if err != nil {
		log.Fatalf("cannot create svcreroute controller: %v", err)
	}
	m.AddController(co)

	go func() {
		if err := m.Start(stopCh); err != nil {
			log.Fatalf("while or after starting multi-cluster manager: %v", err)
		}
	}()
}

func startWebhook(stopCh <-chan struct{}, agentClientCfg *rest.Config, whCfg configv1alpha2.Webhook) {
	webhookMgr, err := manager.New(agentClientCfg, manager.Options{Port: whCfg.Port, CertDir: whCfg.CertDir, MetricsBindAddress: "0"})
	if err != nil {
		log.Fatalf("cannot create webhook manager: %v", err)
	}

	hookServer := webhookMgr.GetWebhookServer()
	hookServer.Register("/mutate-v1-pod", &webhook.Admission{Handler: &proxypod.Handler{}})

	go func() {
		if err := webhookMgr.Start(stopCh); err != nil {
			log.Fatalf("while or after starting webhook manager: %v", err)
		}
	}()
}

func startVirtualKubelet(stopCh <-chan struct{}, agentClientCfg *rest.Config) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stopCh
		cancel()
	}()

	var logLevel string
	flag.StringVar(&logLevel, "log-level", "info", `set the log level, e.g. "debug", "info", "warn", "error"`)
	klog.InitFlags(nil)
	flag.Parse()

	vklog.L = logruslogger.FromLogrus(logrus.NewEntry(logrus.StandardLogger()))
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			vklog.G(ctx).Fatal(errors.Wrap(err, "could not parse log level"))
		}
		logrus.SetLevel(lvl)
	}

	k, err := kubernetes.NewForConfig(agentClientCfg)
	if err != nil {
		vklog.G(ctx).Fatal(err)
	}

	go func() {
		if err := node.Run(ctx, node.Opts{NodeName: "admiralty"}, k); err != nil && errors.Cause(err) != context.Canceled {
			vklog.G(ctx).Fatal(err)
		}
	}()
}
