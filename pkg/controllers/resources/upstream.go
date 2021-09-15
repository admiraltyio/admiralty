/*
 * Copyright 2021 The Multicluster-Scheduler Authors.
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
	"context"
	"fmt"
	"reflect"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	listers "admiralty.io/multicluster-scheduler/pkg/generated/listers/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/model/virtualnode"
)

type NodeStatusUpdater interface {
	UpdateNodeStatus(node *corev1.Node)
}

type upstream struct {
	targetName string

	kubeclientset kubernetes.Interface

	nodeLister           corelisters.NodeLister
	clusterSummaryLister listers.ClusterSummaryLister
	nodeStatusUpdater    NodeStatusUpdater

	excludedLabelsRegexp *regexp.Regexp
}

func NewUpstreamController(
	targetName string,
	kubeclientset kubernetes.Interface,
	nodeInformer coreinformers.NodeInformer,
	clusterSummaryInformer informers.ClusterSummaryInformer,
	nodeStatusUpdater NodeStatusUpdater,
	excludedLabelsRegexp *string,
) *controller.Controller {

	r := &upstream{
		targetName:           targetName,
		kubeclientset:        kubeclientset,
		nodeLister:           nodeInformer.Lister(),
		clusterSummaryLister: clusterSummaryInformer.Lister(),
		nodeStatusUpdater:    nodeStatusUpdater,
	}

	c := controller.New("cluster-resources-upstream", r, nodeInformer.Informer().HasSynced, clusterSummaryInformer.Informer().HasSynced)

	// node informer doesn't use field selector on metadata.name == targetName
	// because we use its cache to list other nodes to infer multi-huge-page support
	// so we need to filter here
	nodeInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(func(obj interface{}) {
		node := obj.(*corev1.Node)
		if node.Name == r.targetName {
			c.EnqueueKey(node.Name)
		}
	}))
	clusterSummaryInformer.Informer().AddEventHandler(controller.HandleAllWith(func(_ interface{}) {
		c.EnqueueKey(targetName)
	}))

	if excludedLabelsRegexp != nil {
		var err error
		r.excludedLabelsRegexp, err = regexp.Compile(*excludedLabelsRegexp)
		if err != nil {
			// don't crash if regexp cannot be compiled
			// TODO reject Target at admission
			utilruntime.HandleError(fmt.Errorf("cannot compile excluded aggregated labels regexp for target %s: %v", targetName, err))
		}
	}

	return c
}

func (r upstream) Handle(key interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()
	targetName := key.(string)

	clusterSummary, err := r.clusterSummaryLister.Get(singletonName)
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

	l := r.reconcileLabels(clusterSummary.Labels)

	// we can't group status update with label update because status update POSTs to the status subresource
	// also, we use patch, not update, because for some reason EKS cloud controller deletes nodes if we use update
	if !labels.Equals(virtualNode.Labels, l) {
		actualCopy := virtualNode.DeepCopy()
		actualCopy.Labels = l

		oldData, err := json.Marshal(virtualNode)
		if err != nil {
			return nil, err
		}

		newData, err := json.Marshal(actualCopy)
		if err != nil {
			return nil, err
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
		if err != nil {
			return nil, err
		}

		virtualNode, err = r.kubeclientset.CoreV1().Nodes().Patch(ctx, targetName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return nil, err
		}
	}

	if !reflect.DeepEqual(virtualNode.Status.Capacity, clusterSummary.Capacity) ||
		!reflect.DeepEqual(virtualNode.Status.Allocatable, clusterSummary.Allocatable) {
		actualCopy := virtualNode.DeepCopy()
		actualCopy.Status.Allocatable = clusterSummary.Allocatable
		actualCopy.Status.Capacity = clusterSummary.Capacity
		// we use nodeStatusUpdater instead of kubeclientset because VK needs to update its internal representation
		// otherwise it would override our changes
		r.nodeStatusUpdater.UpdateNodeStatus(actualCopy)
		// VK doesn't surface errors, so we have no way to requeue if transient error, TODO? fix upstream
	}

	return nil, nil
}

func (r upstream) reconcileLabels(clusterSummaryLabels map[string]string) map[string]string {
	l := virtualnode.BaseLabels()
	for k, v := range clusterSummaryLabels {
		regExp := r.excludedLabelsRegexp
		if regExp == nil || !regExp.MatchString(fmt.Sprintf("%s=%s", k, v)) {
			l[k] = v
		}
	}
	return l
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
