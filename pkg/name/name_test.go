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

func TestFromNamespaceName(t *testing.T) {
	cases := []struct{ namespace, name, expected, description string }{
		{
			"default",
			"foo",
			"default-foo-947c8f7",
			"normal",
		},
		{
			"verylongnamespace-verylongnamespace-verylongnamespace",
			"verylongname",
			"verylongnamespace-verylongnamespace-verylongnamespace-v-9190b58",
			"too long",
		},
		{
			"verylongnamespace-verylongnamespace-verylo",
			"verylongname",
			"verylongnamespace-verylongnamespace-verylo-verylongname-274a70c",
			"at the limit",
		},
		{
			"verylongnamespace-verylongnamespace-verylong",
			"very-long-name",
			"verylongnamespace-verylongnamespace-verylong-very-long-8cddda0",
			"remove trailing dash",
		},
	}
	for _, c := range cases {
		n := FromParts(Short, nil, nil, c.namespace, c.name)
		if n != c.expected {
			t.Errorf("%s failed: %s should be %s", c.description, n, c.expected)
		}
	}
	// TODO test constant and optional indices
}
