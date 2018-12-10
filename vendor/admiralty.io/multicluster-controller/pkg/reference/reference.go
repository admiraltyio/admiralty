/*
Copyright 2018 The Multicluster-Controller Authors.

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

package reference // import "admiralty.io/multicluster-controller/pkg/reference"

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var key string = "multicluster.admiralty.io/controller-reference"

type MulticlusterOwnerReference struct {
	APIVersion string    `json:"apiVersion" protobuf:"bytes,5,opt,name=apiVersion"`
	Kind       string    `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	Name       string    `json:"name" protobuf:"bytes,3,opt,name=name"`
	UID        types.UID `json:"uid" protobuf:"bytes,4,opt,name=uid,casttype=k8s.io/apimachinery/pkg/types.UID"`
	// +optional
	Controller *bool `json:"controller,omitempty" protobuf:"varint,6,opt,name=controller"`
	// +optional
	BlockOwnerDeletion *bool `json:"blockOwnerDeletion,omitempty" protobuf:"varint,7,opt,name=blockOwnerDeletion"`

	ClusterName string `json:"clusterName" protobuf:"bytes,9,opt,name=clusterName"`
	Namespace   string `json:"namespace" protobuf:"bytes,8,opt,name=namespace"`
}

func NewMulticlusterOwnerReference(owner metav1.Object, gvk schema.GroupVersionKind, clusterName string) *MulticlusterOwnerReference {
	blockOwnerDeletion := true
	isController := true
	return &MulticlusterOwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,

		ClusterName: clusterName,
		Namespace:   owner.GetNamespace(),
	}
}

func GetMulticlusterControllerOf(o metav1.Object) *MulticlusterOwnerReference {
	// HACK: until we come up with a better solution for multicluster owner (and controller) references
	// suggestions: a MulticlusterOwnerReference CRD, or upstream support (along with ClusterName)
	a := o.GetAnnotations()
	s, ok := a[key]
	if !ok {
		return nil
	}

	r := &MulticlusterOwnerReference{}
	if err := json.Unmarshal([]byte(s), r); err != nil {
		return nil
	}
	return r
}

func SetMulticlusterControllerReference(o metav1.Object, ref *MulticlusterOwnerReference) error {
	a := o.GetAnnotations()
	if a == nil {
		a = make(map[string]string)
	}
	b, err := json.Marshal(&ref)
	if err != nil {
		return err
	}
	a[key] = string(b)
	o.SetAnnotations(a)
	return nil
}
