/*
Copyright 2018 The Multicluster-Service-Account Authors.

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

package automount // import "admiralty.io/multicluster-service-account/pkg/automount"

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"admiralty.io/multicluster-service-account/pkg/apis/multicluster/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

var saiName string = "multicluster.admiralty.io/service-account-import.name"

// Handler handles pod admission requests, mutating pods that request service account imports.
// It is implemented by the service-account-import-admission-controller command, via controller-runtime.
// If a pod is annotated with the "multicluster.admiralty.io/service-account-import.name" key,
// where the value is a comma-separated list of service account import names, for each
// service account import, a volume is added to the pod, sourced from the first secret
// listed by the service account import, and mounted in each of the pod's containers under
// /var/run/secrets/admiralty.io/serviceaccountimports/%s, where %s is the service account import name.
type Handler struct {
	decoder atypes.Decoder
	client  client.Client
}

func (h *Handler) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	pod := &corev1.Pod{}

	err := h.decoder.Decode(req, pod)
	if err != nil {
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}
	log.Printf("Mutating pod %s in namespace %s", req.AdmissionRequest.Name, req.AdmissionRequest.Namespace)
	copy := pod.DeepCopy()

	err = h.mutatePodsFn(ctx, req, copy)
	if err != nil {
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}
	return admission.PatchResponse(pod, copy)
}

func (h *Handler) mutatePodsFn(ctx context.Context, req atypes.Request, pod *corev1.Pod) error {
	saiNamesStr, ok := pod.Annotations[saiName]
	if !ok {
		log.Printf("no service account import name annotation")
		return nil
	}
	log.Printf("service account import names: %s", saiNamesStr)

	saiNames := strings.Split(saiNamesStr, ",")
	for _, saiName := range saiNames {
		sai := &v1alpha1.ServiceAccountImport{}
		if err := h.client.Get(ctx, types.NamespacedName{Namespace: req.AdmissionRequest.Namespace, Name: saiName}, sai); err != nil {
			// TODO: validating admission webhook shouldn't let this happen
			log.Printf("cannot find service account import %s:%s", req.AdmissionRequest.Namespace, saiName)
			continue
		}

		if len(sai.Status.Secrets) == 0 {
			// TODO: validating admission webhook shouldn't let this happen
			log.Printf("service account import %s has no token", saiName)
			continue
		}

		secretName := sai.Status.Secrets[0].Name

		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name:         secretName,
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}},
		})

		for i := range pod.Spec.Containers {
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{Name: secretName, ReadOnly: true,
				MountPath: fmt.Sprintf("/var/run/secrets/admiralty.io/serviceaccountimports/%s", saiName)})
		}
	}

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
var _ inject.Decoder = &Handler{}

// InjectDecoder injects the decoder.
func (h *Handler) InjectDecoder(d atypes.Decoder) error {
	h.decoder = d
	return nil
}
