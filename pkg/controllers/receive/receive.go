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
	"log"
	"reflect"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-controller/pkg/reference"
	"admiralty.io/multicluster-scheduler/pkg/apis"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
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
	if err := co.WatchResourceReconcileObject(scheduler, &v1alpha1.DeploymentDecision{}, controller.WatchOptions{}); err != nil {
		return nil, err
	}

	if err := co.WatchResourceReconcileController(agent, &appsv1.Deployment{}, controller.WatchOptions{}); err != nil {
		return nil, err
	}

	return co, nil
}

type reconciler struct {
	agent        client.Client
	scheduler    client.Client
	agentContext string
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	odd := &v1alpha1.DeploymentDecision{}
	if err := r.scheduler.Get(context.TODO(), req.NamespacedName, odd); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if odd.Spec.Template.ClusterName != r.agentContext {
		// request for other cluster, do nothing
		// TODO: filter upstream (with Watch predicate)
		return reconcile.Result{}, nil
	}

	found := true
	od := &appsv1.Deployment{}
	if err := r.agent.Get(context.TODO(), types.NamespacedName{
		Namespace: odd.Spec.Template.Namespace,
		Name:      odd.Spec.Template.Name,
	}, od); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		found = false
		// TODO: remove condition when deletion done from namespace and name
		if odd.DeletionTimestamp == nil {
			dd := &appsv1.Deployment{}
			dd.ObjectMeta = odd.Spec.Template.ObjectMeta
			dd.Spec = odd.Spec.Template.Spec

			ref := reference.NewMulticlusterOwnerReference(odd, odd.GetObjectKind().GroupVersionKind(), req.Context)
			reference.SetMulticlusterControllerReference(dd, ref)

			log.Printf("create %s/%s", dd.Namespace, dd.Name)
			return reconcile.Result{}, r.agent.Create(context.TODO(), dd)
		}
	}

	// TODO: delete from namespace and name before vs. after client.Get()
	// For some reason client.Delete() requires a runtime.Object, so we could make one that just holds the required info for deletion.
	if odd.DeletionTimestamp != nil {
		var j int
		for i, f := range odd.Finalizers {
			if f == "multiclusterForegroundDeletion" {
				j = i
			}
		}
		if found {
			log.Printf("delete %s/%s", od.Namespace, od.Name)
			if err := r.agent.Delete(context.TODO(), od); err != nil && !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
		}
		odd.Finalizers = append(odd.Finalizers[:j], odd.Finalizers[j+1:]...)
		log.Printf("remove finalizer from %s/%s", odd.Namespace, odd.Name)
		return reconcile.Result{}, r.scheduler.Update(context.TODO(), odd)
	}

	if reflect.DeepEqual(od.Spec, odd.Spec.Template.Spec) {
		return reconcile.Result{}, nil
	}

	od.Spec = odd.Spec.Template.Spec
	log.Printf("update %s/%s", od.Namespace, od.Name)
	return reconcile.Result{}, r.agent.Update(context.TODO(), od)
}
