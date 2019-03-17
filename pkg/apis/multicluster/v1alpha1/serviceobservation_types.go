/*
Copyright 2019 The Multicluster-Scheduler Authors.

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

// ServiceObservationSpec defines the desired state of ServiceObservation
type ServiceObservationSpec struct {
}

// ServiceObservationStatus defines the observed state of ServiceObservation
type ServiceObservationStatus struct {
	// +optional
	LiveState *corev1.Service `json:"liveState,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceObservation is the Schema for the serviceobservations API
// +k8s:openapi-gen=true
// +kubebuilder:categories=observations
type ServiceObservation struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec ServiceObservationSpec `json:"spec,omitempty"`
	// +optional
	Status ServiceObservationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceObservationList contains a list of ServiceObservation
type ServiceObservationList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceObservation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceObservation{}, &ServiceObservationList{})
}
