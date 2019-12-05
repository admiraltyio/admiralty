package scheduler

import (
	"flag"
	"io/ioutil"
	"log"

	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	configv1alpha1 "admiralty.io/multicluster-scheduler/pkg/apis/config/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// TODO... better typing

type Config struct {
	// Namespaces to watch
	Namespaces []string

	// NamespaceForCluster to create decisions into
	NamespaceForCluster map[string]string

	// FederationsByCluster for now unused outside of transform
	FederationsByCluster map[string]map[string]struct{}

	// ClustersByFederation to filter observations in schedule when federation annotation is NOT empty
	ClustersByFederation map[string]map[string]struct{}

	// NamespacesByFederation to list observations in schedule when federation annotation is NOT empty
	NamespacesByFederation map[string][]string

	// PairedClustersByCluster to filter observations in schedule when federation annotation is empty (any federation)
	PairedClustersByCluster map[string]map[string]struct{}

	// PairedNamespacesByCluster to list observations in schedule when federation annotation is empty (any federation)
	PairedNamespacesByCluster map[string][]string

	// UseClusterNamespaces to determine whether source cluster names are the names of the observations's namespaces
	// or the ParentClusterName multi-cluster GC label (trust the agent then).
	UseClusterNamespaces bool
}

func Load(schedulerNamespace string) *Config {
	path := flag.String("config", "/etc/admiralty/config", "")
	s, err := ioutil.ReadFile(*path)
	if err != nil {
		log.Fatalf("cannot open scheduler configuration: %v", err)
	}
	raw := &configv1alpha1.Scheduler{}
	if err := yaml.Unmarshal(s, raw); err != nil {
		log.Fatalf("cannot unmarshal scheduler configuration: %v", err)
	}
	return New(raw, schedulerNamespace)
}

func New(raw *configv1alpha1.Scheduler, schedulerNamespace string) *Config {
	setDefaults(raw, schedulerNamespace)
	return transform(raw)
}

func setDefaults(raw *configv1alpha1.Scheduler, schedulerNamespace string) {
	for i := range raw.Clusters {
		c := &raw.Clusters[i]
		if c.ClusterNamespace == "" {
			c.ClusterNamespace = schedulerNamespace
		}
		if len(c.Memberships) == 0 {
			c.Memberships = []configv1alpha1.Membership{{FederationName: "default"}}
		}
	}
}

func transform(raw *configv1alpha1.Scheduler) *Config {
	cfg := &Config{
		NamespaceForCluster:       map[string]string{},
		FederationsByCluster:      map[string]map[string]struct{}{},
		ClustersByFederation:      map[string]map[string]struct{}{},
		NamespacesByFederation:    map[string][]string{},
		PairedClustersByCluster:   map[string]map[string]struct{}{},
		PairedNamespacesByCluster: map[string][]string{},
		UseClusterNamespaces:      raw.UseClusterNamespaces,
	}

	namespaces := map[string]struct{}{} // intermediate var to dedup namespaces
	//clustersByFed := map[string]map[string]struct{}{} // intermediate var to dedup clusters by fed

	for _, c := range raw.Clusters {
		cfg.NamespaceForCluster[c.Name] = c.ClusterNamespace
		namespaces[c.ClusterNamespace] = struct{}{}

		for _, m := range c.Memberships {
			if cfg.FederationsByCluster[c.Name] == nil {
				cfg.FederationsByCluster[c.Name] = map[string]struct{}{}
			}
			cfg.FederationsByCluster[c.Name][m.FederationName] = struct{}{}

			if cfg.ClustersByFederation[m.FederationName] == nil {
				cfg.ClustersByFederation[m.FederationName] = map[string]struct{}{}
			}
			cfg.ClustersByFederation[m.FederationName][c.Name] = struct{}{}
		}
	}

	for ns := range namespaces {
		cfg.Namespaces = append(cfg.Namespaces, ns)
	}
	//for f, cm := range clustersByFed {
	//	var cl []string
	//	for c, _ := range cm {
	//		cl = append(cl, c)
	//	}
	//	cfg.ClustersByFederation[f] = cl
	//}

	for f, cs := range cfg.ClustersByFederation {
		namespaces := map[string]struct{}{} // intermediate var to dedup namespaces
		for c := range cs {
			namespaces[cfg.NamespaceForCluster[c]] = struct{}{}
		}
		for ns := range namespaces {
			cfg.NamespacesByFederation[f] = append(cfg.NamespacesByFederation[f], ns)
		}
	}

	for srcC, fs := range cfg.FederationsByCluster {
		namespaces := map[string]struct{}{} // intermediate var to dedup namespaces
		if cfg.PairedClustersByCluster[srcC] == nil {
			cfg.PairedClustersByCluster[srcC] = map[string]struct{}{}
		}
		for f := range fs {
			for c := range cfg.ClustersByFederation[f] {
				cfg.PairedClustersByCluster[srcC][c] = struct{}{}
				namespaces[cfg.NamespaceForCluster[c]] = struct{}{}
			}
		}
		for ns := range namespaces {
			cfg.PairedNamespacesByCluster[srcC] = append(cfg.PairedNamespacesByCluster[srcC], ns)
		}
	}

	return cfg
}

func (c *Config) GetObservationClusterName(obs v1.Object) string {
	if c.UseClusterNamespaces {
		return obs.GetNamespace()
	} else {
		return obs.GetLabels()[gc.LabelParentClusterName]
	}
}
