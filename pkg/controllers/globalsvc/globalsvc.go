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
	"admiralty.io/multicluster-controller/pkg/patterns"
	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-controller/pkg/reference"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	schedulerconfig "admiralty.io/multicluster-scheduler/pkg/config/scheduler"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(scheduler *cluster.Cluster, schedCfg *schedulerconfig.Config) (*controller.Controller, error) {
	client, err := scheduler.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for scheduler cluster: %v", err)
	}

	co := controller.New(&reconciler{
		client:   client,
		schedCfg: schedCfg,
	}, controller.Options{})

	if err := co.WatchResourceReconcileObjectOverrideContext(scheduler, &v1alpha1.ServiceObservation{}, controller.WatchOptions{
		Namespaces: schedCfg.Namespaces,
		CustomPredicate: func(obj interface{}) bool {
			svcObs := obj.(*v1alpha1.ServiceObservation)
			svc := svcObs.Status.LiveState
			return svc.Annotations["io.cilium/global-service"] == "true" &&
				svc.Labels[common.LabelKeyIsDelegate] != "true" // no need to globalyze a delegate service (result of other service's globalyzation)
		},
	}, ""); err != nil {
		return nil, fmt.Errorf("setting up proxy service observation watch: %v", err)
	}
	if err := co.WatchResourceReconcileController(scheduler, &v1alpha1.ServiceDecision{}, controller.WatchOptions{
		Namespaces: schedCfg.Namespaces,
	}); err != nil {
		return nil, fmt.Errorf("setting up delegate service decision watch: %v", err)
	}

	return co, nil
}

type reconciler struct {
	client   client.Client
	schedCfg *schedulerconfig.Config
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	svcObs := &v1alpha1.ServiceObservation{}
	if err := r.client.Get(context.Background(), req.NamespacedName, svcObs); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get service observation %s in namespace %s: %v", req.Name, req.Namespace, err)
		}
		// ServiceObservation was deleted
		return reconcile.Result{}, nil
	}

	svc := svcObs.Status.LiveState

	srcClusterName := r.schedCfg.GetObservationClusterName(svcObs)

	var clusters map[string]struct{}
	fedName := svc.Annotations[common.AnnotationKeyFederationName]
	if fedName == "" {
		clusters = r.schedCfg.PairedClustersByCluster[srcClusterName]
	} else {
		clusters = r.schedCfg.ClustersByFederation[fedName]
	}

	for clusterName := range clusters {
		if clusterName == srcClusterName {
			continue
		}

		delSvc := makeDelegateService(svc)

		svcDecNamespace := r.schedCfg.NamespaceForCluster[clusterName]

		l := &v1alpha1.ServiceDecisionList{}
		s := labels.SelectorFromValidatedSet(labels.Set{
			gc.LabelParentName:      svcObs.Name,
			gc.LabelParentNamespace: svcObs.Namespace,
		})
		err := r.client.List(context.Background(), &client.ListOptions{Namespace: svcDecNamespace, LabelSelector: s}, l)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf(
				"cannot list service decisions in namespace %s with label selector %s: %v",
				svcDecNamespace, s, err)
		}
		if len(l.Items) > 1 {
			return reconcile.Result{}, fmt.Errorf(
				"duplicate service decisions found in namespace %s with label selector %s: %v",
				svcDecNamespace, s, err)
		} else if len(l.Items) == 0 {
			svcDec := &v1alpha1.ServiceDecision{}
			// we use generate name to avoid (unlikely) conflicts
			genName := fmt.Sprintf("%s-%s-", svcObs.Namespace, svcObs.Name)
			if len(genName) > 253 {
				genName = genName[0:253]
			}
			svcDec.GenerateName = genName
			svcDec.Namespace = svcDecNamespace
			svcDec.Labels = labels.Set{
				gc.LabelParentName:      svcObs.Name,
				gc.LabelParentNamespace: svcObs.Namespace,
			}
			svcDec.Annotations = map[string]string{common.AnnotationKeyClusterName: clusterName}
			svcDec.Spec.Template.ObjectMeta = delSvc.ObjectMeta
			svcDec.Spec.Template.Spec = delSvc.Spec

			ref := reference.NewMulticlusterOwnerReference(svcObs, svc.GroupVersionKind(), "")
			if err := reference.SetMulticlusterControllerReference(svcDec, ref); err != nil {
				return reconcile.Result{}, fmt.Errorf(
					"cannot set controller reference on service decision %s (name not yet generated) "+
						"in namespace %s for owner %s in namespace %s: %v",
					genName, svcDecNamespace, svcObs.Name, svcObs.Namespace, err)
			}

			if err := r.client.Create(context.Background(), svcDec); err != nil {
				if !errors.IsAlreadyExists(err) {
					return reconcile.Result{}, fmt.Errorf(
						"cannot create service decision %s (name not yet generated) in namespace %s: %v",
						genName, svcDecNamespace, err)
				}
			}

			continue
		}

		svcDec := &l.Items[0]
		delSvc.Spec.ClusterIP = svcDec.Spec.Template.Spec.ClusterIP

		if deep.Equal(svcDec.Spec.Template.ObjectMeta, delSvc.ObjectMeta) == nil ||
			deep.Equal(svcDec.Spec.Template.Spec, delSvc.Spec) == nil {
			// no need to update
			continue
		}

		svcDec.Spec.Template.ObjectMeta = delSvc.ObjectMeta
		svcDec.Spec.Template.Spec = delSvc.Spec
		if err := r.client.Update(context.Background(), svcDec); err != nil && !patterns.IsOptimisticLockError(err) {
			return reconcile.Result{}, fmt.Errorf("cannot update delegate service decision %s in namespace %s: %v",
				svcDec.Name, svcDec.Namespace, err)
		}
	}

	return reconcile.Result{}, nil
}

func makeDelegateService(svc *corev1.Service) *corev1.Service {
	delSvc := &corev1.Service{}

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
