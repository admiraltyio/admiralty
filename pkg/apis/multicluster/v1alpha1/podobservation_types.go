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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodObservationSpec defines the desired state of PodObservation
type PodObservationSpec struct {
}

// PodObservationStatus defines the observed state of PodObservation
type PodObservationStatus struct {
	// +optional
	LiveState *corev1.Pod `json:"liveState,omitempty"`
	// +optional
	DelegateState *corev1.Pod `json:"delegateState,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodObservation is the Schema for the podobservations API
// +k8s:openapi-gen=true
// +kubebuilder:categories=observations
type PodObservation struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec PodObservationSpec `json:"spec,omitempty"`
	// +optional
	Status PodObservationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodObservationList contains a list of PodObservation
type PodObservationList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodObservation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodObservation{}, &PodObservationList{})
}
