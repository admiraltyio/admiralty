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
	"time"

	"admiralty.io/multicluster-controller/pkg/cluster"
	mcmgr "admiralty.io/multicluster-controller/pkg/manager"
	"admiralty.io/multicluster-service-account/pkg/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	vklog "github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/sample-controller/pkg/signals"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"admiralty.io/multicluster-scheduler/pkg/apis"
	agentconfig "admiralty.io/multicluster-scheduler/pkg/config/agent"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	"admiralty.io/multicluster-scheduler/pkg/controllers/chaperon"
	"admiralty.io/multicluster-scheduler/pkg/controllers/feedback"
	"admiralty.io/multicluster-scheduler/pkg/controllers/follow"
	"admiralty.io/multicluster-scheduler/pkg/controllers/globalsvc"
	"admiralty.io/multicluster-scheduler/pkg/controllers/svcreroute"
	"admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	clientset "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions"
	"admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/vk/node"
	"admiralty.io/multicluster-scheduler/pkg/webhooks/proxypod"
)

// TODO standardize logging

func main() {
	stopCh := signals.SetupSignalHandler()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stopCh
		cancel()
	}()

	agentCfg := agentconfig.New()

	cfg, _, err := config.ConfigAndNamespaceForKubeconfigAndContext("", "")
	utilruntime.Must(err)

	k, err := kubernetes.NewForConfig(cfg)
	utilruntime.Must(err)

	startOldStyleControllers(stopCh, agentCfg, cfg, k)
	startWebhook(stopCh, agentCfg, cfg)

	if len(agentCfg.Targets) > 0 || agentCfg.Raw.TargetSelf {
		startControllers(ctx, stopCh, agentCfg, cfg)
		startVirtualKubelet(ctx, agentCfg, k)
	}

	<-stopCh
}

// TODO: this is a bit messy, we need to refactor using a pattern similar to the one of multicluster-controller,
// but for "old-style" controllers, i.e., using typed informers
func startOldStyleControllers(stopCh <-chan struct{}, agentCfg agentconfig.Config, cfg *rest.Config, k *kubernetes.Clientset) {
	customClient, err := versioned.NewForConfig(cfg)
	utilruntime.Must(err)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(k, time.Second*30)
	customInformerFactory := informers.NewSharedInformerFactory(customClient, time.Second*30)

	n := len(agentCfg.Targets)
	if agentCfg.Raw.TargetSelf {
		n++
	}
	targetCustomClients := make(map[string]clientset.Interface, n)
	targetCustomInformerFactories := make(map[string]informers.SharedInformerFactory, n)
	targetPodChaperonInformers := make(map[string]v1alpha1.PodChaperonInformer, n)
	for _, target := range agentCfg.Targets {
		c, err := versioned.NewForConfig(target.ClientConfig)
		utilruntime.Must(err)
		targetCustomClients[target.Name] = c

		f := informers.NewSharedInformerFactoryWithOptions(c, time.Second*30, informers.WithNamespace(target.Namespace))
		targetCustomInformerFactories[target.Name] = f

		targetPodChaperonInformers[target.Name] = f.Multicluster().V1alpha1().PodChaperons()
	}
	if agentCfg.Raw.TargetSelf {
		targetCustomClients[agentCfg.Raw.ClusterName] = customClient
		targetPodChaperonInformers[agentCfg.Raw.ClusterName] = customInformerFactory.Multicluster().V1alpha1().PodChaperons()
	}

	podInformer := kubeInformerFactory.Core().V1().Pods()
	podChaperonInformer := customInformerFactory.Multicluster().V1alpha1().PodChaperons()

	chapCtrl := chaperon.NewController(k, customClient, podInformer, podChaperonInformer)
	var feedbackCtrl *controller.Controller
	if n > 0 {
		feedbackCtrl = feedback.NewController(k, targetCustomClients, podInformer, targetPodChaperonInformers)
	}

	nt := len(agentCfg.Targets)
	var cmFollowCtrl *controller.Controller
	var secretFollowCtrl *controller.Controller
	if nt > 0 {
		targetKubeClients := make(map[string]kubernetes.Interface, nt)
		targetKubeInformerFactories := make(map[string]kubeinformers.SharedInformerFactory, nt)
		targetConfigMapInformers := make(map[string]v1.ConfigMapInformer, nt)
		targetSecretInformers := make(map[string]v1.SecretInformer, nt)
		for _, target := range agentCfg.Targets {
			k, err := kubernetes.NewForConfig(target.ClientConfig)
			utilruntime.Must(err)
			targetKubeClients[target.Name] = k

			f := kubeinformers.NewSharedInformerFactoryWithOptions(k, time.Second*30, kubeinformers.WithNamespace(target.Namespace))
			targetKubeInformerFactories[target.Name] = f

			targetConfigMapInformers[target.Name] = f.Core().V1().ConfigMaps()
			targetSecretInformers[target.Name] = f.Core().V1().Secrets()
		}
		cmFollowCtrl = follow.NewConfigMapController(k, targetKubeClients, podInformer,
			kubeInformerFactory.Core().V1().ConfigMaps(), targetConfigMapInformers, agentCfg.Raw.ClusterName)
		secretFollowCtrl = follow.NewSecretController(k, targetKubeClients, podInformer,
			kubeInformerFactory.Core().V1().Secrets(), targetSecretInformers, agentCfg.Raw.ClusterName)
		for _, f := range targetKubeInformerFactories {
			f.Start(stopCh)
		}
	}

	kubeInformerFactory.Start(stopCh)
	customInformerFactory.Start(stopCh)
	for _, f := range targetCustomInformerFactories {
		f.Start(stopCh)
	}

	go func() {
		if err = chapCtrl.Run(2, stopCh); err != nil {
			klog.Fatalf("Error running controller: %s", err.Error())
		}
	}()
	if feedbackCtrl != nil {
		go func() {
			if err = feedbackCtrl.Run(2, stopCh); err != nil {
				klog.Fatalf("Error running controller: %s", err.Error())
			}
		}()
	}
	if nt > 0 {
		go func() {
			utilruntime.Must(cmFollowCtrl.Run(2, stopCh))
		}()
		go func() {
			utilruntime.Must(secretFollowCtrl.Run(2, stopCh))
		}()
	}
}

func startControllers(ctx context.Context, stopCh <-chan struct{}, agentCfg agentconfig.Config, cfg *rest.Config) {
	m := mcmgr.New()

	o := cluster.Options{}
	resync := 30 * time.Second
	o.Resync = &resync
	src := cluster.New(agentCfg.Raw.ClusterName, cfg, cluster.Options{})
	if err := apis.AddToScheme(src.GetScheme()); err != nil {
		log.Fatalf("adding APIs to member cluster's scheme: %v", err)
	}
	sourceClusters := []*cluster.Cluster{src}

	n := len(agentCfg.Targets)
	if agentCfg.Raw.TargetSelf {
		n++
	}
	targetClusters := make([]*cluster.Cluster, n)
	for i, target := range agentCfg.Targets {
		o := cluster.Options{}
		o.Namespace = target.Namespace
		o.Resync = &resync
		t := cluster.New(target.Name, target.ClientConfig, o)
		if err := apis.AddToScheme(t.GetScheme()); err != nil {
			log.Fatalf("adding APIs to member cluster's scheme: %v", err)
		}
		targetClusters[i] = t
	}
	if agentCfg.Raw.TargetSelf {
		targetClusters[n-1] = src
	}

	co, err := svcreroute.NewController(ctx, src)
	if err != nil {
		log.Fatalf("cannot create svcreroute controller: %v", err)
	}
	m.AddController(co)

	co, err = globalsvc.NewController(ctx, sourceClusters, targetClusters)
	if err != nil {
		log.Fatalf("cannot create feedback controller: %v", err)
	}
	m.AddController(co)

	go func() {
		if err := m.Start(stopCh); err != nil {
			log.Fatalf("while or after starting multi-cluster manager: %v", err)
		}
	}()
}

func startWebhook(stopCh <-chan struct{}, agentCfg agentconfig.Config, cfg *rest.Config) {
	webhookMgr, err := manager.New(cfg, manager.Options{Port: agentCfg.Raw.Webhook.Port, CertDir: agentCfg.Raw.Webhook.CertDir, MetricsBindAddress: "0"})
	utilruntime.Must(err)

	hookServer := webhookMgr.GetWebhookServer()
	hookServer.Register("/mutate-v1-pod", &webhook.Admission{Handler: &proxypod.Handler{}})

	go func() {
		utilruntime.Must(webhookMgr.Start(stopCh))
	}()
}

func startVirtualKubelet(ctx context.Context, agentCfg agentconfig.Config, k kubernetes.Interface) {
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

	for _, target := range agentCfg.Targets {
		n := "admiralty-" + target.Name
		go func(nodeName string) {
			if err := node.Run(ctx, node.Opts{NodeName: n}, k); err != nil && errors.Cause(err) != context.Canceled {
				vklog.G(ctx).Fatal(err)
			}
		}(n)
	}
	if agentCfg.Raw.TargetSelf {
		go func() {
			if err := node.Run(ctx, node.Opts{NodeName: "admiralty-" + agentCfg.Raw.ClusterName}, k); err != nil && errors.Cause(err) != context.Canceled {
				vklog.G(ctx).Fatal(err)
			}
		}()
	}
}
