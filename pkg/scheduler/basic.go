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
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/controllers/nodepool"
	"gopkg.in/inf.v0"
	corev1 "k8s.io/api/core/v1"
)

type Scheduler struct {
	nodePools map[string][]*v1alpha1.NodePool // by ClusterName
	nodes     map[string][]*corev1.Node       // by ClusterName + NodePool Name
	pods      map[string][]*corev1.Pod        // by ClusterName + Node Name
}

func New() *Scheduler {
	s := &Scheduler{}
	s.Reset()
	return s
}

func (s *Scheduler) Reset() {
	s.nodePools = make(map[string][]*v1alpha1.NodePool)
	s.nodes = make(map[string][]*corev1.Node)
	s.pods = make(map[string][]*corev1.Pod)
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
	podReq := podResourceRequests(&pod.Spec)
	sched := s.scheduleToIncompressibleNodes(podReq)
	clusterName := sched.max()
	if clusterName != "" {
		return clusterName, nil
	}
	// For now, if no cluster can accommodate the delegate pod, let the original cluster deal with it.
	return pod.ClusterName, nil
}

type schedule map[string]*inf.Dec

func (sched schedule) max() string {
	clusterName, max := "", inf.NewDec(-1, 0)
	for c, n := range sched {
		// golang actively returns random order of keys, so in case of equality, a random cluster should be targeted
		if n.Cmp(max) == 1 {
			clusterName, max = c, n
		}
	}
	return clusterName
}

func (s *Scheduler) scheduleToIncompressibleNodes(podReq corev1.ResourceList) schedule {
	sched := make(schedule)
	// iterate over clusters
	for clusterName, nps := range s.nodePools {
		sched[clusterName] = new(inf.Dec)
		// iterate over node pools
		for _, np := range nps {
			ns := s.nodes[np.ClusterName+np.Name]
			// iterate over nodes
			for _, n := range ns {
				maxNode := new(inf.Dec)
				maxNodeSet := false
				ps := s.pods[n.ClusterName+n.Name]
				for res, qa := range available(n, ps) { // TODO... respect max allocatable pods
					if qr := podReq[res]; !qr.IsZero() {
						maxRes := new(inf.Dec).QuoRound(qa.AsDec(), qr.AsDec(), 0, inf.RoundDown)
						if !maxNodeSet || maxRes.Cmp(maxNode) == -1 {
							maxNode = maxRes
							maxNodeSet = true
						}
					}
				}
				if maxNodeSet {
					sched[clusterName] = new(inf.Dec).Add(sched[clusterName], maxNode)
				}
			}
		}
	}
	return sched
}

func available(n *corev1.Node, ps []*corev1.Pod) corev1.ResourceList {
	a := corev1.ResourceList{}
	for k, v := range n.Status.Allocatable {
		q := a[k]
		q.Add(v)
		a[k] = q
	}
	for _, p := range ps {
		podReq := podResourceRequests(&p.Spec)
		for k, v := range podReq {
			q := a[k]
			q.Sub(v)
			a[k] = q
		}
	}
	return a
}

func podResourceRequests(p *corev1.PodSpec) corev1.ResourceList {
	podReq := make(corev1.ResourceList)
	for _, c := range p.Containers {
		for k, v := range c.Resources.Requests {
			q := podReq[k]
			q.Add(v)
			podReq[k] = q
		}
	}
	return podReq
}
