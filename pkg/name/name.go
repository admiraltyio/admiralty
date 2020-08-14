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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const Short int = 63 // label values, namespaces, services, ingresses, anything else?
const Long int = 253 // other names

func FromParts(lengthLimit int, constantIndices []int, optionalIndices []int, parts ...string) string {
	m := make(map[int]bool, len(constantIndices))
	for _, i := range constantIndices {
		m[i] = true
	}
	hashRequired := false
	var nonEmptyParts []string
	for i, p := range parts {
		if p != "" {
			nonEmptyParts = append(nonEmptyParts, p)
			if strings.Contains(p, "-") && !m[i] && len(parts)-len(constantIndices) > 1 {
				hashRequired = true
			}
		} else if len(optionalIndices) > 1 {
			hashRequired = true
		}
	}
	s := strings.Join(nonEmptyParts, "-")
	if !hashRequired && len(s) <= lengthLimit {
		return s
	}
	key := strings.Join(parts, "/") // use / here because it's not allowed in parts, so join is unique (also, use empty parts here)
	return appendHash(lengthLimit, key, s)
}

func appendHash(lengthLimit int, key string, s string) string {
	hashLength := 10                                // TODO... configure
	prefix := truncate(lengthLimit-hashLength-1, s) // 1 for dash between prefix and hash
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(h[:])[:hashLength])
}

func truncate(lengthLimit int, s string) string {
	tooLongBy := len(s) - lengthLimit
	if tooLongBy > 0 {
		s = s[:len(s)-tooLongBy]
		for s[len(s)-1:] == "-" {
			s = s[:len(s)-1]
		}
	}
	return s
}
