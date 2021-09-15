/*
 * Copyright 2021 The Multicluster-Scheduler Authors.
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
	"testing"

	"github.com/ghodss/yaml"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"admiralty.io/multicluster-scheduler/pkg/common"
)

// TODO test webhook namespace selector

var zero int64 = 0

var testCases = map[string]struct {
	pod        corev1.Pod
	mutatedPod corev1.Pod
}{
	"proxy pod": {
		corev1.Pod{
			ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{common.AnnotationKeyElect: ""}},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "nginx",
					Image: "nginx",
				}},
			},
		},
		corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					common.AnnotationKeyElect:             "",
					common.AnnotationKeySourcePodManifest: "HACK", // yaml serialization computed in test code
				},
				Labels: map[string]string{
					common.LabelKeyHasFinalizer: "true",
				},
			},
			Spec: corev1.PodSpec{
				NodeSelector: map[string]string{
					common.LabelAndTaintKeyVirtualKubeletProvider: common.VirtualKubeletProviderName,
				},
				Containers: []corev1.Container{{
					Name:  "nginx",
					Image: "nginx",
				}},
				Tolerations: []corev1.Toleration{{
					Key:   common.LabelAndTaintKeyVirtualKubeletProvider,
					Value: common.VirtualKubeletProviderName,
				}, {
					Key:      corev1.TaintNodeNetworkUnavailable,
					Operator: corev1.TolerationOpExists,
				}},
				SchedulerName:                 common.ProxySchedulerName,
				TerminationGracePeriodSeconds: &zero,
			},
		},
	},
	"other pod": {
		corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "nginx",
					Image: "nginx",
				}},
			},
		},
		corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "nginx",
					Image: "nginx",
				}},
			},
		},
	},
	"keep labels and annotations (in general, object meta)": {
		corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{common.AnnotationKeyElect: "", "k1": "v1"},
				Labels:      map[string]string{"k2": "v2"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "nginx",
					Image: "nginx",
				}},
			},
		},
		corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					common.AnnotationKeyElect:             "",
					common.AnnotationKeySourcePodManifest: "HACK", // yaml serialization computed in test code
					"k1":                                  "v1",
				},
				Labels: map[string]string{"k2": "v2", common.LabelKeyHasFinalizer: "true"},
			},
			Spec: corev1.PodSpec{
				NodeSelector: map[string]string{
					common.LabelAndTaintKeyVirtualKubeletProvider: common.VirtualKubeletProviderName,
				},
				Containers: []corev1.Container{{
					Name:  "nginx",
					Image: "nginx",
				}},
				Tolerations: []corev1.Toleration{{
					Key:   common.LabelAndTaintKeyVirtualKubeletProvider,
					Value: common.VirtualKubeletProviderName,
				}, {
					Key:      corev1.TaintNodeNetworkUnavailable,
					Operator: corev1.TolerationOpExists,
				}},
				SchedulerName:                 common.ProxySchedulerName,
				TerminationGracePeriodSeconds: &zero,
			},
		},
	},
}

func TestMutate(t *testing.T) {
	for k, v := range testCases {
		podManifest, err := yaml.Marshal(v.pod)
		if err != nil {
			t.Errorf("%s failed: %v", k, err)
		}
		if k != "other pod" {
			v.mutatedPod.Annotations[common.AnnotationKeySourcePodManifest] = string(podManifest)
		}
		m := mutator{}
		mutatedPod := v.pod.DeepCopy()
		if err := m.mutate(mutatedPod); err != nil {
			t.Errorf("%s failed: %v", k, err)
		}
		diff := deep.Equal(mutatedPod, &v.mutatedPod)
		if len(diff) > 0 {
			t.Errorf("%s failed with mutated pod diff: %v", k, diff)
		}
	}
}
