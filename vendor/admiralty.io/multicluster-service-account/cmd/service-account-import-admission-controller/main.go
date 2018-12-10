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

package main

import (
	"log"
	"os"

	"admiralty.io/multicluster-service-account/pkg/apis"
	"admiralty.io/multicluster-service-account/pkg/automount"
	"admiralty.io/multicluster-service-account/pkg/config"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/sample-controller/pkg/signals"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/builder"
)

var webhookName = "automount.rbac.multicluster.admiralty.io"

func main() {
	cfg, ns, err := config.ConfigAndNamespace()
	if err != nil {
		log.Fatalf("cannot get config and namespace: %v", err)
	}

	m, err := manager.New(cfg, manager.Options{})
	if err != nil {
		log.Fatalf("cannot create manager: %v", err)
	}

	if err := apis.AddToScheme(m.GetScheme()); err != nil {
		log.Fatalf("cannot add APIs to scheme: %v", err)
	}

	w, err := builder.NewWebhookBuilder().
		Name(webhookName).
		Mutating().
		Operations(admissionregistrationv1beta1.Create).
		WithManager(m).
		ForType(&corev1.Pod{}).
		Handlers(&automount.Handler{}).
		Build()
	if err != nil {
		log.Fatalf("cannot build webhook: %v", err)
	}

	deployName := os.Getenv("DEPLOYMENT_NAME")

	s, err := webhook.NewServer(deployName, m, webhook.ServerOptions{
		Port:                 9876, // TODO debug why cannot default to 443
		CertDir:              "/tmp/cert",
		InstallWebhookConfig: true,
		BootstrapOptions: &webhook.BootstrapOptions{
			Secret: &types.NamespacedName{
				Namespace: ns,
				Name:      deployName + "-cert",
			},
			Service: &webhook.Service{
				Namespace: ns,
				Name:      deployName,
				// Selectors should select the pods that runs this webhook server.
				Selectors: map[string]string{
					"app": deployName,
				},
			},
		},
	})
	if err != nil {
		log.Fatalf("cannot create server: %v", err)
	}

	if err := s.Register(w); err != nil {
		log.Fatalf("cannot register webhook with server: %v", err)
	}

	if err := m.Start(signals.SetupSignalHandler()); err != nil {
		log.Fatalf("while or after starting manager: %v", err)
	}
}
