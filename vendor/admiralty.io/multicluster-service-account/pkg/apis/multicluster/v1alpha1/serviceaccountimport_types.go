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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceAccountImportSpec defines the desired state of ServiceAccountImport
type ServiceAccountImportSpec struct {
	// TODO? some sort of MulticlusterObjectReference
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

// ServiceAccountImportStatus defines the observed state of ServiceAccountImport
type ServiceAccountImportStatus struct {
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Secrets []corev1.ObjectReference `json:"secrets,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceAccountImport is the Schema for the serviceaccountimports API
// +k8s:openapi-gen=true
type ServiceAccountImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceAccountImportSpec   `json:"spec,omitempty"`
	Status ServiceAccountImportStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceAccountImportList contains a list of ServiceAccountImport
type ServiceAccountImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceAccountImport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceAccountImport{}, &ServiceAccountImportList{})
}
