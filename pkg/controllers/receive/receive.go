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

package receive

import (
	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var AllDecisions = map[runtime.Object]runtime.Object{
	&v1alpha1.PodDecision{}:     &corev1.Pod{},
	&v1alpha1.ServiceDecision{}: &corev1.Service{},
}

func NewController(agent *cluster.Cluster, scheduler *cluster.Cluster, decisionNamespace string,
	decisionType runtime.Object, delegateType runtime.Object) (*controller.Controller, error) {
	return gc.NewController(scheduler, agent, gc.Options{
		ParentPrototype: decisionType,
		ChildPrototype:  delegateType,
		ParentWatchOptions: controller.WatchOptions{
			Namespace: decisionNamespace,
			AnnotationSelector: labels.SelectorFromValidatedSet(labels.Set{
				common.AnnotationKeyClusterName: agent.GetClusterName()})},
		Applier: applier{},
	})
}

type applier struct{}

var _ gc.Applier = applier{}

func (a applier) MakeChild(parent interface{}, expectedChild interface{}) error {
	dec, err := runtime.DefaultUnstructuredConverter.ToUnstructured(parent)
	if err != nil {
		return err // TODO
	}
	del, err := runtime.DefaultUnstructuredConverter.ToUnstructured(expectedChild)
	if err != nil {
		return err // TODO
	}

	meta, found, err := unstructured.NestedMap(dec, "spec", "template", "metadata")
	if err != nil || !found {
		panic("bad format") // as in impossible
	}
	spec, found, err := unstructured.NestedFieldCopy(dec, "spec", "template", "spec")
	if err != nil || !found {
		panic("bad format") // as in impossible
	}
	if err := unstructured.SetNestedField(del, meta, "metadata"); err != nil {
		panic("bad format") // as in impossible
	}
	if err := unstructured.SetNestedField(del, spec, "spec"); err != nil {
		panic("bad format") // as in impossible
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(del, expectedChild); err != nil {
		return err // TODO
	}
	return nil
}

func (a applier) ChildNeedsUpdate(interface{}, interface{}, interface{}) (bool, error) {
	return false, nil
}

func (a applier) MutateChild(interface{}, interface{}, interface{}) error {
	return nil
}
