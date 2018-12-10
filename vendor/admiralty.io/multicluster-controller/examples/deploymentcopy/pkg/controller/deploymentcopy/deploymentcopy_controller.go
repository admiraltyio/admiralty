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

package deploymentcopy // import "admiralty.io/multicluster-controller/examples/deploymentcopy/pkg/controller/deploymentcopy"

import (
	"context"
	"fmt"
	"reflect"

	"admiralty.io/multicluster-controller/pkg/reference"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(src *cluster.Cluster, dst *cluster.Cluster) (*controller.Controller, error) {
	srcClient, err := src.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for source cluster: %v", err)
	}
	dstClient, err := dst.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for destination cluster: %v", err)
	}

	co := controller.New(&reconciler{source: srcClient, destination: dstClient}, controller.Options{})

	if err := co.WatchResourceReconcileObject(src, &appsv1.Deployment{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up Deployment watch in source cluster: %v", err)
	}
	if err := co.WatchResourceReconcileController(dst, &appsv1.Deployment{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up Deployment watch in destination cluster: %v", err)
	}

	return co, nil
}

type reconciler struct {
	source      client.Client
	destination client.Client
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	p := &appsv1.Deployment{}
	if err := r.source.Get(context.TODO(), req.NamespacedName, p); err != nil {
		if errors.IsNotFound(err) {
			// ...TODO: multicluster garbage collector
			// Until then...
			err := r.deleteCopy(req.NamespacedName)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, err
	}

	dc := makeCopy(p)
	reference.SetMulticlusterControllerReference(dc, reference.NewMulticlusterOwnerReference(p, p.GroupVersionKind(), req.Context))

	oc := &appsv1.Deployment{}
	if err := r.destination.Get(context.TODO(), req.NamespacedName, oc); err != nil {
		if errors.IsNotFound(err) {
			err := r.destination.Create(context.TODO(), dc)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, err
	}

	if reflect.DeepEqual(dc.Spec, oc.Spec) {
		return reconcile.Result{}, nil
	}

	oc.Spec = dc.Spec
	err := r.destination.Update(context.TODO(), oc)
	return reconcile.Result{}, err
}

func (r *reconciler) deleteCopy(nsn types.NamespacedName) error {
	g := &appsv1.Deployment{}
	if err := r.destination.Get(context.TODO(), nsn, g); err != nil {
		if errors.IsNotFound(err) {
			// all good
			return nil
		}
		return err
	}
	if err := r.destination.Delete(context.TODO(), g); err != nil {
		return err
	}
	return nil
}

func makeCopy(d *appsv1.Deployment) *appsv1.Deployment {
	spec := d.Spec.DeepCopy()
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.Namespace,
			Name:      d.Name,
		},
		Spec: *spec,
	}
}
