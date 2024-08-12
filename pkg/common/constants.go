/*
 * Copyright 2023 The Multicluster-Scheduler Authors.
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

package common

var (
	ProxySchedulerName     = "admiralty-proxy"
	CandidateSchedulerName = "admiralty-candidate"

	LabelAndTaintKeyVirtualKubeletProvider = "virtual-kubelet.io/provider"
	VirtualKubeletProviderName             = "admiralty"

	KeyPrefix = "multicluster.admiralty.io/"

	// annotations on source pod (by user) and proxy pods (copied by mutating admission webhook)

	AnnotationKeyElect = KeyPrefix + "elect"

	// AnnotationKeyNoReservation tells the proxy scheduler to work with a custom scheduler in the target cluster,
	// instead of the candidate scheduler, waiting for candidate pods to be scheduled, instead of reserved,
	// to pass the proxy plugin filter test. Pods deleted in the post-bind plugin (those that didn't pass the score test)
	// may be scheduled already, not just pending. That is an acceptable compromise to work with a custom scheduler.
	// TODO: an alternative option would be to schedule based on virtual node info only (like tensile-kube).
	AnnotationKeyNoReservation = KeyPrefix + "no-reservation"

	AnnotationKeyProxyPodSchedulingConstraints = KeyPrefix + "proxy-pod-scheduling-constraints"

	AnnotationKeyUseConstraintsFromSpecForProxyPodScheduling = KeyPrefix + "use-constraints-from-spec-for-proxy-pod-scheduling"

	// AnnotationKeyDelegateLabelKeysToSkipPrefixing defines label keys that won't get prefixed with KeyPrefix
	// on the delegate pod and retain the original label key
	AnnotationKeyDelegateLabelKeysToSkipPrefixing = KeyPrefix + "label-keys-to-skip-prefixing"

	// annotations on proxy pods (by mutating admission webhook)

	KeyPrefixSourcePod = KeyPrefix + "sourcepod-"

	AnnotationKeySourcePodManifest = KeyPrefixSourcePod + "manifest"

	// annotations on delegate pod chaperons (by scheduler plugins)

	AnnotationKeyIsReserved = KeyPrefix + "is-reserved"
	AnnotationKeyIsAllowed  = KeyPrefix + "is-allowed"

	AnnotationKeyPodMissingSince = KeyPrefix + "pod-missing-since"

	// annotations on following services and ingresses (for cloud controller manager to configure DNS)

	AnnotationKeyGlobal = KeyPrefix + "global"

	// annotations on following objects (by follow controllers)

	AnnotationKeyIsDelegate = KeyPrefix + "is-delegate"

	// labels on proxy pods and services (used by post-delete hook to clean up finalizers)

	LabelKeyHasFinalizer = KeyPrefix + "has-finalizer"

	LabelKeyParentClusterName = KeyPrefix + "parent-cluster-name"
	// used to get pod chaperon (whose name is generated) given proxy pod ("list one" hack), without indexer
	LabelKeyParentUID            = KeyPrefix + "parent-uid"
	AnnotationKeyParentName      = KeyPrefix + "parent-name"
	AnnotationKeyParentNamespace = KeyPrefix + "parent-namespace"

	AnnotationKeyCiliumGlobalService = "io.cilium/global-service"

	AnnotationKeyOriginalSelector = KeyPrefix + "original-selector"

	AnnotationKeyRestartedAt = KeyPrefix + "restartedAt"

	LabelKeyTargetNamespace   = KeyPrefix + "target-namespace"
	LabelKeyTargetName        = KeyPrefix + "target-name"
	LabelKeyClusterTargetName = KeyPrefix + "cluster-target-name"
)
