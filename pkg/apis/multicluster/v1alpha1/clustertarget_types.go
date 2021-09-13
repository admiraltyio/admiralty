/*
 * Copyright 2021 The Multicluster-Scheduler Authors.
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

// ClusterTarget is the Schema for the clustertargets API
// +k8s:openapi-gen=true
type ClusterTarget struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec ClusterTargetSpec `json:"spec,omitempty"`
	// +optional
	Status ClusterTargetStatus `json:"status,omitempty"`
}

type ClusterTargetSpec struct {
	// +optional
	Self bool `json:"self,omitempty"`
	// +optional
	KubeconfigSecret *ClusterKubeconfigSecret `json:"kubeconfigSecret,omitempty"`
	// +optional
	ExcludedLabelsRegexp *string `json:"excludedLabelsRegexp,omitempty"`
}

type ClusterKubeconfigSecret struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// +optional
	Key string `json:"key,omitempty"`
	// +optional
	Context string `json:"context,omitempty"`
}

type ClusterTargetStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// ClusterTargetList contains a list of ClusterTarget
type ClusterTargetList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterTarget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterTarget{}, &ClusterTargetList{})
}
