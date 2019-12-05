package schedule

import (
	"fmt"
	"testing"

	configv1alpha1 "admiralty.io/multicluster-scheduler/pkg/apis/config/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/config/scheduler"
	"github.com/ghodss/yaml"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var schedulerNamespace = "multicluster-scheduler"

var testCases = map[string]struct {
	applier               applier
	podObsBefore          *v1alpha1.PodObservation
	srcPod                *corev1.Pod              // serialization added as annotation before test
	podObsAfter           *v1alpha1.PodObservation // nil if no update
	pendingDecisionsAfter pendingDecisions
}{
	"normal": {
		applier{
			schedCfg: scheduler.New(&configv1alpha1.Scheduler{
				UseClusterNamespaces: true,
				Clusters: []configv1alpha1.Cluster{
					{Name: "c1"},
					{Name: "c2"},
				},
			}, schedulerNamespace),
			scheduler:        testScheduler{"c2", nil},
			pendingDecisions: pendingDecisions{},
		},
		&v1alpha1.PodObservation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "c1",
			},
			Status: v1alpha1.PodObservationStatus{
				LiveState: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "pod1",
						Namespace:   "ns1",
						Annotations: map[string]string{common.AnnotationKeyElect: ""},
					},
				},
			},
		},
		&corev1.Pod{},
		&v1alpha1.PodObservation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "c1",
			},
			Status: v1alpha1.PodObservationStatus{
				LiveState: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns1",
						Annotations: map[string]string{
							common.AnnotationKeyElect:       "",
							common.AnnotationKeyClusterName: "c2",
						},
					},
				},
			},
		},
		pendingDecisions{
			key{"c2", "ns1", "pod1"}: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{ClusterName: "c2"},
			},
		},
	},
	"remove from pending decisions": {
		applier{
			schedCfg: scheduler.New(&configv1alpha1.Scheduler{
				UseClusterNamespaces: true,
				Clusters: []configv1alpha1.Cluster{
					{Name: "c1"},
					{Name: "c2"},
				},
			}, schedulerNamespace),
			pendingDecisions: pendingDecisions{
				key{"c2", "ns1", "pod1"}: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{ClusterName: "c2"},
				},
			},
		},
		&v1alpha1.PodObservation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "c2",
			},
			Status: v1alpha1.PodObservationStatus{
				LiveState: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns1",
					},
				},
			},
		},
		nil,
		nil,
		pendingDecisions{},
	},
	"already scheduled": {
		applier{
			schedCfg: scheduler.New(&configv1alpha1.Scheduler{
				UseClusterNamespaces: true,
				Clusters: []configv1alpha1.Cluster{
					{Name: "c1"},
					{Name: "c2"},
				},
			}, schedulerNamespace),
			pendingDecisions: pendingDecisions{},
		},
		&v1alpha1.PodObservation{
			Status: v1alpha1.PodObservationStatus{
				LiveState: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							common.AnnotationKeyElect:          "",
							common.AnnotationKeyFederationName: "default",
							common.AnnotationKeyClusterName:    "c3",
						},
					},
				},
			},
		},
		nil,
		nil,
		pendingDecisions{},
	},
	"return to original cluster on scheduling error": {
		applier{
			schedCfg: scheduler.New(&configv1alpha1.Scheduler{
				UseClusterNamespaces: true,
				Clusters: []configv1alpha1.Cluster{
					{Name: "c1"},
					{Name: "c2"},
				},
			}, schedulerNamespace),
			scheduler:        testScheduler{"", fmt.Errorf("some error")},
			pendingDecisions: pendingDecisions{},
		},
		&v1alpha1.PodObservation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "c1",
			},
			Status: v1alpha1.PodObservationStatus{
				LiveState: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns1",
						Annotations: map[string]string{
							common.AnnotationKeyElect: "",
						},
					},
				},
			},
		},
		&corev1.Pod{},
		&v1alpha1.PodObservation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "c1",
			},
			Status: v1alpha1.PodObservationStatus{
				LiveState: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns1",
						Annotations: map[string]string{
							common.AnnotationKeyElect:       "",
							common.AnnotationKeyClusterName: "c1",
						},
					},
				},
			},
		},
		pendingDecisions{
			key{"c1", "ns1", "pod1"}: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{ClusterName: "c1"},
			},
		},
	},
}

type testScheduler struct {
	clusterName string
	err         error
}

var _ SchedulerShim = testScheduler{}

func (s testScheduler) Schedule(*corev1.Pod) (string, error) {
	return s.clusterName, s.err
}

func TestSchedule(t *testing.T) {
	for k, v := range testCases {
		if v.podObsAfter != nil {
			srcPodManifest, err := yaml.Marshal(v.srcPod)
			if err != nil {
				t.Errorf("%s failed: cannot serialize source pod: %v", k, err)
			}
			v.podObsBefore.Status.LiveState.Annotations[common.AnnotationKeySourcePodManifest] = string(srcPodManifest)
			v.podObsAfter.Status.LiveState.Annotations[common.AnnotationKeySourcePodManifest] = string(srcPodManifest)
		}
		needUpdate, err := v.applier.NeedUpdate(v.podObsBefore)
		if err != nil {
			t.Errorf("%s failed: %v", k, err)
		}
		if needUpdate != (v.podObsAfter != nil) {
			var shouldOrShouldNot string
			if needUpdate {
				shouldOrShouldNot = "should"
			} else {
				shouldOrShouldNot = "should not"
			}
			t.Errorf("%s failed: %s need update", k, shouldOrShouldNot)
		}
		if needUpdate {
			err := v.applier.Mutate(v.podObsBefore)
			if err != nil {
				t.Errorf("%s failed: %v", k, err)
			}
			diff := deep.Equal(v.podObsBefore, v.podObsAfter)
			if len(diff) > 0 {
				t.Errorf("%s failed with pod obs diff: %v", k, diff)
			}
		}
		diff := deep.Equal(v.pendingDecisionsAfter, v.applier.pendingDecisions)
		if len(diff) > 0 {
			t.Errorf("%s failed with pending decisions diff: %v", k, diff)
		}
	}
}
