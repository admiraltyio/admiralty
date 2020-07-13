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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// ClusterSource is the Schema for the clustersources API
// +k8s:openapi-gen=true
type ClusterSource struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec ClusterSourceSpec `json:"spec,omitempty"`
	// +optional
	Status ClusterSourceStatus `json:"status,omitempty"`
}

type ClusterSourceSpec struct {
	// +optional
	UserName string `json:"userName,omitempty"`
	// +optional
	ServiceAccount *ServiceAccountReference `json:"serviceAccount,omitempty"`
}

type ServiceAccountReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type ClusterSourceStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// ClusterSourceList contains a list of ClusterSource
type ClusterSourceList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterSource{}, &ClusterSourceList{})
}
