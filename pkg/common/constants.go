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

var (
	KeyPrefix = "multicluster.admiralty.io/"

	// annotations on source pod (by user) and proxy pods (copied by mutating admission webhook)

	AnnotationKeyElect          = KeyPrefix + "elect"
	AnnotationKeyFederationName = KeyPrefix + "federationname"
	AnnotationKeyClusterName    = KeyPrefix + "clustername"

	// annotations on proxy pods (by mutating admission webhook)

	KeyPrefixSourcePod = KeyPrefix + "sourcepod-"

	AnnotationKeySourcePodManifest = KeyPrefixSourcePod + "manifest"

	// labels on delegate pods (by bind controller)

	KeyPrefixProxyPod = KeyPrefix + "proxypod-"

	LabelKeyProxyPodClusterName = KeyPrefixProxyPod + "clustername"
	LabelKeyProxyPodNamespace   = KeyPrefixProxyPod + "namespace"
	LabelKeyProxyPodName        = KeyPrefixProxyPod + "name"

	// labels on delegate services (by global service controller)

	LabelKeyIsDelegate = KeyPrefix + "is-delegate"
)
