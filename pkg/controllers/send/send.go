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

package send

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
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(agent *cluster.Cluster, scheduler *cluster.Cluster, federationNamespace string,
	liveType runtime.Object, observationType runtime.Object) (*controller.Controller, error) {
	agentclient, err := agent.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for agent cluster: %v", err)
	}
	schedulerclient, err := scheduler.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for scheduler cluster: %v", err)
	}

	co := controller.New(&reconciler{
		agentContext:        agent.Name,
		agent:               agentclient,
		scheduler:           schedulerclient,
		federationNamespace: federationNamespace,
		liveType:            liveType,
		observationType:     observationType,
	}, controller.Options{})

	if err := apis.AddToScheme(agent.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to agent cluster's scheme: %v", err)
	}
	if err := co.WatchResourceReconcileObject(agent, liveType, controller.WatchOptions{}); err != nil {
		return nil, err
	}

	if err := apis.AddToScheme(scheduler.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to scheduler cluster's scheme: %v", err)
	}
	if err := co.WatchResourceReconcileController(scheduler, observationType, controller.WatchOptions{}); err != nil {
		return nil, err
	}

	return co, nil
}

type reconciler struct {
	agentContext        string
	agent               client.Client
	scheduler           client.Client
	federationNamespace string
	liveType            runtime.Object
	observationType     runtime.Object
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	if req.Context != r.agentContext {
		// request for other cluster, do nothing
		// TODO: filter upstream (with Watch predicate)
		return reconcile.Result{}, nil
	}

	o := r.liveType.DeepCopyObject()
	if err := r.agent.Get(context.TODO(), req.NamespacedName, o); err != nil {
		if errors.IsNotFound(err) {
			// TODO...: multicluster garbage collector
			// Until then...
			return reconcile.Result{}, r.deleteObservation(r.federationNamespacedName(req))
		}
		return reconcile.Result{}, err
	}
	setClusterName(o, req.Context)

	oo := r.observationType.DeepCopyObject()
	if err := r.scheduler.Get(context.TODO(), r.federationNamespacedName(req), oo); err != nil {
		if errors.IsNotFound(err) {
			doo, err := r.makeObservation(o, req)
			if err != nil {
				return reconcile.Result{}, err
			}
			log.Printf("create %v\n", r.federationNamespacedName(req))
			return reconcile.Result{}, r.scheduler.Create(context.TODO(), doo)
		}
		return reconcile.Result{}, err
	}

	ok, err := liveStateEqual(oo, o)
	if err != nil {
		return reconcile.Result{}, err
	}
	if ok {
		return reconcile.Result{}, nil
	}

	if err := setLiveState(oo, o); err != nil {
		return reconcile.Result{}, err
	}
	log.Printf("update %v\n", r.federationNamespacedName(req))
	return reconcile.Result{}, r.scheduler.Update(context.TODO(), oo)
}

func (r *reconciler) federationNamespacedName(req reconcile.Request) types.NamespacedName {
	name := req.Context
	if req.Namespace != "" {
		name += fmt.Sprintf("-%s", req.Namespace)
	}
	name += fmt.Sprintf("-%s", req.Name)

	return types.NamespacedName{
		Namespace: r.federationNamespace,
		Name:      name,
		// TODO: error if len(context) + len(namespace) + len(name) exceeds max. Name length
		// or use GenerateName and Get observed controlled object by label or field selector (using List)
	}
}

func (r *reconciler) deleteObservation(nn types.NamespacedName) error {
	obs := r.observationType.DeepCopyObject()
	if err := r.scheduler.Get(context.TODO(), nn, obs); err != nil {
		if errors.IsNotFound(err) {
			// all good
			return nil
		}
		return err
	}
	log.Printf("delete %v\n", nn)
	if err := r.scheduler.Delete(context.TODO(), obs); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) makeObservation(obj runtime.Object, req reconcile.Request) (runtime.Object, error) {
	var obs runtime.Object
	switch obj := obj.(type) {
	case *v1.Pod:
		obs = &v1alpha1.PodObservation{}
	case *v1.Node:
		obs = &v1alpha1.NodeObservation{}
	case *v1alpha1.NodePool:
		obs = &v1alpha1.NodePoolObservation{}
	case *v1alpha1.MulticlusterDeployment:
		obs = &v1alpha1.MulticlusterDeploymentObservation{}
	default:
		return nil, fmt.Errorf("type %T cannot be observed", obj)
	}

	if err := setLiveState(obs, obj); err != nil {
		return nil, err
	}

	objMeta := obj.(metav1.Object)
	ref := reference.NewMulticlusterOwnerReference(objMeta, obj.GetObjectKind().GroupVersionKind(), req.Context)
	obsMeta := obs.(metav1.Object)
	reference.SetMulticlusterControllerReference(obsMeta, ref)

	nn := r.federationNamespacedName(req)
	obsMeta.SetNamespace(nn.Namespace)
	obsMeta.SetName(nn.Name)

	return obs, nil
}

func setClusterName(o runtime.Object, clusterName string) {
	oMeta := o.(metav1.Object)
	oMeta.SetClusterName(clusterName)
}

func liveStateEqual(obs runtime.Object, live runtime.Object) (bool, error) {
	switch obs := obs.(type) {
	case *v1alpha1.PodObservation:
		return reflect.DeepEqual(live, obs.Status.LiveState), nil
	case *v1alpha1.NodeObservation:
		return reflect.DeepEqual(live, obs.Status.LiveState), nil
	case *v1alpha1.NodePoolObservation:
		return reflect.DeepEqual(live, obs.Status.LiveState), nil
	case *v1alpha1.MulticlusterDeploymentObservation:
		return reflect.DeepEqual(live, obs.Status.LiveState), nil
	default:
		return false, fmt.Errorf("type %T is not an observation", obs)
	}
}

func setLiveState(obs runtime.Object, live runtime.Object) error {
	switch obs := obs.(type) {
	case *v1alpha1.PodObservation:
		live, ok := live.(*v1.Pod)
		if !ok {
			return fmt.Errorf("type %T is not type %T's live form", live, obs)
		}
		obs.Status = v1alpha1.PodObservationStatus{LiveState: live}
		return nil
	case *v1alpha1.NodeObservation:
		live, ok := live.(*v1.Node)
		if !ok {
			return fmt.Errorf("type %T is not type %T's live form", live, obs)
		}
		obs.Status = v1alpha1.NodeObservationStatus{LiveState: live}
		return nil
	case *v1alpha1.NodePoolObservation:
		live, ok := live.(*v1alpha1.NodePool)
		if !ok {
			return fmt.Errorf("type %T is not type %T's live form", live, obs)
		}
		obs.Status = v1alpha1.NodePoolObservationStatus{LiveState: live}
		return nil
	case *v1alpha1.MulticlusterDeploymentObservation:
		live, ok := live.(*v1alpha1.MulticlusterDeployment)
		if !ok {
			return fmt.Errorf("type %T is not type %T's live form", live, obs)
		}
		obs.Status = v1alpha1.MulticlusterDeploymentObservationStatus{LiveState: live}
		return nil
	default:
		return fmt.Errorf("type %T is not an observation", obs)
	}
}
