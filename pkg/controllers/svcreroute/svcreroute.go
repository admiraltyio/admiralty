/*
Copyright 2019 The Multicluster-Scheduler Authors.

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

package svcreroute

import (
	"context"
	"fmt"
	"strings"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/patterns"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-scheduler/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(agent *cluster.Cluster) (*controller.Controller, error) {
	client, err := agent.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for agent cluster: %v", err)
	}

	co := controller.New(&reconciler{
		client: client,
	}, controller.Options{})

	// we watch endpoints to see if their listed pods are proxy pods
	if err := co.WatchResourceReconcileObject(agent, &corev1.Endpoints{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up endpoints watch: %v", err)
	}
	// we watch services because they are updated in the loop,
	// and if those updates fail with an optimistic lock error
	// we must requeue when we receive the cache is updated
	if err := co.WatchResourceReconcileObject(agent, &corev1.Service{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up service watch: %v", err)
	}
	// a service and its endpoints object have the same name/namespace, i.e., the same reconcile key

	return co, nil
}

type reconciler struct {
	client client.Client
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ep := &corev1.Endpoints{}
	if err := r.client.Get(context.Background(), req.NamespacedName, ep); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get endpoints %s in namespace %s: %v", req.Name, req.Namespace, err)
		}
		// Endpoints was deleted
		return reconcile.Result{}, nil
	}

	shouldReroute, err := r.shouldReroute(ep)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot determine whether service %s in namespace %s should be rerouted: %v", req.Name, req.Namespace, err)
	}

	if !shouldReroute {
		return reconcile.Result{}, nil
	}

	svc := &corev1.Service{}
	if err := r.client.Get(context.Background(), req.NamespacedName, svc); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get service %s in namespace %s: %v", req.Name, req.Namespace, err)
		}
		// Service was deleted
		return reconcile.Result{}, nil
	}

	needUpdate := false

	labels := make(map[string]string)
	for k, v := range svc.Spec.Selector {
		keySplit := strings.Split(k, "/")
		if len(keySplit) == 1 || keySplit[0] != common.KeyPrefix {
			needUpdate = true
			newKey := common.KeyPrefix + keySplit[len(keySplit)-1]
			labels[newKey] = v
		} else {
			labels[k] = v
		}
	}
	svc.Spec.Selector = labels

	if svc.Annotations["io.cilium/global-service"] != "true" {
		needUpdate = true
		if svc.Annotations == nil {
			svc.Annotations = make(map[string]string)
		}
		svc.Annotations["io.cilium/global-service"] = "true"
	}

	if !needUpdate {
		return reconcile.Result{}, nil
	}

	if err := r.client.Update(context.Background(), svc); err != nil && !patterns.IsOptimisticLockError(err) {
		return reconcile.Result{}, fmt.Errorf("cannot update service %s in namespace %s: %v", req.Name, req.Namespace, err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) shouldReroute(ep *corev1.Endpoints) (bool, error) {
	for _, s := range ep.Subsets {
		for _, a := range s.Addresses {
			if a.TargetRef != nil && a.TargetRef.Kind == "Pod" {
				key := types.NamespacedName{Name: a.TargetRef.Name, Namespace: a.TargetRef.Namespace}
				pod := &corev1.Pod{}
				if err := r.client.Get(context.Background(), key, pod); err != nil {
					if !errors.IsNotFound(err) {
						return false, fmt.Errorf("cannot get pod %s in namespace %s: %v", key.Name, key.Namespace, err)
					}
					// Pod was deleted, check next endpoint
					continue
				}
				_, isProxy := pod.Annotations[common.AnnotationKeyElect]
				return isProxy, nil
			}
			// we only support pod-only endpoints
			return false, nil
		}
	}
	return false, nil
}
