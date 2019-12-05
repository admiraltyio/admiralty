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
	"admiralty.io/multicluster-scheduler/pkg/apis"
	"admiralty.io/multicluster-scheduler/pkg/config/agent"
	"admiralty.io/multicluster-scheduler/pkg/controllers/feedback"
	"admiralty.io/multicluster-scheduler/pkg/controllers/nodepool"
	"admiralty.io/multicluster-scheduler/pkg/controllers/receive"
	"admiralty.io/multicluster-scheduler/pkg/controllers/send"
	"admiralty.io/multicluster-scheduler/pkg/controllers/svcreroute"
	"admiralty.io/multicluster-service-account/pkg/config"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/sample-controller/pkg/signals"
)

func main() {
	m := manager.New()

	agentClientCfg, _, err := config.ConfigAndNamespaceForContext("")
	if err != nil {
		log.Fatalf("cannot load member cluster config: %v", err)
	}
	log.Printf("Local API server URL: %s\n", agentClientCfg.Host)

	agentClientset, err := kubernetes.NewForConfig(agentClientCfg)
	if err != nil {
		log.Fatalf("cannot create member client set: %v", err)
	}

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

	agentConfig := agent.New()

	for _, remote := range agentConfig.Remotes {
		log.Printf("Remote Kubernetes API server address: %s\n", remote.ClientConfig.Host)
		log.Printf("Remote namespace: %s\n", remote.Namespace)

		// member cluster can be known by different names depending on the remote
		agentCluster := agentCluster.CloneWithName(remote.ClusterName)

		remoteCluster := cluster.New("remote", remote.ClientConfig,
			cluster.Options{CacheOptions: cluster.CacheOptions{Namespace: remote.Namespace}})
		if err := apis.AddToScheme(remoteCluster.GetScheme()); err != nil {
			log.Fatalf("adding APIs to scheduler cluster's scheme: %v", err)
		}

		for liveType, obsType := range send.AllObservations {
			co, err := send.NewController(agentCluster, remoteCluster, remote.Namespace, liveType, obsType)
			if err != nil {
				log.Fatalf("cannot create send controller: %v", err)
			}
			m.AddController(co)
		}

		for decType, delType := range receive.AllDecisions {
			co, err := receive.NewController(agentCluster, remoteCluster, remote.Namespace, decType, delType)
			if err != nil {
				log.Fatalf("cannot create receive controller: %v", err)
			}
			m.AddController(co)
		}

		co, err := feedback.NewController(agentCluster, remoteCluster, remote.Namespace, agentClientset)
		if err != nil {
			log.Fatalf("cannot create feedback controller: %v", err)
		}
		m.AddController(co)
	}

	if err := m.Start(signals.SetupSignalHandler()); err != nil {
		log.Fatalf("while or after starting manager: %v", err)
	}
}
