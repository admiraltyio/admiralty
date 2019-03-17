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
	"reflect"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-controller/pkg/reference"
	"admiralty.io/multicluster-scheduler/pkg/apis"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
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
		return nil, fmt.Errorf("setting up live %T watch in agent cluster: %v", liveType, err)
	}

	if err := apis.AddToScheme(scheduler.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to scheduler cluster's scheme: %v", err)
	}
	if err := co.WatchResourceReconcileController(scheduler, observationType, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up %T watch in scheduler cluster: %v", observationType, err)
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

	obsNamespacedName := r.federationNamespacedName(req)

	live := r.liveType.DeepCopyObject()
	if err := r.agent.Get(context.TODO(), req.NamespacedName, live); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get live %T %s in namespace %s in agent cluster: %v",
				live, req.Name, req.Namespace, err)
		}
		// TODO...: multicluster garbage collector
		// Until then...
		return reconcile.Result{}, r.deleteObservation(obsNamespacedName)
	}
	setClusterName(live, req.Context)

	obs := r.observationType.DeepCopyObject()
	if err := r.scheduler.Get(context.TODO(), obsNamespacedName, obs); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get %T %s in namespace %s in scheduler cluster: %v",
				obs, obsNamespacedName.Name, obsNamespacedName.Namespace, err)
		}
		obs, err := r.makeObservation(live, obsNamespacedName)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot make observation from live %T %s in namespace %s in agent cluster: %v",
				live, req.Name, req.Namespace, err)
		}
		if err := r.scheduler.Create(context.TODO(), obs); err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot create %T %s in namespace %s in scheduler cluster: %v",
				obs, obsNamespacedName.Name, obsNamespacedName.Namespace, err)
		}
		return reconcile.Result{}, nil
	}

	ok, err := liveStateEqual(obs, live)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot compare %T to live %T: %v", obs, live, err)
	}
	if !ok {
		if err := setLiveState(obs, live); err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot set live state of %T from %T: %v", obs, live, err)
		}
		if err := r.scheduler.Update(context.TODO(), obs); err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot update %T %s in namespace %s in scheduler cluster: %v",
				obs, obsNamespacedName.Name, obsNamespacedName.Namespace, err)
		}
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
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

// deleteObservation gets an observation before deleting.
// controller-runtime doesn't seem to offer the possibility to delete by namespaced name.
func (r *reconciler) deleteObservation(nn types.NamespacedName) error {
	obs := r.observationType.DeepCopyObject()
	if err := r.scheduler.Get(context.TODO(), nn, obs); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("cannot get (to delete) %T %s in namespace %s in scheduler cluster: %v",
				obs, nn.Name, nn.Namespace, err)
		}
		// all good then
		return nil
	}
	if err := r.scheduler.Delete(context.TODO(), obs); err != nil {
		return fmt.Errorf("cannot delete %T %s in namespace %s in scheduler cluster: %v",
			obs, nn.Name, nn.Namespace, err)
	}
	return nil
}

func (r *reconciler) makeObservation(live runtime.Object, obsNamespacedName types.NamespacedName) (runtime.Object, error) {
	var obs runtime.Object
	switch live := live.(type) {
	case *v1.Pod:
		obs = &v1alpha1.PodObservation{}
	case *v1.Node:
		obs = &v1alpha1.NodeObservation{}
	case *v1alpha1.NodePool:
		obs = &v1alpha1.NodePoolObservation{}
	case *v1.Service:
		obs = &v1alpha1.ServiceObservation{}
	default:
		return nil, fmt.Errorf("type %T cannot be observed", live)
	}

	if err := setLiveState(obs, live); err != nil {
		return nil, fmt.Errorf("cannot set live state: %v", err)
	}

	liveMeta := live.(metav1.Object)
	ref := reference.NewMulticlusterOwnerReference(liveMeta, live.GetObjectKind().GroupVersionKind(), r.agentContext)
	obsMeta := obs.(metav1.Object)
	reference.SetMulticlusterControllerReference(obsMeta, ref)

	obsMeta.SetNamespace(obsNamespacedName.Namespace)
	obsMeta.SetName(obsNamespacedName.Name)

	labels := map[string]string{
		common.LabelKeyOriginalName:        liveMeta.GetName(),
		common.LabelKeyOriginalNamespace:   liveMeta.GetNamespace(),
		common.LabelKeyOriginalClusterName: liveMeta.GetClusterName(),
	}
	for k, v := range liveMeta.GetLabels() {
		labels[k] = v
	}
	obsMeta.SetLabels(labels)

	return obs, nil
}

func setClusterName(live runtime.Object, clusterName string) {
	meta := live.(metav1.Object)
	meta.SetClusterName(clusterName)
}

func liveStateEqual(obs runtime.Object, live runtime.Object) (bool, error) {
	switch obs := obs.(type) {
	case *v1alpha1.PodObservation:
		return reflect.DeepEqual(live, obs.Status.LiveState), nil
	case *v1alpha1.NodeObservation:
		return reflect.DeepEqual(live, obs.Status.LiveState), nil
	case *v1alpha1.NodePoolObservation:
		return reflect.DeepEqual(live, obs.Status.LiveState), nil
	case *v1alpha1.ServiceObservation:
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
	case *v1alpha1.ServiceObservation:
		live, ok := live.(*v1.Service)
		if !ok {
			return fmt.Errorf("type %T is not type %T's live form", live, obs)
		}
		obs.Status = v1alpha1.ServiceObservationStatus{LiveState: live}
		return nil
	default:
		return fmt.Errorf("type %T is not an observation", obs)
	}
}
