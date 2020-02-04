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

package globalsvc

import (
	"context"
	"fmt"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/patterns"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-controller/pkg/reference"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"github.com/go-test/deep"
	v1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(clusters []*cluster.Cluster, kClients map[string]map[string]kubernetes.Interface) (*controller.Controller, error) {
	r := &reconciler{kClients: kClients}

	co := controller.New(r, controller.Options{})

	r.clients = make(map[string]client.Client, len(clusters))
	for _, clu := range clusters {
		cli, err := clu.GetDelegatingClient()
		if err != nil {
			return nil, fmt.Errorf("getting delegating client for cluster %s: %v", clu.Name, err)
		}
		r.clients[clu.Name] = cli

		s := labels.NewSelector()
		req, err := labels.NewRequirement("io.cilium/global-service", selection.Equals, []string{"true"})
		if err != nil {
			return nil, err
		}
		s = s.Add(*req)
		req, err = labels.NewRequirement(common.LabelKeyIsDelegate, selection.NotEquals, []string{"true"}) // no need to globalyze a delegate service (result of other service's globalyzation)
		if err != nil {
			return nil, err
		}
		s = s.Add(*req)
		if err := co.WatchResourceReconcileObject(clu, &corev1.Service{}, controller.WatchOptions{AnnotationSelector: s}); err != nil {
			return nil, fmt.Errorf("setting up proxy service watch: %v", err)
		}

		s = labels.NewSelector()
		req, err = labels.NewRequirement("io.cilium/global-service", selection.Equals, []string{"true"})
		if err != nil {
			return nil, err
		}
		s = s.Add(*req)
		req, err = labels.NewRequirement(common.LabelKeyIsDelegate, selection.Equals, []string{"true"})
		if err != nil {
			return nil, err
		}
		s = s.Add(*req)
		if err := co.WatchResourceReconcileController(clu, &corev1.Service{}, controller.WatchOptions{AnnotationSelector: s}); err != nil {
			return nil, fmt.Errorf("setting up delegate service watch: %v", err)
		}
	}

	return co, nil
}

type reconciler struct {
	clients  map[string]client.Client
	kClients map[string]map[string]kubernetes.Interface
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	srcClusterName := req.Context
	svc := &corev1.Service{}
	if err := r.clients[srcClusterName].Get(context.Background(), req.NamespacedName, svc); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get service %s in namespace %s in cluster %s: %v", req.Name, req.Namespace, srcClusterName, err)
		}
		// Service was deleted
		return reconcile.Result{}, nil
	}

	for targetClusterName, cli := range r.clients {
		if targetClusterName == srcClusterName {
			continue
		}

		sar := &v1.SelfSubjectAccessReview{
			Spec: v1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &v1.ResourceAttributes{
					Namespace: svc.Namespace,
					Verb:      "create",
					Version:   "v1",
					Resource:  "services",
				},
			},
		}
		sar, err := r.kClients[srcClusterName][targetClusterName].AuthorizationV1().SelfSubjectAccessReviews().Create(sar)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !sar.Status.Allowed {
			continue
		}

		delSvc := makeDelegateService(svc)

		foundDelSvc := &corev1.Service{}
		if err := cli.Get(context.Background(), req.NamespacedName, foundDelSvc); err != nil {
			if !errors.IsNotFound(err) {
				return reconcile.Result{}, fmt.Errorf("cannot get delegate service %s in namespace %s in cluster %s: %v", req.Name, req.Namespace, targetClusterName, err)
			}

			ref := reference.NewMulticlusterOwnerReference(svc, svc.GroupVersionKind(), srcClusterName)
			if err := reference.SetMulticlusterControllerReference(delSvc, ref); err != nil {
				return reconcile.Result{}, fmt.Errorf("cannot set controller reference on delegate service %s in namespace %s in cluster %s: %v", req.Name, req.Namespace, targetClusterName, err)
			}

			_, err := r.kClients[srcClusterName][targetClusterName].CoreV1().Services(delSvc.Namespace).Create(delSvc)
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					return reconcile.Result{}, fmt.Errorf("cannot create delegate service %s in namespace %s in cluster %s: %v", req.Name, req.Namespace, targetClusterName, err)
				}
			}

			continue
		}

		delSvc.Spec.ClusterIP = foundDelSvc.Spec.ClusterIP

		if deep.Equal(delSvc.Labels, foundDelSvc.Labels) == nil &&
			deep.Equal(delSvc.Annotations, foundDelSvc.Annotations) == nil &&
			deep.Equal(delSvc.Spec, foundDelSvc.Spec) == nil {
			// no need to update
			continue
		}

		foundDelSvc.ObjectMeta = delSvc.ObjectMeta
		foundDelSvc.Spec = delSvc.Spec
		_, err = r.kClients[srcClusterName][targetClusterName].CoreV1().Services(delSvc.Namespace).Update(foundDelSvc)
		if err != nil && !patterns.IsOptimisticLockError(err) {
			return reconcile.Result{}, fmt.Errorf("cannot update delegate service %s in namespace %s in cluster %s: %v", req.Name, req.Namespace, targetClusterName, err)
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
