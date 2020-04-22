/*
 * Copyright 2020 The Multicluster-Scheduler Authors.
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

package main

import (
	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-service-account/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
)

func main() {
	patch := `{"metadata":{"$deleteFromPrimitiveList/finalizers":["multicluster.admiralty.io/multiclusterForegroundDeletion"]}}`

	cfg, _, err := config.ConfigAndNamespace()
	utilruntime.Must(err)

	k, err := kubernetes.NewForConfig(cfg)
	utilruntime.Must(err)

	p := patchAll{k, patch}

	p.patchPods()
	p.patchServices()
}

type patchAll struct {
	k     *kubernetes.Clientset
	patch string
}

func (p patchAll) patchPods() {
	l, err := p.k.CoreV1().Pods("").List(metav1.ListOptions{LabelSelector: common.LabelKeyHasFinalizer})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.CoreV1().Pods(o.Namespace).Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchServices() {
	l, err := p.k.CoreV1().Services("").List(metav1.ListOptions{LabelSelector: common.LabelKeyHasFinalizer})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.CoreV1().Services(o.Namespace).Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}
