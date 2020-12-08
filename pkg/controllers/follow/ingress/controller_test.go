package ingress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/networking/v1beta1"
)

func TestShouldNotUpdateOnEqual(t *testing.T) {
	r := &ingressReconciler{}
	ingress := v1beta1.Ingress{}
	remoteIngress := v1beta1.Ingress{}

	_, shouldUpdate := r.shouldUpdate(&remoteIngress, &ingress)
	if shouldUpdate != false {
		t.Error("Equal ingresses should not be updated")
	}
}

func TestShouldUpdateOnSpecDiff(t *testing.T) {
	r := &ingressReconciler{}
	ingress := v1beta1.Ingress{
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "test"},
		},
	}
	remoteIngress := v1beta1.Ingress{
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "test2"},
		},
	}

	updatedIngress, shouldUpdate := r.shouldUpdate(&remoteIngress, &ingress)
	if shouldUpdate != true {
		t.Error("Ingresses with different spec should trigger update")
	}

	assert.Equal(t, updatedIngress.Spec, ingress.Spec)
}

func TestShouldUpdateOnAnnotationDiff(t *testing.T) {
	r := &ingressReconciler{}
	ingress := v1beta1.Ingress{
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "test"},
		},
	}
	ingress.Annotations = make(map[string]string)
	ingress.Annotations = map[string]string{"annotate": "me"}
	remoteIngress := v1beta1.Ingress{
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "test"},
		},
	}
	remoteIngress.Annotations = make(map[string]string)

	updatedIngress, shouldUpdate := r.shouldUpdate(&remoteIngress, &ingress)
	if shouldUpdate != true {
		t.Error("Ingresses with different annotations should trigger update")
	}

	assert.Equal(t, updatedIngress.Annotations, ingress.Annotations)
}

func TestShouldUpdateOnAnnotationDiffIgnoringAdmiraltyAnnotations(t *testing.T) {
	r := &ingressReconciler{}
	ingress := v1beta1.Ingress{
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "test"},
		},
	}
	ingress.Annotations = make(map[string]string)
	ingress.Annotations = map[string]string{
		"multicluster.admiralty.io/global": "true",
		"propagate":                        "me",
	}
	remoteIngress := v1beta1.Ingress{
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "test"},
		},
	}
	remoteIngress.Annotations = make(map[string]string)
	remoteIngress.Annotations = map[string]string{
		"multicluster.admiralty.io/global":           "true",
		"multicluster.admiralty.io/is-delegate":      "",
		"multicluster.admiralty.io/parent-name":      "podinfo",
		"multicluster.admiralty.io/parent-namespace": "default",
		"multicluster.admiralty.io/parent-uid":       "788fff97-76ee-4a55-89fc-33c706f21716",
	}

	updatedIngress, shouldUpdate := r.shouldUpdate(&remoteIngress, &ingress)
	if shouldUpdate != true {
		t.Error("Ingresses with different non Admiralty annotations should trigger update")
	}

	assert.Equal(t, updatedIngress.Annotations["propagate"], "me")
}

func TestShouldNotUpdateOnAnnotationDiffWithAdmiraltyAnnotationsOnly(t *testing.T) {
	r := &ingressReconciler{}
	ingress := v1beta1.Ingress{
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "test"},
		},
	}
	ingress.Annotations = make(map[string]string)
	ingress.Annotations = map[string]string{
		"multicluster.admiralty.io/global": "true",
	}
	remoteIngress := v1beta1.Ingress{
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "test"},
		},
	}
	remoteIngress.Annotations = make(map[string]string)
	remoteIngress.Annotations = map[string]string{
		"multicluster.admiralty.io/global":           "true",
		"multicluster.admiralty.io/is-delegate":      "",
		"multicluster.admiralty.io/parent-name":      "podinfo",
		"multicluster.admiralty.io/parent-namespace": "default",
		"multicluster.admiralty.io/parent-uid":       "788fff97-76ee-4a55-89fc-33c706f21716",
	}

	_, shouldUpdate := r.shouldUpdate(&remoteIngress, &ingress)
	if shouldUpdate != false {
		t.Error("Ingresses with difference of only Admiralty annotations should NOT trigger update")
	}
}
