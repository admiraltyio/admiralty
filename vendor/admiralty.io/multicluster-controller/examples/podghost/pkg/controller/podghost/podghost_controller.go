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

package podghost // import "admiralty.io/multicluster-controller/examples/podghost/pkg/controller/podghost"

import (
	"context"
	"fmt"
	"reflect"

	"admiralty.io/multicluster-controller/pkg/reference"

	"admiralty.io/multicluster-controller/examples/podghost/pkg/apis"
	"admiralty.io/multicluster-controller/examples/podghost/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(live *cluster.Cluster, ghost *cluster.Cluster, ghostNamespace string) (*controller.Controller, error) {
	liveclient, err := live.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for live cluster: %v", err)
	}
	ghostclient, err := ghost.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for ghost cluster: %v", err)
	}

	co := controller.New(&reconciler{live: liveclient, ghost: ghostclient, ghostNamespace: ghostNamespace}, controller.Options{})

	if err := co.WatchResourceReconcileObject(live, &v1.Pod{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up Pod watch in live cluster: %v", err)
	}

	if err := apis.AddToScheme(ghost.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to ghost cluster's scheme: %v", err)
	}
	// Note: At the moment, all clusters share the same scheme under the hood
	// (k8s.io/client-go/kubernetes/scheme.Scheme), yet multicluster-controller gives each cluster a scheme pointer.
	// Therefore, if we needed a custom resource in multiple clusters, we would redundantly
	// add it to each cluster's scheme, which points to the same underlying scheme.
	if err := co.WatchResourceReconcileController(ghost, &v1alpha1.PodGhost{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up PodGhost watch in ghost cluster: %v", err)
	}

	return co, nil
}

type reconciler struct {
	live           client.Client
	ghost          client.Client
	ghostNamespace string
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	p := &v1.Pod{}
	if err := r.live.Get(context.TODO(), req.NamespacedName, p); err != nil {
		if errors.IsNotFound(err) {
			// ...TODO: multicluster garbage collector
			// Until then...
			err := r.deleteGhost(r.ghostNamespacedName(req.NamespacedName))
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, err
	}

	dg := r.makeGhost(p)
	reference.SetMulticlusterControllerReference(dg, reference.NewMulticlusterOwnerReference(p, p.GroupVersionKind(), req.Context))

	og := &v1alpha1.PodGhost{}
	if err := r.ghost.Get(context.TODO(), r.ghostNamespacedName(req.NamespacedName), og); err != nil {
		if errors.IsNotFound(err) {
			err := r.ghost.Create(context.TODO(), dg)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, err
	}

	if reflect.DeepEqual(dg.Spec, og.Spec) {
		return reconcile.Result{}, nil
	}

	og.Spec = dg.Spec
	err := r.ghost.Update(context.TODO(), og)
	return reconcile.Result{}, err
}

func (r *reconciler) ghostNamespacedName(pod types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: r.ghostNamespace,
		Name:      fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
	}
}

func (r *reconciler) deleteGhost(nsn types.NamespacedName) error {
	g := &v1alpha1.PodGhost{}
	if err := r.ghost.Get(context.TODO(), nsn, g); err != nil {
		if errors.IsNotFound(err) {
			// all good
			return nil
		}
		return err
	}
	if err := r.ghost.Delete(context.TODO(), g); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) makeGhost(pod *v1.Pod) *v1alpha1.PodGhost {
	return &v1alpha1.PodGhost{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.ghostNamespace,
			Name:      fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
		},
	}
}
