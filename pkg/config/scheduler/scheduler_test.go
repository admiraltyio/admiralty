package scheduler

import (
	"testing"

	configv1alpha1 "admiralty.io/multicluster-scheduler/pkg/apis/config/v1alpha1"
	"github.com/go-test/deep"
)

func TestNew(t *testing.T) {
	raw := &configv1alpha1.Scheduler{
		Clusters: []configv1alpha1.Cluster{{
			Name: "c1",
		}, {
			Name: "c2",
		}},
	}
	cfg := New(raw, "default")
	expected := &Config{
		Namespaces:                []string{"default"},
		NamespaceForCluster:       map[string]string{"c1": "default", "c2": "default"},
		FederationsByCluster:      map[string]map[string]struct{}{"c1": {"default": {}}, "c2": {"default": {}}},
		ClustersByFederation:      map[string]map[string]struct{}{"default": {"c1": {}, "c2": {}}},
		NamespacesByFederation:    map[string][]string{"default": {"default"}},
		PairedClustersByCluster:   map[string]map[string]struct{}{"c1": {"c1": {}, "c2": {}}, "c2": {"c1": {}, "c2": {}}},
		PairedNamespacesByCluster: map[string][]string{"c1": {"default"}, "c2": {"default"}},
		UseClusterNamespaces:      false,
	}
	if diff := deep.Equal(cfg, expected); diff != nil {
		t.Errorf("diff: %v", diff)
	}
}
