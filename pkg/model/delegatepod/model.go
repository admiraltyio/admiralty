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

package delegatepod

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
)

func IsDelegate(pod *corev1.Pod) bool {
	return pod.Spec.SchedulerName == common.CandidateSchedulerName
}

func MakeDelegatePod(proxyPod *corev1.Pod) (*v1alpha1.PodChaperon, error) {
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

	labels, _ := ChangeLabels(srcPod.Labels)
	// used to get pod chaperon given proxy pod ("list one" hack)
	labels[common.LabelKeyParentUID] = string(proxyPod.UID)

	delegatePod := &v1alpha1.PodChaperon{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    proxyPod.Namespace, // already defaults to "default" (vs. could be empty in srcPod)
			GenerateName: proxyPod.Name + "-",
			Labels:       labels,
			Annotations:  annotations},
		Spec: *srcPod.Spec.DeepCopy()}

	controller.AddRemoteControllerReference(delegatePod, proxyPod)

	if _, ok := srcPod.Annotations[common.AnnotationKeyUseConstraintsFromSpecForProxyPodScheduling]; ok {
		delegatePod.Spec.NodeSelector = nil
		delegatePod.Spec.Tolerations = nil
		delegatePod.Spec.Affinity = nil
		delegatePod.Spec.TopologySpreadConstraints = nil
	}

	removeServiceAccount(&delegatePod.Spec)
	// TODO? add compatible fields instead of removing incompatible ones
	// (for forward compatibility and we've certainly forgotten incompatible fields...)
	// TODO... maybe make this configurable, sort of like Federation v2 Overrides

	if _, ok := srcPod.Annotations[common.AnnotationKeyNoReservation]; !ok {
		delegatePod.Spec.SchedulerName = common.CandidateSchedulerName
	}

	return delegatePod, nil
}

// ChangeLabels changes a delegate pod's labels so as not to confuse potential controller of proxy pod, e.g., replica set.
// If the original label key has a domain prefix, replace it with ours;
// if it doesn't, add our domain prefix.
// Also used to optionally reroute service selector.
// Length is not an issue:
// "Valid label keys have two segments: an optional prefix and name, separated by a slash (/).
// The name segment is required and must be 63 characters or less"
// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
// TODO: resolve conflict two keys have same name but different prefixes
func ChangeLabels(labels map[string]string) (map[string]string, bool) {
	changed := false
	newLabels := make(map[string]string)
	for k, v := range labels {
		keySplit := strings.Split(k, "/") // note: assume no empty key (enforced by Kubernetes)
		if len(keySplit) == 1 || keySplit[0] != common.KeyPrefix {
			changed = true
			newKey := common.KeyPrefix + keySplit[len(keySplit)-1]
			newLabels[newKey] = v
		} else {
			newLabels[k] = v
		}
	}
	return newLabels, changed
}

func removeServiceAccount(podSpec *corev1.PodSpec) {
	var saSecretName string
	for i, c := range podSpec.Containers {
		j := -1
		for i, m := range c.VolumeMounts {
			if m.MountPath == "/var/run/secrets/kubernetes.io/serviceaccount" {
				saSecretName = m.Name // should be the same secret name for all containers
				j = i
				break
			}
		}
		if j > -1 {
			c.VolumeMounts = append(c.VolumeMounts[:j], c.VolumeMounts[j+1:]...)
			podSpec.Containers[i] = c
		}
	}
	for i, c := range podSpec.InitContainers {
		j := -1
		for i, m := range c.VolumeMounts {
			if m.MountPath == "/var/run/secrets/kubernetes.io/serviceaccount" {
				saSecretName = m.Name // should be the same secret name for all containers
				j = i
				break
			}
		}
		if j > -1 {
			c.VolumeMounts = append(c.VolumeMounts[:j], c.VolumeMounts[j+1:]...)
			podSpec.InitContainers[i] = c
		}
	}
	// TODO... what about ephemeral containers
	j := -1
	for i, v := range podSpec.Volumes {
		if v.Name == saSecretName {
			j = i
			break
		}
	}
	if j > -1 {
		podSpec.Volumes = append(podSpec.Volumes[:j], podSpec.Volumes[j+1:]...)
	}
}
