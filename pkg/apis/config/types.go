/*
 * Copyright 2020 The Multicluster-Scheduler Authors.
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

package config

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProxyArgs holds arguments used to configure the Proxy plugin.
type ProxyArgs struct {
	metav1.TypeMeta

	// FilterWaitDurationSeconds specifies how long the filter extension point waits for a terminal
	// signal (reserved or unscheduled) from the pod chaperon
	FilterWaitDurationSeconds int32
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CandidateArgs holds arguments used to configure the Candidate plugin.
type CandidateArgs struct {
	metav1.TypeMeta

	// PreBindWaitDurationSeconds specifies how long the PreBind extension point waits for the proxy
	// scheduler to allow a pod
	PreBindWaitDurationSeconds int32
}
