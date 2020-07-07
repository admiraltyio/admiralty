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
	"context"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	listers "admiralty.io/multicluster-scheduler/pkg/generated/listers/multicluster/v1alpha1"
)

type upstream struct {
	kubeclientset kubernetes.Interface

	nodeLister            corelisters.NodeLister
	clusterSummaryListers map[string]listers.ClusterSummaryLister
}

func NewUpstreamController(kubeclientset kubernetes.Interface,
	nodeInformer coreinformers.NodeInformer,
	clusterSummaryInformers map[string]informers.ClusterSummaryInformer) *controller.Controller {

	r := &upstream{
		kubeclientset:         kubeclientset,
		nodeLister:            nodeInformer.Lister(),
		clusterSummaryListers: make(map[string]listers.ClusterSummaryLister, len(clusterSummaryInformers)),
	}

	informersSynced := make([]cache.InformerSynced, len(clusterSummaryInformers)+1)
	informersSynced[0] = nodeInformer.Informer().HasSynced
	i := 1
	for targetName, informer := range clusterSummaryInformers {
		r.clusterSummaryListers[targetName] = informer.Lister()
		informersSynced[i] = informer.Informer().HasSynced
		i++
	}

	c := controller.New("cluster-resources-downstream", r, informersSynced...)

	nodeInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(func(obj interface{}) {
		node := obj.(*corev1.Node)
		if node.Labels[common.LabelAndTaintKeyVirtualKubeletProvider] == common.VirtualKubeletProviderName {
			c.EnqueueKey(node.Name[10:]) // TODO move name builder to model
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
	ctx := context.Background()

	targetName := key.(string)

	clusterSummary, err := r.clusterSummaryListers[targetName].Get(singletonName)
	if err != nil {
		return nil, err
	}

	virtualNode, err := r.nodeLister.Get("admiralty-" + targetName) // TODO move name builder to model
	if err != nil {
		return nil, err
	}

	if !reflect.DeepEqual(virtualNode.Status.Capacity, clusterSummary.Capacity) ||
		!reflect.DeepEqual(virtualNode.Status.Allocatable, clusterSummary.Allocatable) {
		copy := virtualNode.DeepCopy()
		copy.Status.Allocatable = clusterSummary.Allocatable
		copy.Status.Capacity = clusterSummary.Capacity
		_, err = r.kubeclientset.CoreV1().Nodes().UpdateStatus(ctx, copy, v1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}
