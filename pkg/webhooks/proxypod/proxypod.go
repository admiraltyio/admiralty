/*
 * Copyright 2022 The Multicluster-Scheduler Authors.
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

package proxypod // import "admiralty.io/multicluster-scheduler/pkg/webhooks/proxypod"

import (
	"context"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"admiralty.io/multicluster-scheduler/pkg/common"
)

type Handler struct {
	decoder *admission.Decoder
	client  client.Client
	mutator mutator
}

func NewHandler(knownFinalizers map[string][]string) *Handler {
	return &Handler{mutator: mutator{knownFinalizers: knownFinalizers}}
}

func (h *Handler) Handle(_ context.Context, req admission.Request) admission.Response {
	srcPod := &corev1.Pod{}
	err := h.decoder.Decode(req, srcPod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// namespace isn't necessarily set in the raw object (but defined in resource URL)
	srcPod.Namespace = req.Namespace

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
	knownFinalizers map[string][]string
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
	var srcPod *corev1.Pod
	if _, ok := pod.Annotations[common.AnnotationKeySourcePodManifest]; !ok {
		srcPod = pod.DeepCopy()

		srcPodManifest, err := yaml.Marshal(srcPod)
		if err != nil {
			return err
		}

		// pod.Annotations is not nil because we checked it contains AnnotationKeyElect
		pod.Annotations[common.AnnotationKeySourcePodManifest] = string(srcPodManifest)
	} else {
		if err := yaml.UnmarshalStrict([]byte(pod.Annotations[common.AnnotationKeySourcePodManifest]), srcPod); err != nil {
			return err
		}
	}

	pod.Spec.NodeSelector = map[string]string{common.LabelAndTaintKeyVirtualKubeletProvider: common.VirtualKubeletProviderName}

	pod.Spec.Tolerations = []corev1.Toleration{{
		Key:   common.LabelAndTaintKeyVirtualKubeletProvider,
		Value: common.VirtualKubeletProviderName,
	}, {
		Key:      corev1.TaintNodeNetworkUnavailable,
		Operator: corev1.TolerationOpExists,
	}}
	// In some distributions, route controller adds "network unavailable" condition to our virtual nodes,
	// transformed into a taint by the TaintNodeByCondition feature. We need to tolerate that,
	// because we have no control over it.

	// remove other scheduling constraints (will be respected in target cluster, from source pod manifest)
	pod.Spec.Affinity = nil
	pod.Spec.TopologySpreadConstraints = nil

	proxyPodSched := &corev1.PodSpec{}
	if s, ok := pod.Annotations[common.AnnotationKeyProxyPodSchedulingConstraints]; ok {
		if err := yaml.UnmarshalStrict([]byte(s), proxyPodSched); err != nil {
			return err
		}

		// add user-defined proxy pod scheduling constraints
		for k, v := range proxyPodSched.NodeSelector {
			pod.Spec.NodeSelector[k] = v
		}

		for _, t := range proxyPodSched.Tolerations {
			pod.Spec.Tolerations = append(pod.Spec.Tolerations, t)
		}

		pod.Spec.Affinity = proxyPodSched.Affinity
		pod.Spec.TopologySpreadConstraints = proxyPodSched.TopologySpreadConstraints
	} else if _, ok := pod.Annotations[common.AnnotationKeyUseConstraintsFromSpecForProxyPodScheduling]; ok {
		for k, v := range srcPod.Spec.NodeSelector {
			pod.Spec.NodeSelector[k] = v
		}

		for _, t := range srcPod.Spec.Tolerations {
			pod.Spec.Tolerations = append(pod.Spec.Tolerations, t)
		}

		pod.Spec.Affinity = srcPod.Spec.Affinity
		pod.Spec.TopologySpreadConstraints = srcPod.Spec.TopologySpreadConstraints
	}

	pod.Spec.SchedulerName = common.ProxySchedulerName // we don't allow bypassing the proxy scheduler for now
	// TODO we would need to create delegate pod chaperons for proxy pods bound to virtual nodes outside of proxy scheduler

	// pods are usually deleted with a grace period of 30 seconds
	// and it is the kubelet's responsibility to force delete after that
	// so our responsibility, because we use virtual-kubelet (w/o the pod controller)
	// might as well set it to zero already
	// Note that multicluster-controller's GC pattern keeps the proxy pod alive
	// with a finalizer until its delegate is fully deleted
	var grace int64 = 0
	pod.Spec.TerminationGracePeriodSeconds = &grace

	var finalizers []string
	for _, f := range pod.Finalizers {
		if !strings.HasPrefix(f, common.KeyPrefix) {
			finalizers = append(finalizers, f)
		}
	}
	// don't append finalizers of targets in different namespaces
	// because they're useless, and wouldn't be removed because feedback controllers are namespaced
	for _, f := range m.knownFinalizers[pod.Namespace] {
		finalizers = append(finalizers, f)
	}
	pod.Finalizers = finalizers

	// add label for post-delete hook to remove finalizers
	if pod.Labels == nil {
		pod.Labels = make(map[string]string, 1)
	}
	pod.Labels[common.LabelKeyHasFinalizer] = "true"

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
