/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package registry

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// IsNewer reports whether candidate is a newer version than current,
// after stripping the given prefix from both. Returns false if either
// is not a valid semver (with or without "v" prefix).
func IsNewer(candidate, current, prefix string) bool {
	c := normalizeTag(candidate, prefix)
	cur := normalizeTag(current, prefix)
	if !semver.IsValid(c) || !semver.IsValid(cur) {
		return false
	}
	return semver.Compare(c, cur) > 0
}

// Newest returns the tag from the slice that is greatest by semver under the
// given prefix. Tags that fail semver parsing are skipped. Returns "" if no
// candidate parses successfully. Callers should pre-filter pre-releases /
// floating tags before passing tags in.
func Newest(tags []string, prefix string) string {
	best := ""
	for _, t := range tags {
		if !semver.IsValid(normalizeTag(t, prefix)) {
			continue
		}
		if best == "" || IsNewer(t, best, prefix) {
			best = t
		}
	}
	return best
}

// normalizeTag strips prefix, ensures a leading "v", and pads to vMAJOR.MINOR.PATCH
// to satisfy golang.org/x/mod/semver strict validation (no leading zeros, 3 parts).
func normalizeTag(tag, prefix string) string {
	s := strings.TrimPrefix(tag, prefix)
	if !strings.HasPrefix(s, "v") {
		s = "v" + s
	}
	core := s
	var suffix string
	if idx := strings.IndexAny(s[1:], "-+"); idx >= 0 {
		core = s[:idx+1]
		suffix = s[idx+1:]
	}
	vParts := strings.Split(strings.TrimPrefix(core, "v"), ".")
	normalized := make([]string, len(vParts))
	for i, p := range vParts {
		if n, err := strconv.ParseUint(p, 10, 64); err == nil {
			normalized[i] = fmt.Sprintf("%d", n)
		} else {
			normalized[i] = p
		}
	}
	for len(normalized) < 3 {
		normalized = append(normalized, "0")
	}
	result := "v" + strings.Join(normalized, ".")
	if suffix != "" {
		result += suffix
	}
	return result
}
