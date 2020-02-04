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

	"admiralty.io/multicluster-scheduler/pkg/common"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

func IsProxy(pod *corev1.Pod) bool {
	return pod.Spec.SchedulerName == "admiralty"
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

func GetTargetClusterName(proxyPod *corev1.Pod) string {
	return proxyPod.Annotations[common.AnnotationKeyClusterName]
}

func LocalBind(proxyPod *corev1.Pod, targetClusterName string, client kubernetes.Interface) error {
	proxyPod.Annotations[common.AnnotationKeyClusterName] = targetClusterName
	_, err := client.CoreV1().Pods(proxyPod.Namespace).Update(proxyPod)
	if err != nil {
		return err
	}

	b := &corev1.Binding{
		ObjectMeta: v1.ObjectMeta{Name: proxyPod.Name, UID: proxyPod.UID},
		Target:     corev1.ObjectReference{Kind: "Node", Name: "admiralty"},
	}
	if err := client.CoreV1().Pods(proxyPod.Namespace).Bind(b); err != nil {
		return err
	}

	return nil
}

func IsScheduled(proxyPod *corev1.Pod) bool {
	return proxyPod.Spec.NodeName != ""
}

func GetScheduledClusterName(proxyPod *corev1.Pod) string {
	return proxyPod.Annotations[common.AnnotationKeyClusterName]
}
