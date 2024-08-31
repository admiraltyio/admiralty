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
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
)

func TestIsPodUnschedulable(t *testing.T) {
	tests := []struct {
		name            string
		conditionStatus []v1.PodCondition
		want            bool
	}{
		{
			name:            "reason pod unschedulable",
			conditionStatus: []v1.PodCondition{{Status: v1.ConditionFalse, Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable}},
			want:            true,
		},
		{
			name:            "scheduling gated",
			conditionStatus: []v1.PodCondition{{Status: v1.ConditionFalse, Type: v1.PodScheduled, Reason: v1.PodReasonSchedulingGated}},
			want:            true,
		},
		{
			name:            "pod scheduled",
			conditionStatus: []v1.PodCondition{{Status: v1.ConditionTrue, Type: v1.PodScheduled}},
			want:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podChaperon := &v1alpha1.PodChaperon{
				Status: v1.PodStatus{Conditions: tt.conditionStatus},
			}
			res := isCandidatePodUnschedulable(podChaperon)
			require.Equal(t, tt.want, res)
		})
	}
}
