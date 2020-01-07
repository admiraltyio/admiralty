/*
Copyright 2018 The Multicluster-Scheduler Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxypod // import "admiralty.io/multicluster-scheduler/pkg/webhooks/proxypod"

import (
	"context"
	"net/http"

	"admiralty.io/multicluster-scheduler/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"
)

type Handler struct {
	decoder *admission.Decoder
	client  client.Client
	mutator mutator
}

func (h *Handler) Handle(_ context.Context, req admission.Request) admission.Response {
	srcPod := &corev1.Pod{}
	err := h.decoder.Decode(req, srcPod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	proxyPod := srcPod.DeepCopy()
	if err := h.mutator.mutate(proxyPod); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	proxyPodRaw, err := json.Marshal(proxyPod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, proxyPodRaw)
}

type mutator struct {
}

func (m mutator) mutate(pod *corev1.Pod) error {
	if _, ok := pod.Annotations[common.AnnotationKeyElect]; !ok {
		// not a multicluster pod
		return nil
	}

	// only save the source manifest if it's not set already
	// webhooks may be run multiple times on the same object
	// and have to be idempotent
	// if we didn't check, we could lose the source scheduling constraints that we remove below
	if _, ok := pod.Annotations[common.AnnotationKeySourcePodManifest]; !ok {
		srcPodManifest, err := yaml.Marshal(pod)
		if err != nil {
			return err
		}

		// pod.Annotations is not nil because we checked it contains AnnotationKeyElect
		pod.Annotations[common.AnnotationKeySourcePodManifest] = string(srcPodManifest)
	}

	pod.Spec.Tolerations = []corev1.Toleration{{
		Key:   "virtual-kubelet.io/provider",
		Value: "admiralty",
	}}

	pod.Spec.NodeName = "admiralty"

	// remove other scheduling constraints (will be respected in target cluster, from source pod manifest)
	pod.Spec.SchedulerName = ""
	pod.Spec.NodeSelector = nil
	pod.Spec.Affinity = nil
	pod.Spec.TopologySpreadConstraints = nil

	return nil
}

// Handler implements inject.Client.
// A client will be automatically injected.
var _ inject.Client = &Handler{}

// InjectClient injects the client.
func (h *Handler) InjectClient(c client.Client) error {
	h.client = c
	return nil
}

// Handler implements inject.Decoder.
// A decoder will be automatically injected.
var _ admission.DecoderInjector = &Handler{}

// InjectDecoder injects the decoder.
func (h *Handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}
