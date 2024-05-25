/*
 * Copyright 2024 The Multicluster-Scheduler Authors.
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

package feedback

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestFilterContainerStatuses(t *testing.T) {
	testcases := []struct {
		Name   string
		In     []string
		Has    []string
		Expect []string
	}{
		{
			Name:   "empty",
			In:     []string{},
			Has:    []string{},
			Expect: []string{},
		},
		{
			Name:   "has_all",
			In:     []string{"a", "b", "c"},
			Has:    []string{"a", "b", "c"},
			Expect: []string{"a", "b", "c"},
		},
		{
			Name:   "has_none",
			In:     []string{"a", "b", "c"},
			Has:    []string{},
			Expect: []string{},
		},
		{
			Name:   "has_first",
			In:     []string{"a", "b", "c"},
			Has:    []string{"a"},
			Expect: []string{"a"},
		},
		{
			Name:   "has_last",
			In:     []string{"a", "b", "c"},
			Has:    []string{"c"},
			Expect: []string{"c"},
		},
		{
			Name:   "missing_first",
			In:     []string{"a", "b", "c"},
			Has:    []string{"b", "c"},
			Expect: []string{"b", "c"},
		},
		{
			Name:   "missing_last",
			In:     []string{"a", "b", "c"},
			Has:    []string{"a", "b"},
			Expect: []string{"a", "b"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			in := []corev1.ContainerStatus{}
			for _, name := range tc.In {
				in = append(in, corev1.ContainerStatus{Name: name})
			}

			has := func(name string) bool {
				for _, n := range tc.Has {
					if name == n {
						return true
					}
				}
				return false
			}

			out := filterContainerStatuses(in, has)
			outNames := []string{}
			for _, s := range out {
				outNames = append(outNames, s.Name)
			}
			if !reflect.DeepEqual(outNames, tc.Expect) {
				t.Fatalf("expected %v, got %v", tc.Expect, outNames)
			}

			// test if input was returned as-is or copied
			if len(in) > 0 && len(out) > 0 {
				copied := &in[0] != &out[0]
				if reflect.DeepEqual(tc.In, tc.Expect) {
					if copied {
						t.Fatalf("expected input to be returned as-is when no filtering occurred")
					}
				} else {
					if !copied {
						t.Fatalf("expected input to be copied when filtering occurred")
					}
				}
			}
		})
	}
}
