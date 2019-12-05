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

package bind

import (
	"fmt"
	"strings"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	schedulerconfig "admiralty.io/multicluster-scheduler/pkg/config/scheduler"
	"github.com/ghodss/yaml"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewController(c *cluster.Cluster, schedCfg *schedulerconfig.Config) (*controller.Controller, error) {
	// we can optimize GC's getChild if we know that children can only be in one namespace
	// (there could actually be a further optimization since we know which cluster, hence which namespace, when we list)
	// besides, RBAC requires a namespaced list when not in clusterNamespace mode
	childNamespace := ""
	if len(schedCfg.Namespaces) == 1 {
		childNamespace = schedCfg.Namespaces[0]
	}
	return gc.NewController(c, c, gc.Options{
		ParentPrototype: &v1alpha1.PodObservation{},
		ChildPrototype:  &v1alpha1.PodDecision{},
		ParentWatchOptions: controller.WatchOptions{
			Namespaces: schedCfg.Namespaces,
			CustomPredicate: func(obj interface{}) bool {
				podObs := obj.(*v1alpha1.PodObservation)
				if podObs.Status.LiveState.Annotations == nil {
					return false
				}
				_, isProxy := podObs.Status.LiveState.Annotations[common.AnnotationKeyElect]
				clusterName, isScheduled := podObs.Status.LiveState.Annotations[common.AnnotationKeyClusterName]
				srcClusterName := schedCfg.GetObservationClusterName(podObs)
				_, isAllowed := schedCfg.PairedClustersByCluster[srcClusterName][clusterName]
				return isProxy && isScheduled && isAllowed
			},
		},
		ChildNamespace:             childNamespace,
		Applier:                    applier{schedCfg: schedCfg},
		MakeExpectedChildWhenFound: true,
	})
}

type applier struct {
	schedCfg *schedulerconfig.Config
}

var _ gc.Applier = applier{}

func (a applier) MakeChild(parent interface{}, expectedChild interface{}) error {
	proxyPodObs := parent.(*v1alpha1.PodObservation)
	delegatePodDec := expectedChild.(*v1alpha1.PodDecision)

	delegatePod, err := a.makeDelegatePod(proxyPodObs)
	if err != nil {
		return err
	}

	delegatePodDec.Spec.Template.ObjectMeta = delegatePod.ObjectMeta
	delegatePodDec.Spec.Template.Spec = delegatePod.Spec

	clusterName := proxyPodObs.Status.LiveState.Annotations[common.AnnotationKeyClusterName]
	delegatePodDec.Namespace = a.schedCfg.NamespaceForCluster[clusterName]
	delegatePodDec.Annotations = map[string]string{common.AnnotationKeyClusterName: clusterName}
	return nil
}

func (a applier) ChildNeedsUpdate(_ interface{}, child interface{}, expectedChild interface{}) (bool, error) {
	delegatePodDec := child.(*v1alpha1.PodDecision)
	expectedDelegatePodDec := expectedChild.(*v1alpha1.PodDecision)

	if diff := deep.Equal(delegatePodDec.Spec.Template.ObjectMeta, expectedDelegatePodDec.Spec.Template.ObjectMeta); diff != nil {
		return true, nil
	}
	if diff := deep.Equal(delegatePodDec.Spec.Template.Spec, expectedDelegatePodDec.Spec.Template.Spec); diff != nil {
		return true, nil
	}
	return false, nil
}

func (a applier) MutateChild(_ interface{}, child interface{}, expectedChild interface{}) error {
	delegatePodDec := child.(*v1alpha1.PodDecision)
	expectedDelegatePodDec := expectedChild.(*v1alpha1.PodDecision)

	delegatePodDec.Spec.Template.ObjectMeta = expectedDelegatePodDec.Spec.Template.ObjectMeta
	delegatePodDec.Spec.Template.Spec = expectedDelegatePodDec.Spec.Template.Spec
	return nil
}

func (a applier) makeDelegatePod(proxyPodObs *v1alpha1.PodObservation) (*corev1.Pod, error) {
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
		if !strings.HasPrefix(k, common.KeyPrefix) || k == common.AnnotationKeyFederationName {
			// we don't want to mc-schedule the delegate pod with elect,
			// and the target cluster name and source pod manifest are now redundant
			// we only keep the federation name and user annotations
			annotations[k] = v
		}
	}

	labels := make(map[string]string)
	for k, v := range srcPod.Labels {
		// we need to change the labels so as not to confuse potential controller of proxy pod, e.g., replica set
		// if the original label key has a domain prefix, replace it with ours
		// if it doesn't, add our domain prefix
		// TODO: resolve conflict two keys have same name but different prefixes
		// TODO: ensure we don't go over length limits
		keySplit := strings.Split(k, "/") // note: assume no empty key (enforced by Kubernetes)
		newKey := common.KeyPrefix + keySplit[len(keySplit)-1]
		labels[newKey] = v
	}
	labels[common.LabelKeyProxyPodClusterName] = a.schedCfg.GetObservationClusterName(proxyPodObs)
	labels[common.LabelKeyProxyPodNamespace] = proxyPod.Namespace
	labels[common.LabelKeyProxyPodName] = proxyPod.Name

	delegatePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   proxyPod.Namespace, // already defaults to "default" (vs. could be empty in srcPod)
			Labels:      labels,
			Annotations: annotations},
		Spec: *srcPod.Spec.DeepCopy()}

	removeServiceAccount(delegatePod)
	// TODO? add compatible fields instead of removing incompatible ones
	// (for forward compatibility and we've certainly forgotten incompatible fields...)
	// TODO... maybe make this configurable, sort of like Federation v2 Overrides

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
