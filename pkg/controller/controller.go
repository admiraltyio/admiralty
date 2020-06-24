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

package controller

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"admiralty.io/multicluster-scheduler/pkg/common"
)

type Reconciler interface {
	Handle(key interface{}) (requeueAfter *time.Duration, err error)
}

type Controller struct {
	name            string
	informersSynced []cache.InformerSynced
	reconciler      Reconciler
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
}

func New(name string, reconciler Reconciler, informersSynced ...cache.InformerSynced) *Controller {
	return &Controller{
		name:            name,
		informersSynced: informersSynced,
		reconciler:      reconciler,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
	}
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Infof("Starting %s controller", c.name)

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.informersSynced...); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch workers to process resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	key, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer c.workqueue.Done(key)

	requeueAfter, err := c.reconciler.Handle(key)
	if err != nil {
		// Put the item back on the workqueue to handle any transient errors.
		c.workqueue.AddRateLimited(key)
		utilruntime.HandleError(fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error()))
		return true
	}
	if requeueAfter != nil {
		c.workqueue.AddAfter(key, *requeueAfter)
		return true
	}
	// Finally, if no error occurs we Forget this item so it does not
	// get queued again until another change happens.
	c.workqueue.Forget(key)
	klog.Infof("Successfully synced '%s'", key)
	return true
}

func (c *Controller) EnqueueKey(key interface{}) {
	c.workqueue.Add(key)
}

func (c *Controller) EnqueueObject(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

type GetOwner func(namespace string, name string) (metav1.Object, error)

func (c *Controller) EnqueueController(ownerKind string, getOwner GetOwner) func(obj interface{}) {
	return func(obj interface{}) {
		object := obj.(metav1.Object)
		if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
			if ownerRef.Kind != ownerKind {
				return
			}

			owner, err := getOwner(object.GetNamespace(), ownerRef.Name)
			if err != nil {
				klog.V(4).Infof("ignoring orphaned object '%s' of owner '%s'", object.GetSelfLink(), ownerRef.Name)
				return
			}

			c.EnqueueObject(owner)
			return
		}
	}
}

func (c *Controller) EnqueueRemoteController(ownerKind string, getOwner GetOwner) func(obj interface{}) {
	return func(obj interface{}) {
		object := obj.(metav1.Object)
		l := object.GetLabels()
		if parentUID, ok := l[common.LabelKeyParentUID]; ok {
			parentNamespace := l[common.LabelKeyParentNamespace]
			if parentNamespace == "" {
				parentNamespace = object.GetNamespace()
			}
			parentName := l[common.LabelKeyParentName]
			if parentName == "" {
				parentName = object.GetName()
			}
			owner, err := getOwner(parentNamespace, parentName)
			if err != nil {
				return
			}

			if string(owner.GetUID()) != parentUID {
				// TODO handle unlikely yet possible cross-cluster UID conflict with signing
				return
			}

			c.EnqueueObject(owner)
			return
		}
	}
}
