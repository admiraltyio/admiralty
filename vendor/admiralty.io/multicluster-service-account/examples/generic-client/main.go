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
	"fmt"
	"os"

	"admiralty.io/multicluster-service-account/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	cfg, ns, err := config.ConfigAndNamespace()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("API server URL: %s\n", cfg.Host)
	fmt.Printf("Namespace: %s\n", ns)
	fmt.Printf("Listing pods...\n")
	pods, err := clientset.CoreV1().Pods(ns).List(metav1.ListOptions{})
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	for _, pod := range pods.Items {
		fmt.Printf("%s\n", pod.Name)
	}
}
