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

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/patterns"
	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(clusters []*cluster.Cluster, kClients map[string]kubernetes.Interface, impersonatingKClients map[string]map[string]kubernetes.Interface, s Scheduler) (*controller.Controller, error) {
	clients := make(map[string]client.Client, len(clusters))
	pendingDecisions := make(pendingDecisions)

	r := reconciler{
		clients:          clients,
		kClients:         kClients,
		pendingDecisions: pendingDecisions,
		scheduler: schedulerShim{
			clients:               clients,
			impersonatingKClients: impersonatingKClients,
			pendingDecisions:      pendingDecisions,
			scheduler:             s,
		},
	}

	co := controller.New(r, controller.Options{})

	for _, clu := range clusters {
		cli, err := clu.GetDelegatingClient()
		if err != nil {
			return nil, fmt.Errorf("getting delegating client for cluster %s: %v", clu.Name, err)
		}
		clients[clu.Name] = cli

		if err := co.WatchResourceReconcileObject(clu, &corev1.Pod{}, controller.WatchOptions{}); err != nil {
			return nil, fmt.Errorf("setting up watch for pods in cluster %s: %v", clu.Name, err)
		}
	}

	return co, nil
}

type reconciler struct {
	clients          map[string]client.Client
	kClients         map[string]kubernetes.Interface
	scheduler        SchedulerShim
	pendingDecisions pendingDecisions
	// Note: this makes the reconciler NOT compatible with MaxConccurentReconciles > 1
	// TODO add mutex if we want concurrent reconcilers
}

func (r reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	srcClusterName := req.Context

	pod := &corev1.Pod{}
	if err := r.clients[srcClusterName].Get(context.Background(), req.NamespacedName, pod); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get %s: %v",
				r.objectErrorString(req.Name, req.Namespace, srcClusterName), err)
		}
		return reconcile.Result{}, nil
	}

	if !proxypod.IsProxy(pod) {
		// could be a delegate pod, in which case we want to remove the corresponding pod decision from the pending map
		delete(r.pendingDecisions, types.UID(pod.Labels[gc.LabelParentUID]))

		return reconcile.Result{}, nil
	}

	if proxypod.IsScheduled(pod) {
		// already scheduled
		// bind controller will check if allowed
		return reconcile.Result{}, nil
	}

	srcPod, err := proxypod.GetSourcePod(pod)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot get source pod from proxy %s: %v",
			r.objectErrorString(pod.Name, pod.Namespace, srcClusterName), err)
	}

	targetClusterName := proxypod.GetTargetClusterName(pod)
	if targetClusterName == "" {
		var err error
		targetClusterName, err = r.scheduler.Schedule(srcPod, srcClusterName)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot schedule proxy %s: %v",
				r.objectErrorString(pod.Name, pod.Namespace, srcClusterName), err)
		}
	}

	srcPod.ClusterName = targetClusterName // set ClusterName because scheduler shim expects it on pending decisions
	r.pendingDecisions[pod.UID] = srcPod   // TODO fix unlikely UID collision across clusters

	if err := proxypod.LocalBind(pod, targetClusterName, r.kClients[srcClusterName]); err != nil && !patterns.IsOptimisticLockError(err) {
		return reconcile.Result{}, fmt.Errorf("cannot bind proxy %s: %v",
			r.objectErrorString(pod.Name, pod.Namespace, srcClusterName), err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) objectErrorString(name, namespace, clusterName string) string {
	return fmt.Sprintf("pod %s in namespace %s in cluster %s", name, namespace, clusterName)
}

type pendingDecisions map[types.UID]*corev1.Pod

type SchedulerShim interface {
	Schedule(pod *corev1.Pod, srcClusterName string) (targetClusterName string, err error)
}
