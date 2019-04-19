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

package main

import (
	"log"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/manager"
	"admiralty.io/multicluster-scheduler/pkg/controllers/globalsvc"
	"admiralty.io/multicluster-scheduler/pkg/controllers/schedule"
	"admiralty.io/multicluster-scheduler/pkg/scheduler"
	"admiralty.io/multicluster-service-account/pkg/config"
	"k8s.io/sample-controller/pkg/signals"
)

func main() {
	cfg, _, err := config.ConfigAndNamespace()
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}
	cl := cluster.New("", cfg, cluster.Options{})

	m := manager.New()

	co, err := schedule.NewController(cl, scheduler.New())
	if err != nil {
		log.Fatalf("cannot create schedule controller: %v", err)
	}
	m.AddController(co)

	co, err = globalsvc.NewController(cl)
	if err != nil {
		log.Fatalf("cannot create globalsvc controller: %v", err)
	}
	m.AddController(co)

	if err := m.Start(signals.SetupSignalHandler()); err != nil {
		log.Fatalf("while or after starting manager: %v", err)
	}
}
