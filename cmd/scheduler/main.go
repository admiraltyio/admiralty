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
	"log"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/manager"
	"admiralty.io/multicluster-scheduler/pkg/apis"
	schedulerconfig "admiralty.io/multicluster-scheduler/pkg/config/scheduler"
	"admiralty.io/multicluster-scheduler/pkg/controllers/bind"
	"admiralty.io/multicluster-scheduler/pkg/controllers/globalsvc"
	"admiralty.io/multicluster-scheduler/pkg/controllers/schedule"
	"admiralty.io/multicluster-scheduler/pkg/scheduler"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/sample-controller/pkg/signals"
)

func main() {
	schedCfg := schedulerconfig.New()

	m := manager.New()

	clusters := make([]*cluster.Cluster, len(schedCfg.Clusters), len(schedCfg.Clusters))
	kClients := make(map[string]kubernetes.Interface)
	impersonatingKClients := make(map[string]map[string]kubernetes.Interface)
	for i, c := range schedCfg.Clusters {
		clu := cluster.New(c.Name, c.ClientConfig, cluster.Options{})
		if err := apis.AddToScheme(clu.GetScheme()); err != nil {
			log.Fatalf("adding APIs to member cluster's scheme: %v", err)
		}
		clusters[i] = clu

		kClients[c.Name] = kubernetes.NewForConfigOrDie(c.ClientConfig)

		impersonatingKClients[c.Name] = make(map[string]kubernetes.Interface)
		for _, targetC := range schedCfg.Clusters {
			cfg := rest.CopyConfig(targetC.ClientConfig)
			cfg.Impersonate = rest.ImpersonationConfig{
				UserName: "admiralty:" + c.Name,
			}
			impersonatingKClients[c.Name][targetC.Name] = kubernetes.NewForConfigOrDie(cfg)
		}
	}

	co, err := schedule.NewController(clusters, kClients, impersonatingKClients, scheduler.New())
	if err != nil {
		log.Fatalf("cannot create schedule controller: %v", err)
	}
	m.AddController(co)

	co, err = globalsvc.NewController(clusters, impersonatingKClients)
	if err != nil {
		log.Fatalf("cannot create globalsvc controller: %v", err)
	}
	m.AddController(co)

	co, err = bind.NewController(clusters)
	if err != nil {
		log.Fatalf("cannot create bind controller: %v", err)
	}
	m.AddController(co)

	if err := m.Start(signals.SetupSignalHandler()); err != nil {
		log.Fatalf("while or after starting manager: %v", err)
	}
}
