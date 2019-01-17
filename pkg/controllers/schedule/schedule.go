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
	"admiralty.io/multicluster-controller/pkg/reference"
	"admiralty.io/multicluster-scheduler/pkg/apis"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"github.com/ghodss/yaml"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func NewController(local *cluster.Cluster, s Scheduler) (*controller.Controller, error) {
	client, err := local.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for local cluster: %v", err)
	}

	co := controller.New(&reconciler{
		client:           client,
		scheme:           local.GetScheme(),
		scheduler:        s,
		pendingDecisions: make(map[string]*v1alpha1.PodDecision),
	}, controller.Options{})

	if err := apis.AddToScheme(local.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to local cluster's scheme: %v", err)
	}
	if err := co.WatchResourceReconcileObject(local, &v1alpha1.PodObservation{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up proxy pod observation watch: %v", err)
	} // TODO: filter on annotation (proxy pod observations only)
	if err := co.WatchResourceReconcileController(local, &v1alpha1.PodDecision{}, controller.WatchOptions{}); err != nil {
		return nil, fmt.Errorf("setting up delegate pod decision watch: %v", err)
	}

	return co, nil
}

type reconciler struct {
	client           client.Client
	scheme           *runtime.Scheme
	scheduler        Scheduler
	pendingDecisions map[string]*v1alpha1.PodDecision // Note: this makes the reconciler NOT compatible with MaxConccurentReconciles > 1
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	proxyPodObs := &v1alpha1.PodObservation{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, proxyPodObs); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get proxy pod observation %s in namespace %s: %v", req.Name, req.Namespace, err)
		}
		// PodObservation was deleted:
		// PodDecision will be garbage-collected after the corresponding agent has deleted the delegate Pod
		// and removed the finalizer from the PodDecision.
		return reconcile.Result{}, nil
	}

	proxyPod := proxyPodObs.Status.LiveState
	if _, ok := proxyPod.Annotations[common.AnnotationKeyElect]; !ok {
		// not a proxy pod

		// but it could be a delegate pod, in which case we want to remove the corresponding pod decision from the pending map
		ref := reference.GetMulticlusterControllerOf(proxyPod)
		if ref != nil && ref.Kind == "PodDecision" {
			log.Printf("deleting pending pod decision %s", ref.Name)
			delete(r.pendingDecisions, ref.Name)
		}

		return reconcile.Result{}, nil
	}

	delegatePod, err := r.makeDelegatePod(proxyPodObs)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot make delegate pod from proxy pod observation %s in namespace %s: %v", req.Name, req.Namespace, err)
	}

	delegatePodDec := &v1alpha1.PodDecision{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, delegatePodDec); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get delegate pod decision %s in namespace %s: %v", req.Name, req.Namespace, err)
		}
		return reconcile.Result{}, r.createDelegatePodDecision(delegatePod, proxyPodObs)
	}

	// no rescheduling
	delegatePod.ClusterName = delegatePodDec.Spec.Template.ClusterName
	if needUpdate(delegatePodDec, delegatePod) {
		return reconcile.Result{}, r.updateDelegatePodDecision(delegatePodDec, delegatePod)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) makeDelegatePod(proxyPodObs *v1alpha1.PodObservation) (*corev1.Pod, error) {
	proxyPod := proxyPodObs.Status.LiveState
	srcPodManifest, ok := proxyPod.Annotations[common.AnnotationKeySourcePodManifest]
	if !ok {
		return nil, fmt.Errorf("no source pod manifest on proxy pod")
	}
	srcPod := &corev1.Pod{}
	if err := yaml.Unmarshal([]byte(srcPodManifest), srcPod); err != nil {
		return nil, fmt.Errorf("cannot unmarshal source pod manifest: %v", err)
	}

	annotations := make(map[string]string)
	for k, v := range srcPod.Annotations {
		if k != common.AnnotationKeyElect { // we don't want to mc-schedule the delegate pod
			annotations[k] = v
		}
	}
	annotations[common.AnnotationKeyProxyPodClusterName] = proxyPod.ClusterName
	annotations[common.AnnotationKeyProxyPodNamespace] = proxyPod.Namespace
	annotations[common.AnnotationKeyProxyPodName] = proxyPod.Name

	delegatePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        proxyPodObs.Name,
			Namespace:   proxyPod.Namespace, // already defaults to "default" (vs. could be empty in srcPod)
			ClusterName: proxyPod.ClusterName,
			Annotations: annotations},
		Spec: *srcPod.Spec.DeepCopy()}

	removeServiceAccount(delegatePod)
	// TODO? add compatible fields instead of removing incompatible ones
	// (for forward compatibility and we've certainly forgotten incompatible fields...)
	// TODO... maybe make this configurable, sort of like Federation v2 Overrides

	r.scheduler.Reset()
	if err := r.getObservations(); err != nil {
		return nil, fmt.Errorf("cannot get observations: %v", err)
	}

	if clusterName, ok := annotations[common.AnnotationKeyClusterName]; ok {
		delegatePod.ClusterName = clusterName
	} else {
		// TODO: pod observations pending cluster if not enough resources and node pools not elastic
		// rather than send back to original cluster
		clusterName, err := r.scheduler.Schedule(delegatePod)
		if err != nil {
			return nil, fmt.Errorf("cannot schedule: %v", err)
		}
		delegatePod.ClusterName = clusterName
	}

	return delegatePod, nil
}

func removeServiceAccount(pod *corev1.Pod) {
	var saSecretName string
	for i, c := range pod.Spec.Containers {
		j := -1
		for i, m := range c.VolumeMounts {
			if m.MountPath == "/var/run/secrets/kubernetes.io/serviceaccount" {
				saSecretName = m.Name
				j = i
				break
			}
		}
		if j > -1 {
			c.VolumeMounts = append(c.VolumeMounts[:j], c.VolumeMounts[j+1:]...)
			pod.Spec.Containers[i] = c
		}
	}
	j := -1
	for i, v := range pod.Spec.Volumes {
		if v.Name == saSecretName {
			j = i
			break
		}
	}
	if j > -1 {
		pod.Spec.Volumes = append(pod.Spec.Volumes[:j], pod.Spec.Volumes[j+1:]...)
	}
}

func (r *reconciler) getObservations() error {
	podObsL := &v1alpha1.PodObservationList{}
	if err := r.client.List(context.TODO(), &client.ListOptions{}, podObsL); err != nil {
		return fmt.Errorf("cannot list pod observations: %v", err)
	}
	for _, podObs := range podObsL.Items {
		phase := podObs.Status.LiveState.Status.Phase
		if phase == corev1.PodPending || phase == corev1.PodRunning {
			r.scheduler.SetPod(podObs.Status.LiveState)
		}
	}

	// get pending pod decisions so we can count the requests (cluster targeted but no pod obs yet)
	// otherwise a bunch of pods would be scheduled to one cluster before it would appear to be busy
	for _, podDec := range r.pendingDecisions {
		r.scheduler.SetPod(&corev1.Pod{
			ObjectMeta: podDec.Spec.Template.ObjectMeta,
			Spec:       podDec.Spec.Template.Spec,
		})
	}

	nodeObsL := &v1alpha1.NodeObservationList{}
	if err := r.client.List(context.TODO(), &client.ListOptions{}, nodeObsL); err != nil {
		return fmt.Errorf("cannot list node observations: %v", err)
	}
	for _, nodeObs := range nodeObsL.Items {
		r.scheduler.SetNode(nodeObs.Status.LiveState)
	}

	npObsL := &v1alpha1.NodePoolObservationList{}
	if err := r.client.List(context.TODO(), &client.ListOptions{}, npObsL); err != nil {
		return fmt.Errorf("cannot list node pool observations: %v", err)
	}
	for _, npObs := range npObsL.Items {
		r.scheduler.SetNodePool(npObs.Status.LiveState)
	}

	return nil
}

func (r *reconciler) createDelegatePodDecision(delegatePod *corev1.Pod, proxyPodObs *v1alpha1.PodObservation) error {
	delegatePodDec := &v1alpha1.PodDecision{}
	delegatePodDec.Namespace = proxyPodObs.Namespace
	delegatePodDec.Name = proxyPodObs.Name
	delegatePodDec.Spec.Template.ObjectMeta = *delegatePod.ObjectMeta.DeepCopy()
	delegatePodDec.Spec.Template.Spec = *delegatePod.Spec.DeepCopy()
	if err := controllerutil.SetControllerReference(proxyPodObs, delegatePodDec, r.scheme); err != nil {
		return fmt.Errorf("cannot set controller reference on delegate pod decision %s in namespace %s for owner %s in namespace %s: %v",
			delegatePodDec.Name, delegatePodDec.Namespace, proxyPodObs.Name, proxyPodObs.Namespace, err)
	}
	if err := r.client.Create(context.TODO(), delegatePodDec); err != nil {
		return fmt.Errorf("cannot create delegate pod decision %s in namespace %s: %v", delegatePodDec.Name, delegatePodDec.Namespace, err)
	}
	log.Printf("adding pending pod decision %s", delegatePodDec.Name)
	r.pendingDecisions[delegatePodDec.Name] = delegatePodDec
	return nil
}

func needUpdate(delegatePodDec *v1alpha1.PodDecision, delegatePod *corev1.Pod) bool {
	if diff := deep.Equal(delegatePodDec.Spec.Template.ObjectMeta, delegatePod.ObjectMeta); diff != nil {
		return true
	}
	if diff := deep.Equal(delegatePodDec.Spec.Template.Spec, delegatePod.Spec); diff != nil {
		return true
	}
	return false
}

func (r *reconciler) updateDelegatePodDecision(delegatePodDec *v1alpha1.PodDecision, delegatePod *corev1.Pod) error {
	delegatePodDec.Spec.Template.ObjectMeta = *delegatePod.ObjectMeta.DeepCopy()
	delegatePodDec.Spec.Template.Spec = *delegatePod.Spec.DeepCopy()
	if err := r.client.Update(context.TODO(), delegatePodDec); err != nil {
		return fmt.Errorf("cannot update delegate pod decision %s in namespace %s: %v", delegatePodDec.Name, delegatePodDec.Namespace, err)
	}
	return nil
}

type Scheduler interface {
	Reset()
	SetPod(p *corev1.Pod)
	SetNode(n *corev1.Node)
	SetNodePool(np *v1alpha1.NodePool)
	Schedule(p *corev1.Pod) (string, error)
}
