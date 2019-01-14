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

package receive

import (
	"context"
	"fmt"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-controller/pkg/reference"
	"admiralty.io/multicluster-scheduler/pkg/apis"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(agent *cluster.Cluster, scheduler *cluster.Cluster) (*controller.Controller, error) {
	agentClient, err := agent.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for agent cluster: %v", err)
	}
	schedulerClient, err := scheduler.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for scheduler cluster: %v", err)
	}

	co := controller.New(&reconciler{
		agent:        agentClient,
		scheduler:    schedulerClient,
		agentContext: agent.Name,
	}, controller.Options{})

	if err := apis.AddToScheme(scheduler.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to scheduler cluster's scheme: %v", err)
	}
	if err := co.WatchResourceReconcileObject(scheduler, &v1alpha1.PodDecision{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up delegate pod decision watch on scheduler cluster: %v", err)
	}

	if err := co.WatchResourceReconcileController(agent, &corev1.Pod{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up delegate pod watch on agent cluster: %v", err)
	}

	return co, nil
}

type reconciler struct {
	agent        client.Client
	scheduler    client.Client
	agentContext string
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	podDec := &v1alpha1.PodDecision{}
	if err := r.scheduler.Get(context.TODO(), req.NamespacedName, podDec); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get delegate pod decision %s in namespace %s in scheduler cluster: %v", req.Name, req.Namespace, err)
		}
		return reconcile.Result{}, nil
	}

	podDecTmpl := &podDec.Spec.Template
	if podDecTmpl.ClusterName != r.agentContext {
		// request for other cluster, do nothing
		// TODO: filter upstream (with Watch predicate)
		return reconcile.Result{}, nil
	}

	found := true
	pod := &corev1.Pod{}
	if err := r.agent.Get(context.TODO(), types.NamespacedName{
		Namespace: podDecTmpl.Namespace,
		Name:      podDecTmpl.Name,
	}, pod); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get delegate pod %s in namespace %s in agent cluster: %v", podDecTmpl.Name, podDecTmpl.Namespace, err)
		}
		found = false

		if podDec.DeletionTimestamp == nil {
			pod := &corev1.Pod{}
			pod.ObjectMeta = podDecTmpl.ObjectMeta
			pod.Spec = podDecTmpl.Spec

			ref := reference.NewMulticlusterOwnerReference(podDec, podDec.GetObjectKind().GroupVersionKind(), req.Context)
			reference.SetMulticlusterControllerReference(pod, ref)

			if err := r.agent.Create(context.TODO(), pod); err != nil {
				return reconcile.Result{}, fmt.Errorf("cannot create delegate pod %s in namespace %s in agent cluster: %v", pod.Name, pod.Namespace, err)
			}

			podDec.Finalizers = append(podDec.Finalizers, "multiclusterForegroundDeletion")
			if err := r.scheduler.Update(context.TODO(), podDec); err != nil {
				return reconcile.Result{}, fmt.Errorf("cannot add finalizer to delegate pod decision %s in namespace %s in scheduler cluster: %v", podDec.Name, podDec.Namespace, err)
			}
			return reconcile.Result{}, nil
		}
	}

	if podDec.DeletionTimestamp != nil {
		var j int
		for i, f := range podDec.Finalizers {
			if f == "multiclusterForegroundDeletion" {
				j = i
			}
		}
		if found {
			if err := r.agent.Delete(context.TODO(), pod); err != nil && !errors.IsNotFound(err) {
				return reconcile.Result{}, fmt.Errorf("cannot delete delegate pod %s in namespace %s in agent cluster: %v", pod.Name, pod.Namespace, err)
			}
		}
		podDec.Finalizers = append(podDec.Finalizers[:j], podDec.Finalizers[j+1:]...)
		if err := r.scheduler.Update(context.TODO(), podDec); err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot remove finalizer from delegate pod decision %s in namespace %s in scheduler cluster: %v", podDec.Name, podDec.Namespace, err)
		}
		return reconcile.Result{}, nil
	}

	// TODO: smart delegate pod update (only the allowed fields: container and initContainer images, etc.)

	return reconcile.Result{}, nil
}
