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
	"github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

type Handler struct {
	decoder atypes.Decoder
	client  client.Client
}

func (h *Handler) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	srcPod := &corev1.Pod{}
	err := h.decoder.Decode(req, srcPod)
	if err != nil {
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}

	if _, ok := srcPod.Annotations[common.AnnotationKeyElect]; !ok {
		// not a multicluster pod
		return admission.PatchResponse(srcPod, srcPod)
	}

	srcPodManifest, err := yaml.Marshal(srcPod)
	if err != nil {
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}

	proxyPod := srcPod.DeepCopy()
	// proxyPod.Annotations is not nil because we checked it contains AnnotationKeyElect
	proxyPod.Annotations[common.AnnotationKeySourcePodManifest] = string(srcPodManifest)

	for i, c := range proxyPod.Spec.Containers { // same number of containers because of jsonpatch bug
		proxyPod.Spec.Containers[i] = corev1.Container{
			Name:    c.Name,
			Image:   "busybox",
			Command: []string{"sh", "-c", "trap 'exit 0' SIGUSR1; trap 'exit 1' SIGUSR2; (while sleep 3600; do :; done) & wait"}}
		// the feedback controller will send SIGUSR1 or SIGUSR2 when the delegate pod succeeds or fails, resp.
	}
	// TODO: add resource reqs/lims + other best practices

	return admission.PatchResponse(srcPod, proxyPod)
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
var _ inject.Decoder = &Handler{}

// InjectDecoder injects the decoder.
func (h *Handler) InjectDecoder(d atypes.Decoder) error {
	h.decoder = d
	return nil
}
