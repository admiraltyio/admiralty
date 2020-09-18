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

package proxypod

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"admiralty.io/multicluster-scheduler/pkg/common"
)

func IsProxy(pod *corev1.Pod) bool {
	// The scheduler name is the best indication that a pod is a proxy pod.
	// The elect annotation can be added in namespaces where admiralty isn't enabled;
	// pod would skip mutating admission webhook and be scheduled normally (not a proxy pod),
	// which would cause issues for controllers looking up target client from node name.
	// The node name is only an indicator after scheduling, and feedback needs to be able to remove finalizer even if pod wasn't scheduled;
	// Also, service reroute can start working earlier this way.
	return pod.Spec.SchedulerName == common.ProxySchedulerName
}

func GetSourcePod(proxyPod *corev1.Pod) (*corev1.Pod, error) {
	srcPodManifest, ok := proxyPod.Annotations[common.AnnotationKeySourcePodManifest]
	if !ok {
		return nil, fmt.Errorf("no source pod manifest on proxy pod")
	}
	srcPod := &corev1.Pod{}
	if err := yaml.Unmarshal([]byte(srcPodManifest), srcPod); err != nil {
		return nil, fmt.Errorf("cannot unmarshal source pod manifest: %v", err)
	}
	return srcPod, nil
}

func IsScheduled(proxyPod *corev1.Pod) bool {
	return proxyPod.Spec.NodeName != ""
}

func GetScheduledClusterName(proxyPod *corev1.Pod) string {
	if !IsScheduled(proxyPod) {
		return ""
	}
	return proxyPod.Spec.NodeName
}
