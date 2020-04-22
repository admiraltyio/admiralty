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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO use gc pattern

func NewController(sources []*cluster.Cluster, targets []*cluster.Cluster) (*controller.Controller, error) {
	r := &reconciler{}

	co := controller.New(r, controller.Options{})

	r.sources = make(map[string]client.Client, len(sources))
	for _, clu := range sources {
		cli, err := clu.GetDelegatingClient()
		if err != nil {
			return nil, fmt.Errorf("getting delegating client for cluster %s: %v", clu.Name, err)
		}
		r.sources[clu.Name] = cli

		// note we're using the labels package to build an annotation selector
		s := labels.NewSelector()
		req, err := labels.NewRequirement("io.cilium/global-service", selection.Equals, []string{"true"})
		if err != nil {
			return nil, err
		}
		s = s.Add(*req)
		req, err = labels.NewRequirement(common.AnnotationKeyIsDelegate, selection.DoesNotExist, nil) // no need to globalyze a delegate service (result of other service's globalyzation)
		if err != nil {
			return nil, err
		}
		s = s.Add(*req)
		if err := co.WatchResourceReconcileObject(clu, &corev1.Service{}, controller.WatchOptions{AnnotationSelector: s}); err != nil {
			return nil, fmt.Errorf("setting up proxy service watch: %v", err)
		}
	}

	r.targets = make(map[string]client.Client, len(targets))
	for _, clu := range targets {
		cli, err := clu.GetDelegatingClient()
		if err != nil {
			return nil, fmt.Errorf("getting delegating client for cluster %s: %v", clu.Name, err)
		}
		r.targets[clu.Name] = cli

		// note we're using the labels package to build an annotation selector
		s := labels.NewSelector()
		req, err := labels.NewRequirement("io.cilium/global-service", selection.Equals, []string{"true"})
		if err != nil {
			return nil, err
		}
		s = s.Add(*req)
		req, err = labels.NewRequirement(common.AnnotationKeyIsDelegate, selection.Exists, nil)
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
	sources map[string]client.Client
	targets map[string]client.Client
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	srcClusterName := req.Context
	svc := &corev1.Service{}
	if err := r.sources[srcClusterName].Get(ctx, req.NamespacedName, svc); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get service %s in namespace %s in cluster %s: %v", req.Name, req.Namespace, srcClusterName, err)
		}
		// Service was deleted
		return reconcile.Result{}, nil
	}

	for targetClusterName, cli := range r.targets {
		if targetClusterName == srcClusterName {
			continue
		}

		delSvc := makeDelegateService(svc)

		foundDelSvc := &corev1.Service{}
		if err := cli.Get(ctx, req.NamespacedName, foundDelSvc); err != nil {
			if errors.IsForbidden(err) {
				// target might not authorize this namespace
				continue
			}
			if !errors.IsNotFound(err) {
				return reconcile.Result{}, fmt.Errorf("cannot get delegate service %s in namespace %s in cluster %s: %v", req.Name, req.Namespace, targetClusterName, err)
			}

			ref := reference.NewMulticlusterOwnerReference(svc, svc.GroupVersionKind(), srcClusterName)
			if err := reference.SetMulticlusterControllerReference(delSvc, ref); err != nil {
				return reconcile.Result{}, fmt.Errorf("cannot set controller reference on delegate service %s in namespace %s in cluster %s: %v", req.Name, req.Namespace, targetClusterName, err)
			}

			if err := cli.Create(ctx, delSvc); err != nil {
				if errors.IsForbidden(err) {
					// target might not authorize this namespace
					// already checked for Get (double-check for Create)
					continue
				}
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

		foundDelSvc.Labels = delSvc.Labels
		foundDelSvc.Annotations = delSvc.Annotations
		foundDelSvc.Spec = delSvc.Spec
		if err := cli.Update(ctx, foundDelSvc); err != nil {
			if errors.IsForbidden(err) {
				// target might not authorize this namespace
				// already checked for Get/Create (double-check for Update)
				continue
			}
			if !patterns.IsOptimisticLockError(err) {
				return reconcile.Result{}, fmt.Errorf("cannot update delegate service %s in namespace %s in cluster %s: %v", req.Name, req.Namespace, targetClusterName, err)
			}
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
	delSvc.Labels = labels

	annotations := make(map[string]string)
	for k, v := range svc.Annotations { // including "io.cilium/global-service"
		annotations[k] = v
	}
	annotations[common.AnnotationKeyIsDelegate] = ""
	delSvc.Annotations = annotations

	delSvc.Spec = *svc.Spec.DeepCopy()

	delSvc.Spec.ClusterIP = "" // cluster IP given by each cluster (not really a top-level spec)

	return delSvc
}
