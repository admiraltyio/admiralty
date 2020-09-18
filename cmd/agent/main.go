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
	"fmt"
	"log"
	"time"

	"admiralty.io/multicluster-controller/pkg/cluster"
	mcmgr "admiralty.io/multicluster-controller/pkg/manager"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	vklog "github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/sample-controller/pkg/signals"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"admiralty.io/multicluster-scheduler/pkg/apis"
	agentconfig "admiralty.io/multicluster-scheduler/pkg/config/agent"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	"admiralty.io/multicluster-scheduler/pkg/controllers/chaperon"
	"admiralty.io/multicluster-scheduler/pkg/controllers/feedback"
	"admiralty.io/multicluster-scheduler/pkg/controllers/follow"
	"admiralty.io/multicluster-scheduler/pkg/controllers/globalsvc"
	"admiralty.io/multicluster-scheduler/pkg/controllers/resources"
	"admiralty.io/multicluster-scheduler/pkg/controllers/source"
	"admiralty.io/multicluster-scheduler/pkg/controllers/svcreroute"
	"admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	clientset "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions"
	"admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/vk/csr"
	"admiralty.io/multicluster-scheduler/pkg/vk/http"
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

	agentCfg := agentconfig.NewFromCRD(ctx)

	cfg := config.GetConfigOrDie()

	k, err := kubernetes.NewForConfig(cfg)
	utilruntime.Must(err)

	startOldStyleControllers(ctx, stopCh, agentCfg, cfg, k)
	startWebhook(stopCh, cfg)

	if len(agentCfg.Targets) > 0 {
		startControllers(ctx, stopCh, agentCfg, cfg)
		startVirtualKubelet(ctx, agentCfg, k)
	}

	<-stopCh
}

// TODO: this is very messy, we need to refactor using a pattern similar to the one of multicluster-controller,
// but for "old-style" controllers, i.e., using typed informers
func startOldStyleControllers(ctx context.Context, stopCh <-chan struct{}, agentCfg agentconfig.Config, cfg *rest.Config, k *kubernetes.Clientset) {
	customClient, err := versioned.NewForConfig(cfg)
	utilruntime.Must(err)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(k, time.Second*30)
	customInformerFactory := informers.NewSharedInformerFactory(customClient, time.Second*30)

	podInformer := kubeInformerFactory.Core().V1().Pods()
	nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	podChaperonInformer := customInformerFactory.Multicluster().V1alpha1().PodChaperons()

	// TODO? local namespaced informers, as an added security layer

	n := len(agentCfg.Targets)
	nt := 0
	for _, target := range agentCfg.Targets {
		if !target.Self {
			nt++
		}
	}

	targetKubeClients := make(map[string]kubernetes.Interface, nt)
	targetCustomClients := make(map[string]clientset.Interface, n)

	targetKubeInformerFactories := make(map[string]kubeinformers.SharedInformerFactory, nt)
	targetCustomInformerFactories := make(map[string]informers.SharedInformerFactory, n)

	targetPodChaperonInformers := make(map[string]v1alpha1.PodChaperonInformer, n)
	targetClusterSummaryInformers := make(map[string]v1alpha1.ClusterSummaryInformer, n)
	targetConfigMapInformers := make(map[string]v1.ConfigMapInformer, nt)
	targetSecretInformers := make(map[string]v1.SecretInformer, nt)

	selfTargetKeys := make(map[string]bool, n-nt)

	for _, target := range agentCfg.Targets {
		if target.Self {
			selfTargetKeys[target.GetKey()] = true

			// re-use
			targetCustomClients[target.GetKey()] = customClient

			targetPodChaperonInformers[target.GetKey()] = customInformerFactory.Multicluster().V1alpha1().PodChaperons()
			targetClusterSummaryInformers[target.GetKey()] = customInformerFactory.Multicluster().V1alpha1().ClusterSummaries()
		} else {
			k, err := kubernetes.NewForConfig(target.ClientConfig)
			utilruntime.Must(err)
			targetKubeClients[target.GetKey()] = k
			c, err := versioned.NewForConfig(target.ClientConfig)
			utilruntime.Must(err)
			targetCustomClients[target.GetKey()] = c

			kf := kubeinformers.NewSharedInformerFactoryWithOptions(k, time.Second*30, kubeinformers.WithNamespace(target.Namespace))
			targetKubeInformerFactories[target.GetKey()] = kf
			f := informers.NewSharedInformerFactoryWithOptions(c, time.Second*30, informers.WithNamespace(target.Namespace))
			targetCustomInformerFactories[target.GetKey()] = f

			targetPodChaperonInformers[target.GetKey()] = f.Multicluster().V1alpha1().PodChaperons()
			targetClusterSummaryInformers[target.GetKey()] = f.Multicluster().V1alpha1().ClusterSummaries()
			targetConfigMapInformers[target.GetKey()] = kf.Core().V1().ConfigMaps()
			targetSecretInformers[target.GetKey()] = kf.Core().V1().Secrets()
		}
	}

	chapCtrl := chaperon.NewController(k, customClient, podInformer, podChaperonInformer)
	downstreamResCtrl := resources.NewDownstreamController(customClient, nodeInformer)
	var feedbackCtrl *controller.Controller
	var upstreamResCtrl *controller.Controller
	if n > 0 {
		feedbackCtrl = feedback.NewController(k, targetCustomClients, podInformer, targetPodChaperonInformers)
		upstreamResCtrl = resources.NewUpstreamController(k, nodeInformer, targetClusterSummaryInformers)
	}
	var cmFollowCtrl *controller.Controller
	var secretFollowCtrl *controller.Controller
	if nt > 0 {
		cmFollowCtrl = follow.NewConfigMapController(k, targetKubeClients, podInformer,
			kubeInformerFactory.Core().V1().ConfigMaps(), targetConfigMapInformers, selfTargetKeys)
		secretFollowCtrl = follow.NewSecretController(k, targetKubeClients, podInformer,
			kubeInformerFactory.Core().V1().Secrets(), targetSecretInformers, selfTargetKeys)
	}

	var srcCtrl *controller.Controller
	// HACK: indirect feature gate, disable source controller if clustersources cannot be listed (e.g., not allowed)
	srcCtrlEnabled := true
	_, err = customClient.MulticlusterV1alpha1().ClusterSources().List(ctx, metav1.ListOptions{})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("cannot list clustersources, disabling source controller: %s", err))
		srcCtrlEnabled = false
	}
	if srcCtrlEnabled {
		sourceInformer := customInformerFactory.Multicluster().V1alpha1().Sources()
		clusterSourceInformer := customInformerFactory.Multicluster().V1alpha1().ClusterSources()
		saInformer := kubeInformerFactory.Core().V1().ServiceAccounts()
		rbInformer := kubeInformerFactory.Rbac().V1().RoleBindings()
		crbInformer := kubeInformerFactory.Rbac().V1().ClusterRoleBindings()
		srcCtrl = source.NewController(k, sourceInformer, clusterSourceInformer, saInformer, rbInformer, crbInformer)
	}

	kubeInformerFactory.Start(stopCh)
	customInformerFactory.Start(stopCh)
	for _, f := range targetKubeInformerFactories {
		f.Start(stopCh)
	}
	for _, f := range targetCustomInformerFactories {
		f.Start(stopCh)
	}

	go func() { utilruntime.Must(chapCtrl.Run(2, stopCh)) }()
	go func() { utilruntime.Must(downstreamResCtrl.Run(1, stopCh)) }()
	if n > 0 {
		go func() { utilruntime.Must(upstreamResCtrl.Run(1, stopCh)) }()
		go func() { utilruntime.Must(feedbackCtrl.Run(2, stopCh)) }()
	}
	if nt > 0 {
		go func() { utilruntime.Must(cmFollowCtrl.Run(2, stopCh)) }()
		go func() { utilruntime.Must(secretFollowCtrl.Run(2, stopCh)) }()
	}
	if srcCtrlEnabled {
		go func() { utilruntime.Must(srcCtrl.Run(2, stopCh)) }()
	}
}

func startControllers(ctx context.Context, stopCh <-chan struct{}, agentCfg agentconfig.Config, cfg *rest.Config) {
	m := mcmgr.New()

	o := cluster.Options{}
	resync := 30 * time.Second
	o.Resync = &resync
	src := cluster.New("local", cfg, cluster.Options{})
	if err := apis.AddToScheme(src.GetScheme()); err != nil {
		log.Fatalf("adding APIs to member cluster's scheme: %v", err)
	}
	sourceClusters := []*cluster.Cluster{src}

	var targetClusters []*cluster.Cluster
	for _, target := range agentCfg.Targets {
		if !target.Self {
			o := cluster.Options{}
			o.Namespace = target.Namespace
			o.Resync = &resync
			t := cluster.New(target.GetKey(), target.ClientConfig, o)
			if err := apis.AddToScheme(t.GetScheme()); err != nil {
				log.Fatalf("adding APIs to member cluster's scheme: %v", err)
			}
			targetClusters = append(targetClusters, t)
		}
	}

	co, err := svcreroute.NewController(ctx, src)
	utilruntime.Must(err)
	m.AddController(co)

	co, err = globalsvc.NewController(ctx, sourceClusters, targetClusters)
	utilruntime.Must(err)
	m.AddController(co)

	go func() { utilruntime.Must(m.Start(stopCh)) }()
}

func startWebhook(stopCh <-chan struct{}, cfg *rest.Config) {
	webhookMgr, err := manager.New(cfg, manager.Options{
		Port:               9443,
		CertDir:            "/tmp/k8s-webhook-server/serving-certs",
		MetricsBindAddress: "0",
	})
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

	targetConfigs := make(map[string]*rest.Config, len(agentCfg.Targets))
	targetClients := make(map[string]kubernetes.Interface, len(agentCfg.Targets))
	for _, target := range agentCfg.Targets {
		n := target.GetKey()
		targetConfigs[n] = target.ClientConfig
		targetClient, err := kubernetes.NewForConfig(target.ClientConfig)
		utilruntime.Must(err)
		targetClients[n] = targetClient

		go func() {
			if err := node.Run(ctx, node.Opts{NodeName: n}, k); err != nil && errors.Cause(err) != context.Canceled {
				vklog.G(ctx).Fatal(err)
			}
		}()
	}

	certPEM, keyPEM, err := csr.GetCertificateFromKubernetesAPIServer(ctx, k)
	utilruntime.Must(err) // likely RBAC issue

	cancelHTTP, err := http.SetupHTTPServer(ctx, &http.LogsExecProvider{
		SourceClient:  k,
		TargetConfigs: targetConfigs,
		TargetClients: targetClients,
	}, certPEM, keyPEM)
	utilruntime.Must(err)

	// this is a little convoluted, TODO: check the close/cancel/context mess with SetupHTTPServer
	go func() {
		select {
		case <-ctx.Done():
			cancelHTTP()
		}
	}()
}
