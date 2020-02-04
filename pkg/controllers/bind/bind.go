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

package bind

import (
	"reflect"
	"strings"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewController(clusters []*cluster.Cluster) (*controller.Controller, error) {
	return gc.NewController(clusters, clusters, gc.Options{
		ParentPrototype: &corev1.Pod{},
		ChildPrototype:  &corev1.Pod{},
		ParentWatchOptions: controller.WatchOptions{CustomPredicate: func(obj interface{}) bool {
			pod := obj.(*corev1.Pod)
			return proxypod.IsProxy(pod) && proxypod.IsScheduled(pod)
		}},
		Applier: applier{},
		GetImpersonatorForChildWriter: func(clusterName string) string {
			return "admiralty:" + clusterName
		},
	})
}

type applier struct{}

var _ gc.Applier = applier{}

func (a applier) ChildClusterName(parent interface{}) (string, error) {
	proxyPod := parent.(*corev1.Pod)
	clusterName := proxypod.GetScheduledClusterName(proxyPod)
	return clusterName, nil
}

func (a applier) MutateParent(parent interface{}, childFound bool, child interface{}) (needUpdate bool, needStatusUpdate bool, err error) {
	if !childFound {
		return false, false, nil
	}

	proxyPod := parent.(*corev1.Pod)
	delegatePod := child.(*corev1.Pod)

	mcProxyPodAnnotations, otherProxyPodAnnotations := filterAnnotations(proxyPod.Annotations)
	_, otherDelegatePodAnnotations := filterAnnotations(delegatePod.Annotations)

	needUpdate = !reflect.DeepEqual(otherProxyPodAnnotations, otherDelegatePodAnnotations)
	if needUpdate {
		for k, v := range otherDelegatePodAnnotations {
			mcProxyPodAnnotations[k] = v
		}
		proxyPod.Annotations = mcProxyPodAnnotations
	}

	// we can't group annotation and status updates into an update,
	// because general update ignores status

	needStatusUpdate = deep.Equal(proxyPod.Status, delegatePod.Status) != nil
	if needStatusUpdate {
		proxyPod.Status = delegatePod.Status
	}

	return needUpdate, needStatusUpdate, nil
}

func (a applier) MakeChild(parent interface{}, expectedChild interface{}) error {
	proxyPod := parent.(*corev1.Pod)
	delegatePod := expectedChild.(*corev1.Pod)

	p, err := a.makeDelegatePod(proxyPod)
	if err != nil {
		return err
	}

	p.DeepCopyInto(delegatePod)
	return nil
}

func (a applier) MutateChild(_, _, _ interface{}) (needUpdate bool, err error) {
	return false, nil
}

func (a applier) makeDelegatePod(proxyPod *corev1.Pod) (*corev1.Pod, error) {
	srcPod, err := proxypod.GetSourcePod(proxyPod)
	if err != nil {
		return nil, err
	}

	annotations := make(map[string]string)
	for k, v := range srcPod.Annotations {
		if !strings.HasPrefix(k, common.KeyPrefix) {
			// we don't want to mc-schedule the delegate pod with elect,
			// and the target cluster name and source pod manifest are now redundant
			// we only keep the user annotations
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

func filterAnnotations(annotations map[string]string) (map[string]string, map[string]string) {
	mcAnnotations := make(map[string]string)
	otherAnnotations := make(map[string]string)
	for k, v := range annotations {
		if strings.HasPrefix(k, common.KeyPrefix) {
			mcAnnotations[k] = v
		} else {
			otherAnnotations[k] = v
		}
	}
	return mcAnnotations, otherAnnotations
}
