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
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeLabels(t *testing.T) {
	tests := []struct {
		name               string
		inputLabels        map[string]string
		noPrefixLabelRegex string
		outputLabels       map[string]string
		expectedErr        error
	}{
		{
			name: "mix of keys, no skip",
			inputLabels: map[string]string{
				"foo.com/bar":                   "a",
				"baz.com/foo":                   "b",
				"multicluster.admiralty.io/baz": "c",
				"a":                             "d",
			},
			noPrefixLabelRegex: "",
			outputLabels: map[string]string{
				"multicluster.admiralty.io/bar": "a",
				"multicluster.admiralty.io/foo": "b",
				"multicluster.admiralty.io/baz": "c",
				"multicluster.admiralty.io/a":   "d",
			},
		},
		{
			name: "skip multiple keys",
			inputLabels: map[string]string{
				"foo.com/bar":                   "a",
				"baz.com/foo":                   "b",
				"multicluster.admiralty.io/foo": "c",
				"a":                             "d",
			},
			noPrefixLabelRegex: "foo.com/bar|baz.com/foo",
			outputLabels: map[string]string{
				"foo.com/bar":                   "a",
				"baz.com/foo":                   "b",
				"multicluster.admiralty.io/foo": "c",
				"multicluster.admiralty.io/a":   "d",
			},
		},
		{
			name: "skip key/value pair",
			inputLabels: map[string]string{
				"foo.com/bar": "a",
				"baz.com/foo": "b",
				"a":           "d",
			},
			noPrefixLabelRegex: "^a=d$",
			outputLabels: map[string]string{
				"multicluster.admiralty.io/bar": "a",
				"multicluster.admiralty.io/foo": "b",
				"a":                             "d",
			},
		},
		{
			name: "skip value",
			inputLabels: map[string]string{
				"foo.com/bar": "skip",
			},
			noPrefixLabelRegex: "^.*=skip$",
			outputLabels: map[string]string{
				"foo.com/bar": "skip",
			},
		},
		{
			name: "invalid regex lookahead",
			inputLabels: map[string]string{
				"foo.com/bar": "a",
			},
			noPrefixLabelRegex: "?!foo",
			outputLabels:       nil,
			expectedErr:        errors.New("invalid regex"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, err := ChangeLabels(tt.inputLabels, tt.noPrefixLabelRegex)
			if tt.expectedErr != nil {
				require.Error(t, err)
			}
			require.Equal(t, tt.outputLabels, out)
		})
	}
}
