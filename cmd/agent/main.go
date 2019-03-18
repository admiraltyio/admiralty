/*
Copyright 2018 The Multicluster-Scheduler Authors.

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

package main

import (
	"log"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/manager"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/controllers/feedback"
	"admiralty.io/multicluster-scheduler/pkg/controllers/nodepool"
	"admiralty.io/multicluster-scheduler/pkg/controllers/receive"
	"admiralty.io/multicluster-scheduler/pkg/controllers/send"
	"admiralty.io/multicluster-scheduler/pkg/controllers/svcreroute"
	"admiralty.io/multicluster-service-account/pkg/config"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/sample-controller/pkg/signals"
)

func main() {
	// In the context of multicluster-scheduler, the service account import used to
	// create the *remote* Cluster (to login to the scheduler) gives its name to the *local* Cluster.
	// Indeed, each member Cluster of a federation has a corresponding service account
	// in the federation namespace controlled by the remote scheduler.
	// TODO: one namespace per member cluster to enforce member-level RBAC (not just federation-level).
	agentName, err := config.SingleServiceAccountImportName()
	if err != nil {
		log.Fatalf("cannot get agent name: %v", err)
	}
	log.Printf("Agent name: %s", agentName)

	cfg, _, err := config.ConfigAndNamespaceForContext("")
	// here, we just want to make sure we DON'T use a service account import,
	// which, if there is only one, would be loaded by config.ConfigAndNamespace()
	// TODO (multicluster-service-account): create function with better name or use client-go directly
	if err != nil {
		log.Fatalf("cannot load local cluster config: %v", err)
	}
	log.Printf("Agent API server URL: %s\n", cfg.Host)
	agent := cluster.New(agentName, cfg, cluster.Options{})

	agentClientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("cannot create agent client set: %v", err)
	}

	cfg, ns, err := config.NamedServiceAccountImportConfigAndNamespace(agentName)
	if err != nil {
		log.Fatalf("cannot load remote cluster config: %v", err)
	}
	log.Printf("Scheduler API server URL: %s\n", cfg.Host)
	log.Printf("Federation namespace: %s\n", ns)
	scheduler := cluster.New("scheduler", cfg, cluster.Options{CacheOptions: cluster.CacheOptions{Namespace: ns}})

	observations := map[runtime.Object]runtime.Object{
		&v1.Service{}:        &v1alpha1.ServiceObservation{},
		&v1.Pod{}:            &v1alpha1.PodObservation{},
		&v1.Node{}:           &v1alpha1.NodeObservation{},
		&v1alpha1.NodePool{}: &v1alpha1.NodePoolObservation{},
	}

	m := manager.New()

	for liveType, obsType := range observations {
		co, err := send.NewController(agent, scheduler, ns, liveType, obsType)
		if err != nil {
			log.Fatalf("cannot create send controller: %v", err)
		}
		m.AddController(co)
	}

	decisions := map[runtime.Object]runtime.Object{
		&v1alpha1.PodDecision{}:     &v1.Pod{},
		&v1alpha1.ServiceDecision{}: &v1.Service{},
	}

	for decType, delType := range decisions {
		co, err := receive.NewController(agent, scheduler, decType, delType)
		if err != nil {
			log.Fatalf("cannot create receive controller: %v", err)
		}
		m.AddController(co)
	}

	co, err := feedback.NewController(agent, scheduler, agentClientset)
	if err != nil {
		log.Fatalf("cannot create feedback controller: %v", err)
	}
	m.AddController(co)

	co, err = nodepool.NewController(agent)
	if err != nil {
		log.Fatalf("cannot create nodepool controller: %v", err)
	}
	m.AddController(co)

	co, err = svcreroute.NewController(agent)
	if err != nil {
		log.Fatalf("cannot create svcreroute controller: %v", err)
	}
	m.AddController(co)

	if err := m.Start(signals.SetupSignalHandler()); err != nil {
		log.Fatalf("while or after starting manager: %v", err)
	}
}
