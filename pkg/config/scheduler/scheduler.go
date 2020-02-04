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

package scheduler

import (
	"flag"
	"io/ioutil"
	"log"

	cfgapi "admiralty.io/multicluster-scheduler/pkg/apis/config/v1alpha2"
	"admiralty.io/multicluster-service-account/pkg/config"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

type Config struct {
	Clusters []Cluster
}

type Cluster struct {
	Name         string
	ClientConfig *rest.Config
	Namespace    string
}

func New() Config {
	agentCfg := Config{}
	cfgPath := flag.String("config", "/etc/admiralty/config", "")
	flag.Parse()
	s, err := ioutil.ReadFile(*cfgPath)
	if err != nil {
		log.Fatalf("cannot open agent configuration: %v", err)
	}
	raw := &cfgapi.Scheduler{}
	if err := yaml.Unmarshal(s, raw); err != nil {
		log.Fatalf("cannot unmarshal agent configuration: %v", err)
	}
	for _, rawC := range raw.Clusters {
		cfg, ns, err := config.ConfigAndNamespaceForKubeconfigAndContext(rawC.Kubeconfig, rawC.Context)
		if err != nil {
			log.Fatalf("cannot load kubeconfig: %v", err)
		}
		c := Cluster{Name: rawC.Name, ClientConfig: cfg, Namespace: ns}
		agentCfg.Clusters = append(agentCfg.Clusters, c)
	}
	return agentCfg
}
