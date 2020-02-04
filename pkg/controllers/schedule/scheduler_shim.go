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

package schedule

import (
	"context"
	"fmt"

	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
	v1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type schedulerShim struct {
	clients               map[string]client.Client
	impersonatingKClients map[string]map[string]kubernetes.Interface
	pendingDecisions      pendingDecisions
	scheduler             Scheduler
}

func (r schedulerShim) Schedule(srcPod *corev1.Pod, srcClusterName string) (string, error) {
	r.scheduler.Reset()

	if err := r.getObservations(srcClusterName, srcPod.Namespace, labels.SelectorFromSet(srcPod.Spec.NodeSelector)); err != nil {
		return "", fmt.Errorf("cannot get observations: %v", err)
	}
	// get pending pod decisions so we can count the requests (cluster targeted but no pod obs yet)
	// otherwise a bunch of pods would be scheduled to one cluster before it would appear to be busy
	for _, pod := range r.pendingDecisions {
		r.scheduler.SetPod(pod)
	}

	return r.scheduler.Schedule(srcPod)
}

func (r *schedulerShim) getObservations(srcClusterName, namespace string, nodeSelector labels.Selector) error {
	req, err := labels.NewRequirement("virtual-kubelet.io/provider", selection.NotEquals, []string{"admiralty"})
	utilruntime.Must(err)
	s := nodeSelector.Add(*req)

	for clusterName, cli := range r.clients {
		sar := &v1.SelfSubjectAccessReview{
			Spec: v1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &v1.ResourceAttributes{
					Namespace: namespace,
					Verb:      "create",
					Version:   "v1",
					Resource:  "pods",
				},
			},
		}
		sar, err := r.impersonatingKClients[srcClusterName][clusterName].AuthorizationV1().SelfSubjectAccessReviews().Create(sar)
		if err != nil {
			return err
		}
		if !sar.Status.Allowed {
			continue
		}

		podL := &corev1.PodList{}
		if err := cli.List(context.Background(), podL); err != nil {
			return fmt.Errorf("cannot list pods: %v", err)
		}
		for _, pod := range podL.Items {
			phase := pod.Status.Phase
			if !proxypod.IsProxy(&pod) && (phase == corev1.PodPending || phase == corev1.PodRunning) {
				pod.ClusterName = clusterName
				r.scheduler.SetPod(&pod)
			}
		}

		nodeL := &corev1.NodeList{}
		if err := cli.List(context.Background(), nodeL, client.MatchingLabelsSelector{Selector: s}); err != nil {
			return fmt.Errorf("cannot list node observations: %v", err)
		}
		for _, node := range nodeL.Items {
			node.ClusterName = clusterName
			r.scheduler.SetNode(&node)
		}

		npL := &v1alpha1.NodePoolList{} // TODO respect node label selector when scheduling to elastic node pool
		if err := cli.List(context.Background(), npL); err != nil {
			return fmt.Errorf("cannot list node pool observations: %v", err)
		}
		for _, np := range npL.Items {
			np.ClusterName = clusterName
			r.scheduler.SetNodePool(&np)
		}
	}

	return nil
}

type Scheduler interface {
	Reset()
	SetPod(p *corev1.Pod)
	SetNode(n *corev1.Node)
	SetNodePool(np *v1alpha1.NodePool)
	Schedule(p *corev1.Pod) (string, error)
}
