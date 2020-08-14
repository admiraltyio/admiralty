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
	_, isProxy := pod.Annotations[common.AnnotationKeyElect]
	return isProxy
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
