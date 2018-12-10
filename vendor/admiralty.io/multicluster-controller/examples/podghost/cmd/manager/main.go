/*
Copyright 2018 The Multicluster-Controller Authors.

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
	"flag"
	"log"

	"admiralty.io/multicluster-controller/examples/podghost/pkg/controller/podghost"
	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/manager"
	"admiralty.io/multicluster-service-account/pkg/config"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/sample-controller/pkg/signals"
)

func main() {
	flag.Parse()
	if flag.NArg() != 2 {
		log.Fatalf("Usage: deploymentcopy sourcecontext destinationcontext")
	}
	srcCtx, dstCtx := flag.Arg(0), flag.Arg(1)

	cfg, _, err := config.NamedConfigAndNamespace(srcCtx)
	if err != nil {
		log.Fatal(err)
	}
	live := cluster.New(srcCtx, cfg, cluster.Options{})

	cfg, _, err = config.NamedConfigAndNamespace(dstCtx)
	if err != nil {
		log.Fatal(err)
	}
	ghost := cluster.New(dstCtx, cfg, cluster.Options{})

	co, err := podghost.NewController(live, ghost, "default")
	if err != nil {
		log.Fatalf("creating podghost controller: %v", err)
	}

	m := manager.New()
	m.AddController(co)

	if err := m.Start(signals.SetupSignalHandler()); err != nil {
		log.Fatalf("while or after starting manager: %v", err)
	}
}
