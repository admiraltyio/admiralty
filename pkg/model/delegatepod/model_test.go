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

package delegatepod

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeLabels(t *testing.T) {
	tests := []struct {
		name                     string
		inputLabels              map[string]string
		labelKeysToSkipPrefixing string
		outputLabels             map[string]string
	}{
		{
			name: "mix of domains, no skip",
			inputLabels: map[string]string{
				"foo.com/bar":                   "a",
				"baz.com/foo":                   "b",
				"multicluster.admiralty.io/baz": "c",
				"a":                             "d",
			},
			labelKeysToSkipPrefixing: "",
			outputLabels: map[string]string{
				"multicluster.admiralty.io/bar": "a",
				"multicluster.admiralty.io/foo": "b",
				"multicluster.admiralty.io/baz": "c",
				"multicluster.admiralty.io/a":   "d",
			},
		},
		{
			name: "skip multiple domains",
			inputLabels: map[string]string{
				"foo.com/bar":                   "a",
				"baz.com/foo":                   "b",
				"multicluster.admiralty.io/foo": "c",
				"a":                             "d",
			},
			labelKeysToSkipPrefixing: "foo.com/bar,baz.com/foo",
			outputLabels: map[string]string{
				"foo.com/bar":                   "a",
				"baz.com/foo":                   "b",
				"multicluster.admiralty.io/foo": "c",
				"multicluster.admiralty.io/a":   "d",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _ := ChangeLabels(tt.inputLabels, tt.labelKeysToSkipPrefixing)
			require.Equal(t, tt.outputLabels, out)
		})
	}
}
