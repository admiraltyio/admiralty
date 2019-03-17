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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(agent *cluster.Cluster, scheduler *cluster.Cluster, decisionType runtime.Object, delegateType runtime.Object) (*controller.Controller, error) {
	agentClient, err := agent.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for agent cluster: %v", err)
	}
	schedulerClient, err := scheduler.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for scheduler cluster: %v", err)
	}

	decisionGVKs, _, err := scheduler.GetScheme().ObjectKinds(decisionType)
	if err != nil {
		return nil, fmt.Errorf("getting decision group version kind: %v", err)
	}
	delegateGVKs, _, err := scheduler.GetScheme().ObjectKinds(delegateType)
	if err != nil {
		return nil, fmt.Errorf("getting delegate group version kind: %v", err)
	}

	co := controller.New(&reconciler{
		agent:        agentClient,
		scheduler:    schedulerClient,
		agentContext: agent.Name,
		decisionGVK:  decisionGVKs[0], // TODO... get preferred GVK if many
		delegateGVK:  delegateGVKs[0], // TODO... get preferred GVK if many
	}, controller.Options{})

	if err := apis.AddToScheme(scheduler.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to scheduler cluster's scheme: %v", err)
	}
	if err := co.WatchResourceReconcileObject(scheduler, decisionType, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up decision watch on scheduler cluster: %v", err)
	}

	if err := apis.AddToScheme(agent.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to agent cluster's scheme: %v", err)
	}
	// if err := co.WatchResourceReconcileController(agent, delegateType, controller.WatchOptions{}); err != nil {
	// 	return nil, fmt.Errorf("setting up delegate watch on agent cluster: %v", err)
	// }
	// TODO: when multicluster-controller implements it, use WatchResourceReconcileMulticlusterController
	h := &EnqueueRequestForMulticlusterController{Queue: co.Queue}
	if err := agent.AddEventHandler(delegateType, h); err != nil {
		return nil, err
	}

	return co, nil
}

type reconciler struct {
	agent        client.Client
	scheduler    client.Client
	agentContext string
	decisionGVK  schema.GroupVersionKind
	delegateGVK  schema.GroupVersionKind
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	decName := req.Name
	decNamespace := req.Namespace

	dec := &unstructured.Unstructured{}
	dec.SetGroupVersionKind(r.decisionGVK)
	if err := r.scheduler.Get(context.TODO(), req.NamespacedName, dec); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get decision %s in namespace %s in scheduler cluster: %v", decName, decNamespace, err)
		}
		// TODO? reconcile on child and delete child if parent doesn't exist anymore,
		// in case we allow force deletes of parents in intermittent cluster connectivity use case.
		return reconcile.Result{}, nil
	}

	tmplMeta, found, err := unstructured.NestedMap(dec.Object, "spec", "template", "metadata")
	if err != nil || !found {
		panic("bad format") // as in impossible
	}
	clusterName, found, err := unstructured.NestedString(tmplMeta, "clusterName")
	if err != nil {
		panic("bad format") // as in impossible
	}
	if !found {
		return reconcile.Result{}, fmt.Errorf("decision %s in namespace %s template missing cluster name: %v", decName, decNamespace, err)
	}
	if clusterName != r.agentContext {
		// request for other cluster, do nothing
		// TODO: filter upstream (with Watch predicate)
		return reconcile.Result{}, nil
	}
	delName, found, err := unstructured.NestedString(tmplMeta, "name")
	if err != nil {
		panic("bad format") // as in impossible
	}
	if !found {
		return reconcile.Result{}, fmt.Errorf("decision %s in namespace %s template missing name: %v", decName, decNamespace, err)
	}
	delNamespace, found, err := unstructured.NestedString(tmplMeta, "namespace")
	if err != nil {
		panic("bad format") // as in impossible
	}
	if !found {
		return reconcile.Result{}, fmt.Errorf("decision %s in namespace %s template missing namespace: %v", decName, decNamespace, err)
	}

	delFound := true
	del := &unstructured.Unstructured{}
	del.SetGroupVersionKind(r.delegateGVK)
	if err := r.agent.Get(context.TODO(), types.NamespacedName{Name: delName, Namespace: delNamespace}, del); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get delegate %s in namespace %s in agent cluster: %v", delName, delNamespace, err)
		}
		delFound = false
	}

	decTerminating := dec.GetDeletionTimestamp() != nil

	finalizers := dec.GetFinalizers()
	j := -1
	for i, f := range finalizers {
		if f == "multiclusterForegroundDeletion" {
			j = i
			break
		}
	}
	decHasFinalizer := j > -1

	if decTerminating {
		if delFound {
			if err := r.agent.Delete(context.TODO(), del); err != nil && !errors.IsNotFound(err) {
				return reconcile.Result{}, fmt.Errorf("cannot delete delegate %s in namespace %s in agent cluster: %v", delName, delNamespace, err)
			}
		} else if decHasFinalizer {
			// remove finalizer
			dec.SetFinalizers(append(finalizers[:j], finalizers[j+1:]...))
			if err := r.scheduler.Update(context.TODO(), dec); err != nil {
				return reconcile.Result{}, fmt.Errorf("cannot remove finalizer from decision %s in namespace %s in scheduler cluster: %v", decName, decNamespace, err)
			}
		}
	} else {
		if !decHasFinalizer {
			dec.SetFinalizers(append(finalizers, "multiclusterForegroundDeletion"))
			if err := r.scheduler.Update(context.TODO(), dec); err != nil {
				return reconcile.Result{}, fmt.Errorf("cannot add finalizer to decision %s in namespace %s in scheduler cluster: %v", decName, decNamespace, err)
			}
		} else if !delFound {
			// create child only after multicluster GC finalizer has been set
			del := &unstructured.Unstructured{}
			del.SetGroupVersionKind(r.delegateGVK)
			if err := unstructured.SetNestedField(del.Object, tmplMeta, "metadata"); err != nil {
				panic("bad format") // as in impossible
			}
			spec, found, err := unstructured.NestedFieldCopy(dec.Object, "spec", "template", "spec") // TODO error
			if err != nil || !found {
				panic("bad format") // as in impossible
			}
			if err := unstructured.SetNestedField(del.Object, spec, "spec"); err != nil {
				panic("bad format") // as in impossible
			}

			ref := reference.NewMulticlusterOwnerReference(dec, dec.GroupVersionKind(), req.Context)
			reference.SetMulticlusterControllerReference(del, ref)

			if err := r.agent.Create(context.TODO(), del); err != nil && !errors.IsAlreadyExists(err) {
				return reconcile.Result{}, fmt.Errorf("cannot create delegate %s in namespace %s in agent cluster: %v", delName, delNamespace, err)
			}
		}
	}

	// TODO: smart delegate pod update (only the allowed fields: container and initContainer images, etc.)

	return reconcile.Result{}, nil
}
