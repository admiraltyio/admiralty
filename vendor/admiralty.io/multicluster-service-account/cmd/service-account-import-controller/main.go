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

	"admiralty.io/multicluster-service-account/pkg/apis"
	"admiralty.io/multicluster-service-account/pkg/config"
	"admiralty.io/multicluster-service-account/pkg/importer"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/sample-controller/pkg/signals"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	cfg, _, err := config.ConfigAndNamespaceForContext("")
	// here, we just want to make sure we DON'T use a service account import,
	// which, if there is only one, would be loaded by config.ConfigAndNamespace()
	// TODO: create function with better name or use client-go directly
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	m, err := manager.New(cfg, manager.Options{})
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	all, err := config.AllNamedConfigsAndNamespaces()
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	if err := apis.AddToScheme(m.GetScheme()); err != nil {
		log.Fatalf("%v\n", err)
	}

	if err := importer.Add(m, all); err != nil {
		log.Fatalf("%v\n", err)
	}

	if err := m.Start(signals.SetupSignalHandler()); err != nil {
		log.Fatalf("while or after starting manager: %v", err)
	}
}
