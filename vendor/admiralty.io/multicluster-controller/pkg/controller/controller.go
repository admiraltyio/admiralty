/*
Copyright 2018 The Multicluster-Controller Authors.

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

// Package controller implements the controller pattern.
package controller // import "admiralty.io/multicluster-controller/pkg/controller"

import (
	"log"
	"os"
	"time"

	"admiralty.io/multicluster-controller/pkg/handler"
	"admiralty.io/multicluster-controller/pkg/manager"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Controller implements the controller pattern.
// A Controller owns a client-go workqueue. Watch methods set up the queue to receive reconcile Requests,
// e.g., on resource CRUD events in a cluster. The Requests are processed by the user-provided Reconciler.
// A Controller can watch multiple resources in multiple clusters. It saves those clusters in a set,
// so the Manager knows which caches to start and sync before starting the Controller.
type Controller struct {
	reconciler reconcile.Reconciler
	clusters   manager.CacheSet
	Options
}

// Options is used as an argument of New.
type Options struct {
	// JitterPeriod is the time to wait after an error to start working again.
	JitterPeriod time.Duration
	// MaxConcurrentReconciles is the number of concurrent control loops.
	// Use this if your Reconciler is slow, but thread safe.
	MaxConcurrentReconciles int
	// Queue can be used to override the default queue.
	Queue workqueue.RateLimitingInterface
	// Logger can be used to override the default logger.
	Logger *log.Logger
}

// Cluster decouples the controller package from the cluster package.
type Cluster interface {
	GetClusterName() string
	AddEventHandler(runtime.Object, cache.ResourceEventHandler) error
	manager.Cache
}

// New creates a new Controller.
func New(r reconcile.Reconciler, o Options) *Controller {
	c := &Controller{
		reconciler: r,
		clusters:   make(manager.CacheSet),
		Options:    o,
	}

	if c.JitterPeriod == 0 {
		c.JitterPeriod = 1 * time.Second
	}

	if c.MaxConcurrentReconciles <= 0 {
		c.MaxConcurrentReconciles = 1
	}

	if c.Queue == nil {
		c.Queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	}

	if c.Logger == nil {
		c.Logger = log.New(os.Stdout, "", log.Lshortfile)
	}

	return c
}

// WatchOptions is used as an argument of WatchResource methods (just a placeholder for now).
// TODO: consider implementing predicates.
type WatchOptions struct {
}

// WatchResourceReconcileObject configures the Controller to watch resources of the same Kind as objectType,
// in the specified cluster, generating reconcile Requests from the Cluster's context
// and the watched objects' namespaces and names.
func (c *Controller) WatchResourceReconcileObject(cluster Cluster, objectType runtime.Object, o WatchOptions) error {
	c.clusters[cluster] = struct{}{}
	h := &handler.EnqueueRequestForObject{Context: cluster.GetClusterName(), Queue: c.Queue}
	return cluster.AddEventHandler(objectType, h)
}

// WatchResourceReconcileController configures the Controller to watch resources of the same Kind as objectType,
// in the specified cluster, generating reconcile Requests from the Cluster's context
// and the namespaces and names of the watched objects' controller references.
func (c *Controller) WatchResourceReconcileController(cluster Cluster, objectType runtime.Object, o WatchOptions) error {
	c.clusters[cluster] = struct{}{}
	h := &handler.EnqueueRequestForController{Context: cluster.GetClusterName(), Queue: c.Queue}
	return cluster.AddEventHandler(objectType, h)
}

// TODO: more watch methods (owner, arbitrary mapping, channel, etc.)

// GetCaches gets the current set of clusters (which implement manager.Cache) watched by the Controller.
// Manager uses this to ensure the necessary caches are started and synced before it starts the Controller.
func (c *Controller) GetCaches() manager.CacheSet {
	return c.clusters
}

// Start starts the Controller's control loops (as many as MaxConcurrentReconciles) in separate channels
// and blocks until an empty struct is sent to the stop channel.
func (c *Controller) Start(stop <-chan struct{}) error {
	defer c.Queue.ShutDown()

	for i := 0; i < c.MaxConcurrentReconciles; i++ {
		go wait.Until(func() {
			for c.processNextWorkItem() {
			}
		}, c.JitterPeriod, stop)
	}

	<-stop
	return nil
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.Queue.Get()
	if obj == nil {
		c.Queue.Forget(obj)
	}

	if shutdown {
		c.Logger.Print("Shutting down. Ignore work item and stop working.")
		return false
	}

	defer c.Queue.Done(obj)
	var req reconcile.Request
	var ok bool
	if req, ok = obj.(reconcile.Request); !ok {
		c.Logger.Print("Work item is not a Request. Ignore it. Next.")
		c.Queue.Forget(obj)
		return true
	}

	if result, err := c.reconciler.Reconcile(req); err != nil {
		c.Logger.Print(err)
		c.Logger.Print("Could not reconcile Request. Stop working.")
		c.Queue.AddRateLimited(req)
		return false
	} else if result.RequeueAfter > 0 {
		c.Queue.AddAfter(req, result.RequeueAfter)
		return true
	} else if result.Requeue {
		c.Queue.AddRateLimited(req)
		return true
	}

	c.Queue.Forget(obj)
	return true
}
