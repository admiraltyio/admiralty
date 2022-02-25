/*
 * Copyright 2022 The Multicluster-Scheduler Authors.
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

package resources

import (
	"regexp"
	"testing"

	"admiralty.io/multicluster-scheduler/pkg/config/agent"
	"admiralty.io/multicluster-scheduler/pkg/model/virtualnode"
	"github.com/stretchr/testify/require"
)

func Test_upstream_reconcileLabels(t *testing.T) {
	clusterSummaryLabels := map[string]string{"k1": "v1", "k2": "v2", "prefix.io/k1": "v1"}
	addBaseLabels := func(m map[string]string) map[string]string {
		l := virtualnode.BaseLabels("target-namespace", "target-name")
		for k, v := range m {
			l[k] = v
		}
		return l
	}
	tests := []struct {
		name                 string
		excludedLabelsRegexp *regexp.Regexp
		want                 map[string]string
	}{{
		name:                 "no exclusions",
		excludedLabelsRegexp: nil,
		want:                 addBaseLabels(map[string]string{"k1": "v1", "k2": "v2", "prefix.io/k1": "v1"}),
	}, {
		name:                 "exclude exact aggregated key",
		excludedLabelsRegexp: regexp.MustCompile(`^k1=`),
		want:                 addBaseLabels(map[string]string{"k2": "v2", "prefix.io/k1": "v1"}),
	}, {
		name:                 "exclude aggregated key suffix",
		excludedLabelsRegexp: regexp.MustCompile(`k1=`),
		want:                 addBaseLabels(map[string]string{"k2": "v2"}),
	}, {
		name:                 "exclude exact aggregated prefix",
		excludedLabelsRegexp: regexp.MustCompile(`^prefix\.io/`),
		want:                 addBaseLabels(map[string]string{"k1": "v1", "k2": "v2"}),
	}, {
		name:                 "exclude exact aggregated key-value pair",
		excludedLabelsRegexp: regexp.MustCompile(`^k1=v1$`),
		want:                 addBaseLabels(map[string]string{"k2": "v2", "prefix.io/k1": "v1"}),
	}, {
		name:                 "exclude two keys",
		excludedLabelsRegexp: regexp.MustCompile(`^k1=|^k2=`),
		want:                 addBaseLabels(map[string]string{"prefix.io/k1": "v1"}),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := upstream{
				target:                       agent.Target{Name: "target-name", Namespace: "target-namespace"},
				compiledExcludedLabelsRegexp: tt.excludedLabelsRegexp,
			}
			got := r.reconcileLabels(clusterSummaryLabels)
			require.Equal(t, tt.want, got)
		})
	}
}
