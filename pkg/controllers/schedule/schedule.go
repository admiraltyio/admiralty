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

package schedule

import (
	"context"
	"fmt"
	"log"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-scheduler/pkg/apis"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"github.com/go-test/deep"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func NewController(local *cluster.Cluster, s Scheduler) (*controller.Controller, error) {
	client, err := local.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for local cluster: %v", err)
	}

	co := controller.New(&reconciler{client: client, scheme: local.GetScheme(), scheduler: s}, controller.Options{})

	if err := apis.AddToScheme(local.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to local cluster's scheme: %v", err)
	}
	if err := co.WatchResourceReconcileObject(local, &v1alpha1.MulticlusterDeploymentObservation{}, controller.WatchOptions{}); err != nil {
		return nil, err
	}
	if err := co.WatchResourceReconcileController(local, &v1alpha1.DeploymentDecision{}, controller.WatchOptions{}); err != nil {
		return nil, err
	}

	return co, nil
}

type reconciler struct {
	client    client.Client
	scheme    *runtime.Scheme
	scheduler Scheduler
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	omcdo := &v1alpha1.MulticlusterDeploymentObservation{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, omcdo); err != nil {
		if errors.IsNotFound(err) {
			// MulticlusterDeploymentObservation was deleted
			// DeploymentDecisions will be garbage-collected
			// after the corresponding agents have deleted the deployments
			// and removed the finalizers.
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("cannot get MulticlusterDeploymentObservation: %v", err)
	}

	opol := &v1alpha1.PodObservationList{}
	if err := r.client.List(context.TODO(), &client.ListOptions{}, opol); err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot list PodObservationList: %v", err)
	}
	for _, pg := range opol.Items {
		r.scheduler.SetPod(pg.Status.LiveState)
	}

	onol := &v1alpha1.NodeObservationList{}
	if err := r.client.List(context.TODO(), &client.ListOptions{}, onol); err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot list NodeObservationList: %v", err)
	}
	for _, ng := range onol.Items {
		r.scheduler.SetNode(ng.Status.LiveState)
	}

	nopol := &v1alpha1.NodePoolObservationList{}
	if err := r.client.List(context.TODO(), &client.ListOptions{}, nopol); err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot list NodePoolObservationList: %v", err)
	}
	for _, npg := range nopol.Items {
		r.scheduler.SetNodePool(npg.Status.LiveState)
	}

	oddl := &v1alpha1.DeploymentDecisionList{}
	if err := r.client.List(context.TODO(), client.MatchingLabels(map[string]string{
		"multicluster-deployment-observation-name": omcdo.Name, // TODO? use selector instead
	}), oddl); err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot list DeploymentDecisionList: %v", err)
	}
	oddm := make(map[string]v1alpha1.DeploymentDecision)
	for _, odd := range oddl.Items {
		oddm[odd.Name] = odd
	}

	dl, err := r.scheduler.Schedule(omcdo.Status.LiveState)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot schedule: %v", err)
	}

	for _, d := range dl {
		k := fmt.Sprintf("%s-%s", omcdo.Name, d.ClusterName)
		if odd, ok := oddm[k]; ok {
			if needUpdate(&odd, d) {
				odd.Spec.Template.ObjectMeta = *d.ObjectMeta.DeepCopy()
				odd.Spec.Template.Spec = *d.Spec.DeepCopy()
				log.Printf("update %s/%s", odd.Namespace, odd.Name)
				if err := r.client.Update(context.TODO(), &odd); err != nil {
					return reconcile.Result{}, fmt.Errorf("cannot update deployment decision: %v", err)
				}
			}
			delete(oddm, k)
		} else {
			ddd := &v1alpha1.DeploymentDecision{}
			ddd.Namespace = omcdo.Namespace
			ddd.Name = k
			ddd.Labels = map[string]string{"multicluster-deployment-observation-name": omcdo.Name} // TODO? use selector instead
			ddd.Finalizers = []string{"multiclusterForegroundDeletion"}
			ddd.Spec.Template.ObjectMeta = *d.ObjectMeta.DeepCopy()
			ddd.Spec.Template.Spec = *d.Spec.DeepCopy()
			if err := controllerutil.SetControllerReference(omcdo, ddd, r.scheme); err != nil {
				return reconcile.Result{}, fmt.Errorf("cannot SetControllerReference: %v", err)
			}
			log.Printf("create %s/%s", ddd.Namespace, ddd.Name)
			if err := r.client.Create(context.TODO(), ddd); err != nil {
				return reconcile.Result{}, fmt.Errorf("cannot create deployment decision: %v", err)
			}
		}
	}

	for _, odd := range oddm {
		log.Printf("delete %s/%s", odd.Namespace, odd.Name)
		if err := r.client.Delete(context.TODO(), &odd); err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot delete deployment decision: %v", err)
		}
	}

	return reconcile.Result{}, nil
}

func needUpdate(odd *v1alpha1.DeploymentDecision, d *appsv1.Deployment) bool {
	if diff := deep.Equal(odd.Spec.Template.ObjectMeta, d.ObjectMeta); diff != nil {
		log.Println(diff)
		return true
	}
	if diff := deep.Equal(odd.Spec.Template.Spec, d.Spec); diff != nil {
		log.Println(diff)
		return true
	}
	return false
}

type Scheduler interface {
	SetPod(p *corev1.Pod)
	SetNode(n *corev1.Node)
	SetNodePool(np *v1alpha1.NodePool)
	Schedule(mcd *v1alpha1.MulticlusterDeployment) ([]*appsv1.Deployment, error)
}
