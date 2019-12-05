package agent

import (
	configv1alpha1 "admiralty.io/multicluster-scheduler/pkg/apis/config/v1alpha1"
	"admiralty.io/multicluster-service-account/pkg/config"
	"flag"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"log"
	"sigs.k8s.io/yaml"
)

type Config struct {
	Remotes []Remote
}

type Remote struct {
	ClientConfig *rest.Config
	Namespace    string
	ClusterName  string
}

func New() Config {
	agentCfg := Config{}
	cfgPath := flag.String("config", "/etc/admiralty/config", "")
	s, err := ioutil.ReadFile(*cfgPath)
	if err != nil {
		log.Fatalf("cannot open agent configuration: %v", err)
	}
	raw := &configv1alpha1.Agent{}
	if err := yaml.Unmarshal(s, raw); err != nil {
		log.Fatalf("cannot unmarshal agent configuration: %v", err)
	}
	for _, m := range raw.Remotes {
		cfg, ns, err := config.ConfigAndNamespaceForKubeconfigAndContext(m.Kubeconfig, m.Context)
		if err != nil {
			log.Fatalf("cannot load kubeconfig: %v", err)
		}
		r := Remote{ClientConfig: cfg, Namespace: ns}
		if raw.UseClusterNamespaces {
			r.ClusterName = r.Namespace
		} else {
			r.ClusterName = m.ClusterName
		}
		agentCfg.Remotes = append(agentCfg.Remotes, r)
	}
	return agentCfg
}
