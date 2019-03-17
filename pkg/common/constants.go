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

package common

// TODO... standardize label and annotation propagation story

var (
	KeyPrefix = "multicluster.admiralty.io/"

	AnnotationKeyElect                       = KeyPrefix + "elect"
	AnnotationKeyClusterName                 = KeyPrefix + "clustername"
	AnnotationKeyServiceDependenciesSelector = KeyPrefix + "service-dependencies-selector"

	KeyPrefixSourcePod = KeyPrefix + "sourcepod-"

	AnnotationKeySourcePodManifest = KeyPrefixSourcePod + "manifest"

	KeyPrefixProxyPod = KeyPrefix + "proxypod-"

	AnnotationKeyProxyPodClusterName = KeyPrefixProxyPod + "clustername"
	AnnotationKeyProxyPodNamespace   = KeyPrefixProxyPod + "namespace"
	AnnotationKeyProxyPodName        = KeyPrefixProxyPod + "name"

	KeyPrefixOriginal = KeyPrefix + "original-"

	LabelKeyOriginalName        = KeyPrefixOriginal + "name"
	LabelKeyOriginalNamespace   = KeyPrefixOriginal + "namespace"
	LabelKeyOriginalClusterName = KeyPrefixOriginal + "clusterName"

	LabelKeyIsDelegate = KeyPrefix + "is-delegate"
)
