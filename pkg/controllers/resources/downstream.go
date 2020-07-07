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
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"

	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	clientset "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
)

type downstream struct {
	customclientset clientset.Interface

	nodeLister corelisters.NodeLister
}

const key = "key"
const singletonName = "singleton"

func NewDownstreamController(customclientset clientset.Interface,
	nodeInformer coreinformers.NodeInformer) *controller.Controller {

	r := &downstream{
		customclientset: customclientset,
		nodeLister:      nodeInformer.Lister(),
	}

	c := controller.New("cluster-resources-downstream", r, nodeInformer.Informer().HasSynced)
	nodeInformer.Informer().AddEventHandler(controller.HandleAllWith(func(obj interface{}) {
		node := obj.(*corev1.Node)
		if node.Labels[common.LabelAndTaintKeyVirtualKubeletProvider] != common.VirtualKubeletProviderName {
			c.EnqueueKey(key)
		}
	}))

	return c
}

func (r downstream) Handle(_ interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	capacity := corev1.ResourceList{}
	allocatable := corev1.ResourceList{}

	sel, err := labels.Parse(fmt.Sprintf("%s!=%s", common.LabelAndTaintKeyVirtualKubeletProvider, common.VirtualKubeletProviderName))
	utilruntime.Must(err)
	nodes, err := r.nodeLister.List(sel)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		for res, qty := range node.Status.Capacity {
			if val, ok := capacity[res]; ok {
				val.Add(qty)
				capacity[res] = val
			} else {
				capacity[res] = qty
			}
		}
		for res, qty := range node.Status.Allocatable {
			if val, ok := allocatable[res]; ok {
				val.Add(qty)
				allocatable[res] = val
			} else {
				allocatable[res] = qty
			}
		}
	}

	ClusterSummary, err := r.customclientset.MulticlusterV1alpha1().ClusterSummaries().Get(ctx, singletonName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			gold := &v1alpha1.ClusterSummary{
				Allocatable: allocatable,
				Capacity:    capacity,
			}
			gold.Name = singletonName
			ClusterSummary, err = r.customclientset.MulticlusterV1alpha1().ClusterSummaries().Create(ctx, gold, v1.CreateOptions{})
			if err != nil {
				return nil, err
			}
		}
	}

	if !reflect.DeepEqual(capacity, ClusterSummary.Capacity) || !reflect.DeepEqual(allocatable, ClusterSummary.Allocatable) {
		copy := ClusterSummary.DeepCopy()
		copy.Allocatable = allocatable
		copy.Capacity = capacity
		ClusterSummary, err = r.customclientset.MulticlusterV1alpha1().ClusterSummaries().Update(ctx, copy, v1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}
