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

package name

import (
	"testing"
)

func Test(t *testing.T) {
	cases := []struct {
		constantIndices []int
		optionalIndices []int
		parts           []string
		expected        string
		description     string
	}{
		{
			nil,
			nil,
			[]string{"default", "foo"},
			"default-foo",
			"no hyphen",
		},
		{
			nil,
			nil,
			[]string{"default", "foo-bar"},
			"default-foo-bar-b1f5f88a38",
			"with hyphen",
		},
		{
			nil,
			nil,
			[]string{"default-foo", "bar"},
			"default-foo-bar-ce0b03a405",
			"with hyphen, different hash",
		},
		{
			nil,
			nil,
			[]string{"verylongnamespaceverylongnamespace", "verylongnameverylongnameverylongname"},
			"verylongnamespaceverylongnamespace-verylongnameveryl-79f7165434",
			"too long",
		},
		{
			nil,
			nil,
			[]string{"verylongnamespaceverylongnamespace", "verylongnamevery-longnameverylongname"},
			"verylongnamespaceverylongnamespace-verylongnamevery-f098147021", // 62 characters long
			"remove trailing hyphen",
		},
		{
			[]int{0},
			nil,
			[]string{"prefix", "foo-bar"},
			"prefix-foo-bar",
			"single variable part with hyphen",
		},
		{
			nil,
			[]int{0},
			[]string{"", "foo"},
			"foo",
			"single non-optional part",
		},
		{
			nil,
			[]int{0, 1},
			[]string{"", "foo"},
			"foo-6f64c6e626",
			"multiple optional parts",
		},
		{
			nil,
			[]int{0, 1},
			[]string{"foo", ""},
			"foo-14fe48f0fb",
			"multiple optional parts, different hash",
		},
	}
	for _, c := range cases {
		n := FromParts(Short, c.constantIndices, c.optionalIndices, c.parts...)
		if n != c.expected {
			t.Errorf("%s failed: %s should be %s", c.description, n, c.expected)
		}
	}
}
