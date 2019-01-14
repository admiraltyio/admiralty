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

package scheduler

import (
	"fmt"
	"sort"

	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/controllers/nodepool"
	corev1 "k8s.io/api/core/v1"
)

type Scheduler struct {
	nodePools map[string][]*v1alpha1.NodePool // by ClusterName
	nodes     map[string][]*corev1.Node       // by ClusterName + NodePool Name
	pods      map[string][]*corev1.Pod        // by ClusterName + Node Name
}

func New() *Scheduler {
	s := &Scheduler{}
	s.nodePools = make(map[string][]*v1alpha1.NodePool)
	s.nodes = make(map[string][]*corev1.Node)
	s.pods = make(map[string][]*corev1.Pod)
	return s
}

func (s *Scheduler) SetNodePool(np *v1alpha1.NodePool) {
	clusterName := np.ClusterName
	s.nodePools[clusterName] = append(s.nodePools[clusterName], np)
}

func (s *Scheduler) SetNode(n *corev1.Node) {
	nodePoolName := n.Labels[nodepool.NodePoolLabel]
	nodePoolKey := n.ClusterName + nodePoolName
	s.nodes[nodePoolKey] = append(s.nodes[nodePoolKey], n)
}

func (s *Scheduler) SetPod(p *corev1.Pod) {
	nodeName := p.Spec.NodeName
	nodeKey := p.ClusterName + nodeName
	s.pods[nodeKey] = append(s.pods[nodeKey], p)
}

func (s *Scheduler) Schedule(pod *corev1.Pod) (string, error) {
	nClusters := len(s.nodePools) // s.nodePools is a map of node pool slices by cluster name
	if nClusters == 0 {
		return "", fmt.Errorf("no cluster to schedule to")
	}

	// sort cluster names to schedule deterministically
	clusterNames := make([]string, 0, nClusters)
	for clusterName := range s.nodePools {
		clusterNames = append(clusterNames, clusterName)
	}
	sort.Sort(sort.StringSlice(clusterNames))

	// map each cluster name to the next cluster name alphabetically
	nextClusterName := make(map[string]string, nClusters)
	for i, clusterName := range clusterNames {
		nextClusterName[clusterName] = clusterNames[(i+1)%nClusters]
	}

	return nextClusterName[pod.ClusterName], nil
}
