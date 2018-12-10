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

// DeploymentTemplateSpec describes the data a deployment should have when created from a template
type DeploymentTemplateSpec struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec appsv1.DeploymentSpec `json:"spec,omitempty"`
}

// DeploymentDecisionSpec defines the desired state of DeploymentDecision
type DeploymentDecisionSpec struct {
	Template DeploymentTemplateSpec `json:"template"`
}

// DeploymentDecisionStatus defines the observed state of DeploymentDecision
type DeploymentDecisionStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeploymentDecision is the Schema for the deploymentdecisions API
// +k8s:openapi-gen=true
// +kubebuilder:categories=decisions
type DeploymentDecision struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// +optional
	Spec DeploymentDecisionSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// +optional
	Status DeploymentDecisionStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeploymentDecisionList contains a list of DeploymentDecision
type DeploymentDecisionList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeploymentDecision `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeploymentDecision{}, &DeploymentDecisionList{})
}
