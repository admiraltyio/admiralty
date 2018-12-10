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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MulticlusterDeploymentStatus defines the observed state of MulticlusterDeployment
type MulticlusterDeploymentStatus struct {
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// +optional
	LabelSelector string `json:"labelSelector,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MulticlusterDeployment is the Schema for the multiclusterdeployments API
// +k8s:openapi-gen=true
// +kubebuilder:categories=multicluster
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.labelSelector
type MulticlusterDeployment struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec appsv1.DeploymentSpec `json:"spec,omitempty"`
	// +optional
	Status MulticlusterDeploymentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MulticlusterDeploymentList contains a list of MulticlusterDeployment
type MulticlusterDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MulticlusterDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MulticlusterDeployment{}, &MulticlusterDeploymentList{})
}
