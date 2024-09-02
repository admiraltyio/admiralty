/*
 * Copyright The Multicluster-Scheduler Authors.
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

package proxy

import (
	v1 "k8s.io/api/core/v1"

	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
)

func isCandidatePodUnschedulable(c *v1alpha1.PodChaperon) bool {
	for _, cond := range c.Status.Conditions {
		if cond.Type == v1.PodScheduled && cond.Status == v1.ConditionFalse && (cond.Reason == v1.PodReasonUnschedulable || cond.Reason == v1.PodReasonSchedulingGated) {
			return true
		}
	}
	return false
}
