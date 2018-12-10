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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MulticlusterDeploymentObservationSpec defines the desired state of MulticlusterDeploymentObservation
type MulticlusterDeploymentObservationSpec struct {
}

// MulticlusterDeploymentObservationStatus defines the observed state of MulticlusterDeploymentObservation
type MulticlusterDeploymentObservationStatus struct {
	// +optional
	LiveState *MulticlusterDeployment `json:"liveState,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MulticlusterDeploymentObservation is the Schema for the multiclusterdeploymentobservations API
// +k8s:openapi-gen=true
// +kubebuilder:categories=observations
type MulticlusterDeploymentObservation struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec MulticlusterDeploymentObservationSpec `json:"spec,omitempty"`
	// +optional
	Status MulticlusterDeploymentObservationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MulticlusterDeploymentObservationList contains a list of MulticlusterDeploymentObservation
type MulticlusterDeploymentObservationList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MulticlusterDeploymentObservation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MulticlusterDeploymentObservation{}, &MulticlusterDeploymentObservationList{})
}
