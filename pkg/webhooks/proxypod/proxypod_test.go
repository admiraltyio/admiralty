package proxypod

import (
	"testing"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"github.com/ghodss/yaml"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO test webhook namespace selector

var testCases = map[string]struct {
	pod        corev1.Pod
	mutatedPod corev1.Pod
}{
	"proxy pod": {
		corev1.Pod{
			ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{common.AnnotationKeyElect: ""}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  "nginx",
				Image: "nginx",
			}}}},
		corev1.Pod{
			ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{
				common.AnnotationKeyElect:             "",
				common.AnnotationKeySourcePodManifest: "HACK", // yaml serialization computed in test code
			}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:    "nginx",
				Image:   image,
				Command: command,
			}}}},
	},
	"other pod": {
		corev1.Pod{
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  "nginx",
				Image: "nginx",
			}}}},
		corev1.Pod{
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  "nginx",
				Image: "nginx",
			}}}},
	},
	"federation name": {
		corev1.Pod{
			ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{
				common.AnnotationKeyElect:          "",
				common.AnnotationKeyFederationName: "f1",
			}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  "nginx",
				Image: "nginx",
			}}}},
		corev1.Pod{
			ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{
				common.AnnotationKeyElect:             "",
				common.AnnotationKeyFederationName:    "f1",
				common.AnnotationKeySourcePodManifest: "HACK", // yaml serialization computed in test code
			}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:    "nginx",
				Image:   image,
				Command: command,
			}}}},
	},
	"keep labels and annotations (in general, object meta)": {
		corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{common.AnnotationKeyElect: "", "k1": "v1"},
				Labels:      map[string]string{"k2": "v2"},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  "nginx",
				Image: "nginx",
			}}}},
		corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					common.AnnotationKeyElect:             "",
					common.AnnotationKeySourcePodManifest: "HACK", // yaml serialization computed in test code
					"k1":                                  "v1",
				},
				Labels: map[string]string{"k2": "v2"},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:    "nginx",
				Image:   image,
				Command: command,
			}}}},
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
