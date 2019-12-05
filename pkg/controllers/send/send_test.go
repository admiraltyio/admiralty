package send

import (
	"testing"

	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeObservation(t *testing.T) {
	testCases := map[string]struct {
		live metav1.Object
		obs  metav1.Object
	}{
		"normal": {
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod2",
					Namespace:   "ns2",
					Labels:      map[string]string{"k3": "v3"},
					Annotations: map[string]string{"k4": "v4"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			},
			&v1alpha1.PodObservation{
				Status: v1alpha1.PodObservationStatus{
					LiveState: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "pod2",
							Namespace:   "ns2",
							Labels:      map[string]string{"k3": "v3"},
							Annotations: map[string]string{"k4": "v4"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			},
		},
	}

	for k, v := range testCases {
		a := applier{}
		expectedObs := &v1alpha1.PodObservation{}
		if err := a.MakeChild(v.live, expectedObs); err != nil {
			t.Errorf("%s failed: %v", k, err)
		}
		diff := deep.Equal(v.obs, expectedObs)
		if len(diff) > 0 {
			t.Errorf("%s failed with observation diff: %v", k, diff)
		}
	}
}

func TestObservationNeedsUpdate(t *testing.T) {
	testCases := map[string]struct {
		live       metav1.Object
		obs        metav1.Object
		needUpdate bool
	}{
		"no change": {
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod2",
					Namespace:   "ns2",
					Labels:      map[string]string{"k3": "v3"},
					Annotations: map[string]string{"k4": "v4"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			},
			&v1alpha1.PodObservation{
				Status: v1alpha1.PodObservationStatus{
					LiveState: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "pod2",
							Namespace:   "ns2",
							Labels:      map[string]string{"k3": "v3"},
							Annotations: map[string]string{"k4": "v4"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			},
			false,
		},
		"new annotation": {
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod2",
					Namespace:   "ns2",
					Labels:      map[string]string{"k3": "v3"},
					Annotations: map[string]string{"k4": "v4", "k5": "v5"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			},
			&v1alpha1.PodObservation{
				Status: v1alpha1.PodObservationStatus{
					LiveState: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "pod2",
							Namespace:   "ns2",
							Labels:      map[string]string{"k3": "v3"},
							Annotations: map[string]string{"k4": "v4"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			},
			true,
		},
		"completed": {
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod2",
					Namespace:   "ns2",
					Labels:      map[string]string{"k3": "v3"},
					Annotations: map[string]string{"k4": "v4"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: "Succeeded",
				},
			},
			&v1alpha1.PodObservation{
				Status: v1alpha1.PodObservationStatus{
					LiveState: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "pod2",
							Namespace:   "ns2",
							Labels:      map[string]string{"k3": "v3"},
							Annotations: map[string]string{"k4": "v4"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			},
			true,
		},
	}

	for k, v := range testCases {
		a := applier{}
		needUpdate, err := a.ChildNeedsUpdate(v.live, v.obs, &v1alpha1.PodObservation{})
		if err != nil {
			t.Errorf("%s failed: %v", k, err)
		}
		if needUpdate != v.needUpdate {
			t.Errorf("%s failed", k)
		}
	}
}
