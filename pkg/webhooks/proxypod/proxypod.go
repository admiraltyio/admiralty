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
	"fmt"
	"net/http"
	"os"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"github.com/ghodss/yaml"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/builder"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

var webhookName = "proxypod.multicluster.admiralty.io"

func NewServer(mgr manager.Manager, namespace string) (*webhook.Server, error) {
	w, err := NewWebhook(mgr)
	if err != nil {
		return nil, fmt.Errorf("cannot build webhook: %v", err)
	}

	deployName := os.Getenv("DEPLOYMENT_NAME")

	s, err := webhook.NewServer(deployName, mgr, webhook.ServerOptions{
		Port:    9876, // TODO debug why cannot default to 443
		CertDir: "/tmp/cert",
		BootstrapOptions: &webhook.BootstrapOptions{
			Secret: &types.NamespacedName{
				Namespace: namespace,
				Name:      deployName + "-cert",
			},
			Service: &webhook.Service{
				Namespace: namespace,
				Name:      deployName,
				// Selectors should select the pods that runs this webhook server.
				Selectors: map[string]string{
					"app.kubernetes.io/name": deployName,
					// HACK: there should be more here (cf. selectorLabels in helm chart)
					// we could get the labels using the downward API, just like we get DEPLOYMENT_NAME
					// but this won't be necessary once we stop using BootstrapOptions
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create server: %v", err)
	}

	if err := s.Register(w); err != nil {
		return nil, fmt.Errorf("cannot register webhook with server: %v", err)
	}

	return s, nil
}

// https://kubernetes.slack.com/archives/CAR30FCJZ/p1547254570666900
func NewWebhook(mgr manager.Manager) (*admission.Webhook, error) {
	return builder.NewWebhookBuilder().
		Name(webhookName).
		Mutating().
		Operations(admissionregistrationv1beta1.Create). // TODO: update (but careful not to proxy the proxy)
		WithManager(mgr).
		ForType(&corev1.Pod{}).
		Handlers(&Handler{mutator: mutator{}}).
		FailurePolicy(admissionregistrationv1beta1.Fail).
		NamespaceSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"multicluster-scheduler": "enabled"}}).
		Build()
}

type Handler struct {
	decoder atypes.Decoder
	client  client.Client
	mutator mutator
}

func (h *Handler) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	srcPod := &corev1.Pod{}
	err := h.decoder.Decode(req, srcPod)
	if err != nil {
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}

	proxyPod := srcPod.DeepCopy()
	if err := h.mutator.mutate(proxyPod); err != nil {
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}
	return admission.PatchResponse(srcPod, proxyPod)
}

type mutator struct {
}

func (m mutator) mutate(pod *corev1.Pod) error {
	if _, ok := pod.Annotations[common.AnnotationKeyElect]; !ok {
		// not a multicluster pod
		return nil
	}

	srcPodManifest, err := yaml.Marshal(pod)
	if err != nil {
		return err
	}

	// pod.Annotations is not nil because we checked it contains AnnotationKeyElect
	pod.Annotations[common.AnnotationKeySourcePodManifest] = string(srcPodManifest)

	for i, c := range pod.Spec.Containers {
		pod.Spec.Containers[i] = corev1.Container{
			Name:    c.Name,
			Image:   image,
			Command: command}
		// the feedback controller will send SIGUSR1 or SIGUSR2 when the delegate pod succeeds or fails, resp.
	}
	// TODO: add resource reqs/lims + other best practices
	// TODO!!! remove scheduling prefs

	return nil
}

var image = "busybox"
var command = []string{"sh", "-c", "trap 'exit 0' SIGUSR1; trap 'exit 1' SIGUSR2; (while sleep 3600; do :; done) & wait"}

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
