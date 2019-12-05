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
	"fmt"
	"reflect"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var AllObservations map[runtime.Object]runtime.Object

func init() {
	pvobs := &unstructured.Unstructured{}
	pvobs.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "multicluster.admiralty.io",
		Version: "v1alpha1",
		Kind:    "PersistentVolumeObservation"})
	pvcobs := &unstructured.Unstructured{}
	pvcobs.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "multicluster.admiralty.io",
		Version: "v1alpha1",
		Kind:    "PersistentVolumeClaimObservation"})
	rcobs := &unstructured.Unstructured{}
	rcobs.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "multicluster.admiralty.io",
		Version: "v1alpha1",
		Kind:    "ReplicationControllerObservation"})
	rsobs := &unstructured.Unstructured{}
	rsobs.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "multicluster.admiralty.io",
		Version: "v1alpha1",
		Kind:    "ReplicaSetObservation"})
	ssobs := &unstructured.Unstructured{}
	ssobs.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "multicluster.admiralty.io",
		Version: "v1alpha1",
		Kind:    "StatefulSetObservation"})
	pdbobs := &unstructured.Unstructured{}
	pdbobs.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "multicluster.admiralty.io",
		Version: "v1alpha1",
		Kind:    "PodDisruptionBudgetObservation"})
	scobs := &unstructured.Unstructured{}
	scobs.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "multicluster.admiralty.io",
		Version: "v1alpha1",
		Kind:    "StorageClassObservation"})

	AllObservations = map[runtime.Object]runtime.Object{
		&corev1.Service{}:                    &v1alpha1.ServiceObservation{},
		&corev1.Pod{}:                        &v1alpha1.PodObservation{},
		&corev1.Node{}:                       &v1alpha1.NodeObservation{},
		&v1alpha1.NodePool{}:                 &v1alpha1.NodePoolObservation{},
		&corev1.PersistentVolume{}:           pvobs,
		&corev1.PersistentVolumeClaim{}:      pvcobs,
		&corev1.ReplicationController{}:      rcobs,
		&appsv1.ReplicaSet{}:                 rsobs,
		&appsv1.StatefulSet{}:                ssobs,
		&policyv1beta1.PodDisruptionBudget{}: pdbobs,
		&storagev1.StorageClass{}:            scobs,
	}
}

func NewController(agent *cluster.Cluster, scheduler *cluster.Cluster, observationNamespace string,
	liveType runtime.Object, observationType runtime.Object) (*controller.Controller, error) {
	return gc.NewController(agent, scheduler, gc.Options{
		ParentPrototype: liveType,
		ChildPrototype:  observationType,
		ChildNamespace:  observationNamespace,
		Applier:         applier{},
	})
}

type applier struct{}

var _ gc.Applier = applier{}

func (a applier) MakeChild(parent interface{}, expectedChild interface{}) error {
	live, err := runtime.DefaultUnstructuredConverter.ToUnstructured(parent)
	if err != nil {
		return err // TODO
	}
	obs, err := runtime.DefaultUnstructuredConverter.ToUnstructured(expectedChild)
	if err != nil {
		return err // TODO
	}

	if err := unstructured.SetNestedField(obs, map[string]interface{}{"liveState": live}, "status"); err != nil {
		return fmt.Errorf("cannot set live state: %v", err)
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obs, expectedChild); err != nil {
		return err // TODO
	}
	return nil
}

func (a applier) ChildNeedsUpdate(parent interface{}, child interface{}, _ interface{}) (bool, error) {
	live := parent.(runtime.Object)
	liveCopy := live.DeepCopyObject()
	obs := child.(runtime.Object)
	obsCopy := obs.DeepCopyObject()

	liveCopyU, err := runtime.DefaultUnstructuredConverter.ToUnstructured(liveCopy)
	if err != nil {
		return false, err // TODO
	}
	obsCopyU, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obsCopy)
	if err != nil {
		return false, err // TODO
	}

	obsLiveState, found, err := unstructured.NestedMap(obsCopyU, "status", "liveState")
	if !found {
		return false, err
	}
	if err != nil {
		return false, err
	}

	// target clustername annotation is updated by the scheduler, so don't use it for comparison
	err = unstructured.SetNestedField(obsLiveState, "", "metadata", "annotations", common.AnnotationKeyClusterName)
	if err != nil {
		panic(err)
	}
	err = unstructured.SetNestedField(liveCopyU, "", "metadata", "annotations", common.AnnotationKeyClusterName)
	if err != nil {
		panic(err)
	}

	return !reflect.DeepEqual(liveCopyU, obsLiveState), nil
}

func (a applier) MutateChild(parent interface{}, child interface{}, _ interface{}) error {
	liveU, err := runtime.DefaultUnstructuredConverter.ToUnstructured(parent)
	if err != nil {
		return err // TODO
	}
	obsU, err := runtime.DefaultUnstructuredConverter.ToUnstructured(child)
	if err != nil {
		return err // TODO
	}

	// backup target clustername annotation, which is updated by scheduler
	targetClusterName, found, err := unstructured.NestedString(obsU, "status", "liveState", "metadata", "annotations", common.AnnotationKeyClusterName)
	if err != nil {
		panic(err)
	}
	err = unstructured.SetNestedField(obsU, map[string]interface{}{"liveState": liveU}, "status")
	if err != nil {
		panic(err)
	}
	if found {
		err = unstructured.SetNestedField(obsU, targetClusterName, "status", "liveState", "metadata", "annotations", common.AnnotationKeyClusterName)
		if err != nil {
			panic(err)
		}
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obsU, child); err != nil {
		return err // TODO
	}
	return nil
}
