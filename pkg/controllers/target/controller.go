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

package target

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"admiralty.io/multicluster-scheduler/pkg/controller"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	listers "admiralty.io/multicluster-scheduler/pkg/generated/listers/multicluster/v1alpha1"
)

type reconciler struct {
	kubeClient kubernetes.Interface

	clusterTargetLister listers.ClusterTargetLister
	targetLister        listers.TargetLister

	installNamespace string

	mu          sync.Mutex
	targetSpecs map[string]interface{}
}

func NewController(kubeClient kubernetes.Interface, installNamespace string, clusterTargetInformer informers.ClusterTargetInformer, targetInformer informers.TargetInformer) *controller.Controller {

	r := &reconciler{
		kubeClient: kubeClient,

		clusterTargetLister: clusterTargetInformer.Lister(),
		targetLister:        targetInformer.Lister(),

		installNamespace: installNamespace,
	}

	c := controller.New("source", r,
		clusterTargetInformer.Informer().HasSynced,
		targetInformer.Informer().HasSynced)

	clusterTargetInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))
	targetInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))

	return c
}

func (c *reconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	clusterTargets, err := c.clusterTargetLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	targets, err := c.targetLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	targetSpecs := make(map[string]interface{}, len(targets)+len(clusterTargets))
	for _, t := range clusterTargets {
		targetSpecs[fmt.Sprintf("%s/%s", t.Namespace, t.Name)] = t.Spec
	}
	for _, t := range targets {
		targetSpecs[fmt.Sprintf("%s/%s", t.Namespace, t.Name)] = t.Spec
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !reflect.DeepEqual(c.targetSpecs, targetSpecs) {
		if err := c.kubeClient.CoreV1().Pods(c.installNamespace).DeleteCollection(ctx, metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: "component in (controller-manager, proxy-scheduler)"}); err != nil {
			return nil, err
		}
		c.targetSpecs = targetSpecs
	}

	return nil, nil
}
