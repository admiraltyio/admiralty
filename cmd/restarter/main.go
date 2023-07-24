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
	"os"
	"time"

	"admiralty.io/multicluster-scheduler/pkg/leaderelection"
	"admiralty.io/multicluster-service-account/pkg/config"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/sample-controller/pkg/signals"

	"admiralty.io/multicluster-scheduler/pkg/controllers/target"
	client "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions"
)

func main() {
	ctx := signals.SetupSignalHandler()

	cfg, ns, err := config.ConfigAndNamespaceForKubeconfigAndContext("", "")
	utilruntime.Must(err)

	k, err := kubernetes.NewForConfig(cfg)
	utilruntime.Must(err)

	customClient, err := client.NewForConfig(cfg)
	utilruntime.Must(err)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(k, time.Second*30)
	customInformerFactory := informers.NewSharedInformerFactory(customClient, time.Second*30)

	targetCtrl := target.NewController(k, ns,
		os.Getenv("ADMIRALTY_CONTROLLER_MANAGER_DEPLOYMENT_NAME"),
		os.Getenv("ADMIRALTY_PROXY_SCHEDULER_DEPLOYMENT_NAME"),
		customInformerFactory.Multicluster().V1alpha1().ClusterTargets(),
		customInformerFactory.Multicluster().V1alpha1().Targets(),
		kubeInformerFactory.Core().V1().Secrets())

	kubeInformerFactory.Start(ctx.Done())
	customInformerFactory.Start(ctx.Done())

	var leaderElect bool
	flag.BoolVar(&leaderElect, "leader-elect", false, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flag.Parse()

	if leaderElect {
		leaderelection.Run(ctx, ns, "admiralty-restarter", k, func(ctx context.Context) {
			utilruntime.Must(targetCtrl.Run(ctx, 1))
		})
	} else {
		utilruntime.Must(targetCtrl.Run(ctx, 1))
	}
}
