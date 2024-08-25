/*
 * Copyright 2023 The Multicluster-Scheduler Authors.
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
	"os"
	"time"

	agentconfig "admiralty.io/multicluster-scheduler/pkg/config/agent"
	"admiralty.io/multicluster-scheduler/pkg/controllers/chaperon"
	"admiralty.io/multicluster-scheduler/pkg/controllers/cleanup"
	"admiralty.io/multicluster-scheduler/pkg/controllers/feedback"
	"admiralty.io/multicluster-scheduler/pkg/controllers/follow"
	"admiralty.io/multicluster-scheduler/pkg/controllers/follow/ingress"
	"admiralty.io/multicluster-scheduler/pkg/controllers/follow/service"
	"admiralty.io/multicluster-scheduler/pkg/controllers/resources"
	"admiralty.io/multicluster-scheduler/pkg/controllers/source"
	"admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	clientset "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions"
	"admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/leaderelection"
	"admiralty.io/multicluster-scheduler/pkg/vk/csr"
	"admiralty.io/multicluster-scheduler/pkg/vk/http"
	"admiralty.io/multicluster-scheduler/pkg/vk/node"
	"admiralty.io/multicluster-scheduler/pkg/webhooks/proxypod"
	"admiralty.io/multicluster-service-account/pkg/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	vklog "github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/sample-controller/pkg/signals"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// TODO standardize logging

func main() {
	ctx := signals.SetupSignalHandler()

	o := parseFlags()
	setupLogging(ctx, o)

	agentCfg := agentconfig.NewFromCRD(ctx)

	cfg, ns, err := config.ConfigAndNamespaceForKubeconfigAndContext("", "")
	utilruntime.Must(err)

	k, err := kubernetes.NewForConfig(cfg)
	utilruntime.Must(err)

	startWebhook(ctx, cfg, agentCfg)
	go startVirtualKubeletServers(ctx, agentCfg, k)

	if o.leaderElect {
		leaderelection.Run(ctx, ns, "admiralty-controller-manager", k, func(ctx context.Context) {
			runControllers(ctx, agentCfg, cfg, k)
		})
	} else {
		runControllers(ctx, agentCfg, cfg, k)
	}
}

func runControllers(ctx context.Context, agentCfg agentconfig.Config, cfg *rest.Config, k *kubernetes.Clientset) {
	var nodeStatusUpdaters map[string]resources.NodeStatusUpdater
	if len(agentCfg.Targets) > 0 {
		nodeStatusUpdaters = startVirtualKubeletControllers(ctx, agentCfg, k)
	}
	startOldStyleControllers(ctx, agentCfg, cfg, k, nodeStatusUpdaters)
	<-ctx.Done()
}

type startable interface {
	// Start doesn't block
	Start(stopCh <-chan struct{})
}

type runnable interface {
	// Run blocks
	Run(ctx context.Context, threadiness int) error
}

func startOldStyleControllers(
	ctx context.Context,
	agentCfg agentconfig.Config,
	cfg *rest.Config,
	k *kubernetes.Clientset,
	nodeStatusUpdaters map[string]resources.NodeStatusUpdater,
) {
	customClient, err := versioned.NewForConfig(cfg)
	utilruntime.Must(err)

	var factories []startable
	var controllers []runnable

	clusterName := os.Getenv("CLUSTER_NAME")

	for _, target := range agentCfg.Targets {
		kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(k, time.Second*30, kubeinformers.WithNamespace(target.Namespace))
		factories = append(factories, kubeInformerFactory)
		customInformerFactory := informers.NewSharedInformerFactoryWithOptions(customClient, time.Second*30, informers.WithNamespace(target.Namespace))
		factories = append(factories, customInformerFactory)

		var targetCustomClient versioned.Interface
		var targetPodChaperonInformer v1alpha1.PodChaperonInformer
		var targetClusterSummaryInformer v1alpha1.ClusterSummaryInformer
		if target.Self {
			// re-use
			targetCustomClient = customClient
			targetPodChaperonInformer = customInformerFactory.Multicluster().V1alpha1().PodChaperons()
			targetClusterSummaryInformer = customInformerFactory.Multicluster().V1alpha1().ClusterSummaries()
		} else {
			targetKubeClient, err := kubernetes.NewForConfig(target.ClientConfig)
			utilruntime.Must(err)
			targetCustomClient, err = versioned.NewForConfig(target.ClientConfig)
			utilruntime.Must(err)

			targetKubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(targetKubeClient, time.Second*30, kubeinformers.WithNamespace(target.Namespace))
			factories = append(factories, targetKubeInformerFactory)
			targetCustomInformerFactory := informers.NewSharedInformerFactoryWithOptions(targetCustomClient, time.Second*30, informers.WithNamespace(target.Namespace))
			factories = append(factories, targetCustomInformerFactory)

			targetPodChaperonInformer = targetCustomInformerFactory.Multicluster().V1alpha1().PodChaperons()
			targetClusterSummaryInformer = targetCustomInformerFactory.Multicluster().V1alpha1().ClusterSummaries()

			controllers = append(
				controllers,
				follow.NewConfigMapController(
					clusterName,
					target,
					k,
					targetKubeClient,
					kubeInformerFactory.Core().V1().Pods(),
					kubeInformerFactory.Core().V1().ConfigMaps(),
					targetKubeInformerFactory.Core().V1().ConfigMaps(),
				),
				service.NewController(
					clusterName,
					target,
					k,
					targetKubeClient,
					kubeInformerFactory.Core().V1().Endpoints(),
					kubeInformerFactory.Core().V1().Services(),
					kubeInformerFactory.Core().V1().Pods(),
					targetKubeInformerFactory.Core().V1().Services(),
				),
				follow.NewSecretController(
					clusterName,
					target,
					k,
					targetKubeClient,
					kubeInformerFactory.Core().V1().Pods(),
					kubeInformerFactory.Core().V1().Secrets(),
					targetKubeInformerFactory.Core().V1().Secrets(),
				),
				ingress.NewIngressController(
					clusterName,
					target,
					k,
					targetKubeClient,
					kubeInformerFactory.Core().V1().Services(),
					kubeInformerFactory.Networking().V1().Ingresses(),
					targetKubeInformerFactory.Networking().V1().Ingresses(),
				),
			)
		}
		controllers = append(
			controllers,
			feedback.NewController(
				clusterName,
				target,
				k,
				targetCustomClient,
				kubeInformerFactory.Core().V1().Pods(),
				targetPodChaperonInformer,
			),
			resources.NewUpstreamController(
				target,
				k,
				kubeInformerFactory.Core().V1().Nodes(),
				targetClusterSummaryInformer,
				nodeStatusUpdaters[target.VirtualNodeName],
			),
		)
	}
	factories, controllers = addClusterScopedFactoriesAndControllers(ctx, agentCfg, k, customClient, factories, controllers)

	for _, f := range factories {
		f.Start(ctx.Done())
	}

	for _, c := range controllers {
		c := c
		go func() { utilruntime.Must(c.Run(ctx, 1)) }()
	}
}

func addClusterScopedFactoriesAndControllers(
	ctx context.Context,
	agentCfg agentconfig.Config,
	k *kubernetes.Clientset,
	customClient *clientset.Clientset,
	factories []startable,
	controllers []runnable,
) ([]startable, []runnable) {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(k, time.Second*30)
	factories = append(factories, kubeInformerFactory)
	customInformerFactory := informers.NewSharedInformerFactory(customClient, time.Second*30)
	factories = append(factories, customInformerFactory)

	controllers = append(
		controllers,
		chaperon.NewController(
			k,
			customClient,
			kubeInformerFactory.Core().V1().Pods(),
			customInformerFactory.Multicluster().V1alpha1().PodChaperons(),
		),
		resources.NewDownstreamController(
			customClient,
			kubeInformerFactory.Core().V1().Nodes(),
		),
		cleanup.NewController(
			k,
			kubeInformerFactory.Core().V1().Pods(),
			kubeInformerFactory.Core().V1().Services(),
			kubeInformerFactory.Networking().V1().Ingresses(),
			kubeInformerFactory.Core().V1().ConfigMaps(),
			kubeInformerFactory.Core().V1().Secrets(),
			agentCfg.GetKnownFinalizers(),
		),
	)

	// HACK: indirect feature gate, disable source controller if clustersources cannot be listed (e.g., not allowed)
	srcCtrlEnabled := true
	_, err := customClient.MulticlusterV1alpha1().ClusterSources().List(ctx, metav1.ListOptions{})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("cannot list clustersources, disabling source controller: %s", err))
		srcCtrlEnabled = false
	}
	if srcCtrlEnabled {
		controllers = append(controllers, source.NewController(
			k,
			customInformerFactory.Multicluster().V1alpha1().Sources(),
			customInformerFactory.Multicluster().V1alpha1().ClusterSources(),
			kubeInformerFactory.Core().V1().ServiceAccounts(),
			kubeInformerFactory.Rbac().V1().RoleBindings(),
			kubeInformerFactory.Rbac().V1().ClusterRoleBindings(),
		))
	}
	return factories, controllers
}

func startWebhook(ctx context.Context, cfg *rest.Config, agentCfg agentconfig.Config) {
	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: ":8080",
	})
	utilruntime.Must(err)

	err = builder.WebhookManagedBy(mgr).
		For(&corev1.Pod{}).
		WithDefaulter(proxypod.Mutator{KnownFinalizers: agentCfg.GetKnownFinalizersByNamespace()}).
		Complete()
	utilruntime.Must(err)

	utilruntime.Must(mgr.AddReadyzCheck("webhook-ready", mgr.GetWebhookServer().StartedChecker()))

	go func() {
		utilruntime.Must(mgr.Start(ctx))
	}()
}

func startVirtualKubeletControllers(ctx context.Context, agentCfg agentconfig.Config, k kubernetes.Interface) map[string]resources.NodeStatusUpdater {
	nodeStatusUpdaters := make(map[string]resources.NodeStatusUpdater, len(agentCfg.Targets))
	for _, target := range agentCfg.Targets {
		t := target
		p := &node.NodeProvider{}
		nodeStatusUpdaters[t.VirtualNodeName] = p
		go func() {
			if err := node.Run(ctx, t, k, p); err != nil && errors.Cause(err) != context.Canceled {
				vklog.G(ctx).Fatal(err)
			}
		}()
	}
	return nodeStatusUpdaters
}

func startVirtualKubeletServers(ctx context.Context, agentCfg agentconfig.Config, k kubernetes.Interface) {
	targetConfigs := make(map[string]*rest.Config, len(agentCfg.Targets))
	targetClients := make(map[string]kubernetes.Interface, len(agentCfg.Targets))
	for _, target := range agentCfg.Targets {
		n := target.VirtualNodeName
		targetConfigs[n] = target.ClientConfig
		targetClient, err := kubernetes.NewForConfig(target.ClientConfig)
		utilruntime.Must(err)
		targetClients[n] = targetClient
	}

	certPEM, keyPEM, err := csr.GetCertificateFromKubernetesAPIServer(ctx, k)
	if wait.Interrupted(err) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for virtual kubelet serving certificate to be signed, pod logs/exec won't be supported"))
		return
	}
	utilruntime.Must(err) // likely RBAC issue

	cancelHTTP, err := http.SetupHTTPServer(ctx, &http.LogsExecProvider{
		SourceClient:  k,
		TargetConfigs: targetConfigs,
		TargetClients: targetClients,
	}, certPEM, keyPEM)
	utilruntime.Must(err)

	// this is a little convoluted, TODO: check the close/cancel/context mess with SetupHTTPServer
	<-ctx.Done()
	cancelHTTP()
}

type options struct {
	logLevel    string
	leaderElect bool
}

func parseFlags() *options {
	o := &options{}
	flag.StringVar(&o.logLevel, "log-level", "info", `set the log level, e.g. "debug", "info", "warn", "error"`)
	flag.BoolVar(&o.leaderElect, "leader-elect", false, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	klog.InitFlags(nil)
	flag.Parse()
	return o
}

func setupLogging(ctx context.Context, o *options) {
	vklog.L = logruslogger.FromLogrus(logrus.NewEntry(logrus.StandardLogger()))
	if o.logLevel != "" {
		lvl, err := logrus.ParseLevel(o.logLevel)
		if err != nil {
			vklog.G(ctx).Fatal(errors.Wrap(err, "could not parse log level"))
		}
		logrus.SetLevel(lvl)
	}
}
