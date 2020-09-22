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

package resources

import (
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	listers "admiralty.io/multicluster-scheduler/pkg/generated/listers/multicluster/v1alpha1"
)

type NodeStatusUpdater interface {
	UpdateNodeStatus(node *corev1.Node)
}

type upstream struct {
	kubeclientset kubernetes.Interface

	nodeLister            corelisters.NodeLister
	clusterSummaryListers map[string]listers.ClusterSummaryLister
	nodeStatusUpdaters    map[string]NodeStatusUpdater
}

func NewUpstreamController(kubeclientset kubernetes.Interface,
	nodeInformer coreinformers.NodeInformer,
	clusterSummaryInformers map[string]informers.ClusterSummaryInformer,
	nodeStatusUpdaters map[string]NodeStatusUpdater) *controller.Controller {

	r := &upstream{
		kubeclientset:         kubeclientset,
		nodeLister:            nodeInformer.Lister(),
		clusterSummaryListers: make(map[string]listers.ClusterSummaryLister, len(clusterSummaryInformers)),
		nodeStatusUpdaters:    nodeStatusUpdaters,
	}

	informersSynced := make([]cache.InformerSynced, len(clusterSummaryInformers)+1)
	informersSynced[0] = nodeInformer.Informer().HasSynced
	i := 1
	for targetName, informer := range clusterSummaryInformers {
		r.clusterSummaryListers[targetName] = informer.Lister()
		informersSynced[i] = informer.Informer().HasSynced
		i++
	}

	c := controller.New("cluster-resources-upstream", r, informersSynced...)

	nodeInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(func(obj interface{}) {
		node := obj.(*corev1.Node)
		if node.Labels[common.LabelAndTaintKeyVirtualKubeletProvider] == common.VirtualKubeletProviderName {
			c.EnqueueKey(node.Name)
		}
	}))
	for targetName, informer := range clusterSummaryInformers {
		informer.Informer().AddEventHandler(controller.HandleAllWith(func(_ interface{}) {
			c.EnqueueKey(targetName)
		}))
	}

	return c
}

func (r upstream) Handle(key interface{}) (requeueAfter *time.Duration, err error) {
	targetName := key.(string)

	clusterSummary, err := r.clusterSummaryListers[targetName].Get(singletonName)
	if err != nil {
		return nil, err
	}

	// if HugePageStorageMediumSize is enabled in target cluster but not in source cluster,
	// virtual node status update would fail if we included multiple huge page sizes in capacity or allocatable
	// in that case, we purge all page sizes (because there's no reason to keep one over the others)
	// huge pages requests are still respected, thanks to our proxy-candidate scheduling algorithm
	// TODO: find a good way to e2e test this, because kind inherits huge page sizes from host,
	// so we can't have two kind clusters with different huge page sizes on test host.

	// hack to infer HugePageStorageMediumSize Kubernetes feature flag from other nodes
	sel, err := labels.Parse(common.LabelAndTaintKeyVirtualKubeletProvider + "!=" + common.VirtualKubeletProviderName)
	nodes, err := r.nodeLister.List(sel)
	nodesHaveMultipleHugePageSizes := false
	for _, n := range nodes {
		if hasMultipleHugePageSizes(n.Status.Capacity) || hasMultipleHugePageSizes(n.Status.Allocatable) {
			nodesHaveMultipleHugePageSizes = true
			break
		}
	}
	clusterSummaryHasMultipleHugePageSizes := hasMultipleHugePageSizes(clusterSummary.Capacity) || hasMultipleHugePageSizes(clusterSummary.Allocatable)
	if clusterSummaryHasMultipleHugePageSizes && !nodesHaveMultipleHugePageSizes {
		clusterSummary = clusterSummary.DeepCopy()
		purgeHugePageResources(clusterSummary.Capacity)
		purgeHugePageResources(clusterSummary.Allocatable)
	}

	virtualNode, err := r.nodeLister.Get(targetName)
	if err != nil {
		return nil, err
	}

	if !reflect.DeepEqual(virtualNode.Status.Capacity, clusterSummary.Capacity) ||
		!reflect.DeepEqual(virtualNode.Status.Allocatable, clusterSummary.Allocatable) {
		actualCopy := virtualNode.DeepCopy()
		actualCopy.Status.Allocatable = clusterSummary.Allocatable
		actualCopy.Status.Capacity = clusterSummary.Capacity
		r.nodeStatusUpdaters[targetName].UpdateNodeStatus(actualCopy)
		// VK doesn't surface errors, so we have no way to requeue if transient error, TODO? fix upstream
	}

	return nil, nil
}

func hasMultipleHugePageSizes(rl corev1.ResourceList) bool {
	n := 0
	for res, qty := range rl {
		if helper.IsHugePageResourceName(res) && !qty.IsZero() {
			n += 1
			if n > 1 {
				return true
			}
		}
	}
	return false
}

func purgeHugePageResources(rl corev1.ResourceList) {
	var keys []corev1.ResourceName
	for res, qty := range rl {
		if helper.IsHugePageResourceName(res) && !qty.IsZero() {
			keys = append(keys, res)
		}
	}
	for _, k := range keys {
		delete(rl, k)
	}
}
