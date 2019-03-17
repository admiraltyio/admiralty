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

package globalsvc

import (
	"context"
	"fmt"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-scheduler/pkg/apis"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func NewController(scheduler *cluster.Cluster) (*controller.Controller, error) {
	client, err := scheduler.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for scheduler cluster: %v", err)
	}

	co := controller.New(&reconciler{
		client: client,
		scheme: scheduler.GetScheme(),
	}, controller.Options{})

	if err := apis.AddToScheme(scheduler.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to scheduler cluster's scheme: %v", err)
	}
	if err := co.WatchResourceReconcileObject(scheduler, &v1alpha1.ServiceObservation{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up proxy service observation watch: %v", err)
	}
	if err := co.WatchResourceReconcileController(scheduler, &v1alpha1.ServiceDecision{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up delegate service decision watch: %v", err)
	}

	return co, nil
}

type reconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	svcObs := &v1alpha1.ServiceObservation{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, svcObs); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get service observation %s in namespace %s: %v", req.Name, req.Namespace, err)
		}
		// ServiceObservation was deleted
		return reconcile.Result{}, nil
	}

	svc := svcObs.Status.LiveState

	if svc.Labels[common.LabelKeyIsDelegate] == "true" {
		// no need to globalyze a delegate service (result of other service's globalyzation)
		return reconcile.Result{}, nil
	}

	if svc.Annotations["io.cilium/global-service"] != "true" {
		return reconcile.Result{}, nil
	}

	// HACK: get all cluster names from node pools
	npObsL := &v1alpha1.NodePoolObservationList{}
	if err := r.client.List(context.TODO(), &client.ListOptions{}, npObsL); err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot list node pool observations: %v", err)
	}
	clusterNames := make(map[string]struct{})
	for _, npObs := range npObsL.Items {
		clusterNames[npObs.Status.LiveState.ClusterName] = struct{}{}
	}

	for clusterName := range clusterNames {
		if clusterName == svc.ClusterName {
			continue
		}

		delSvc := makeDelegateService(svc, clusterName)
		svcDecName := req.Name + "-" + clusterName

		svcDec := &v1alpha1.ServiceDecision{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: svcDecName, Namespace: req.Namespace}, svcDec); err != nil {
			if !errors.IsNotFound(err) {
				return reconcile.Result{}, fmt.Errorf("cannot get service decision %s in namespace %s: %v", svcDecName, req.Namespace, err)
			}

			svcDec := &v1alpha1.ServiceDecision{}
			svcDec.Name = svcDecName
			svcDec.Namespace = req.Namespace
			svcDec.Spec.Template.ObjectMeta = delSvc.ObjectMeta
			svcDec.Spec.Template.Spec = delSvc.Spec

			if err := controllerutil.SetControllerReference(svcObs, svcDec, r.scheme); err != nil {
				return reconcile.Result{}, fmt.Errorf("cannot set controller reference on service decision %s in namespace %s for owner %s in namespace %s: %v",
					svcDecName, req.Namespace, svcObs.Name, svcObs.Namespace, err)
			}

			if err := r.client.Create(context.TODO(), svcDec); err != nil {
				if !errors.IsAlreadyExists(err) {
					return reconcile.Result{}, fmt.Errorf("cannot create service decision %s in namespace %s: %v", svcDecName, req.Namespace, err)
				}
			}

			continue
		}

		delSvc.Spec.ClusterIP = svcDec.Spec.Template.Spec.ClusterIP

		if deep.Equal(svcDec.Spec.Template.ObjectMeta, delSvc.ObjectMeta) == nil ||
			deep.Equal(svcDec.Spec.Template.Spec, delSvc.Spec) == nil {
			// no need to update
			continue
		}

		svcDec.Spec.Template.ObjectMeta = delSvc.ObjectMeta
		svcDec.Spec.Template.Spec = delSvc.Spec
		if err := r.client.Update(context.TODO(), svcDec); err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot update delegate service decision %s in namespace %s: %v", svcDecName, req.Namespace, err)
		}
	}

	return reconcile.Result{}, nil
}

func makeDelegateService(svc *corev1.Service, clusterName string) *corev1.Service {
	delSvc := &corev1.Service{}

	delSvc.ClusterName = clusterName
	delSvc.Name = svc.Name
	delSvc.Namespace = svc.Namespace

	labels := make(map[string]string)
	for k, v := range svc.Labels {
		labels[k] = v
	}
	labels[common.LabelKeyIsDelegate] = "true"
	delSvc.Labels = labels

	annotations := make(map[string]string)
	for k, v := range svc.Annotations { // including "io.cilium/global-service"
		annotations[k] = v
	}
	delSvc.Annotations = annotations

	delSvc.Spec = *svc.Spec.DeepCopy()

	delSvc.Spec.ClusterIP = "" // cluster IP given by each cluster (not really a top-level spec)

	return delSvc
}
