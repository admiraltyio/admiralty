package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Agent struct {
	metav1.TypeMeta      `json:",inline"`
	UseClusterNamespaces bool     `json:"useClusterNamespaces"`
	Remotes              []Remote `json:"remotes"`
}

type Remote struct {
	Kubeconfig  string `json:"kubeconfig"`
	Context     string `json:"context"`
	ClusterName string `json:"clusterName"`
}

type Scheduler struct {
	metav1.TypeMeta      `json:",inline"`
	Clusters             []Cluster `json:"clusters"`
	UseClusterNamespaces bool      `json:"useClusterNamespaces"`
}

type Cluster struct {
	Name             string       `json:"name"`
	ClusterNamespace string       `json:"clusterNamespace"`
	Memberships      []Membership `json:"memberships"`
}

type Membership struct {
	FederationName string `json:"federationName"`
}
