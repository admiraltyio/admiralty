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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Scheduler struct {
	metav1.TypeMeta `json:",inline"`
	Clusters        []Cluster `json:"clusters"`
}

type Cluster struct {
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"`
	Context    string `json:"context"`
}

type Agent struct {
	metav1.TypeMeta `json:",inline"`
	Webhook         Webhook `json:"webhook"`
}

type Webhook struct {
	Port    int    `json:"port"`
	CertDir string `json:"certDir"`
}
