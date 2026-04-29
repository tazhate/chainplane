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

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		candidate, current, prefix string
		want                       bool
	}{
		{"v29.0", "v28.0", "", true},
		{"v28.0", "v29.0", "", false},
		{"v28.0", "v28.0", "", false},
		{"GreatVoyage-v4.9.0", "GreatVoyage-v4.8.1", "GreatVoyage-", true},
		{"GreatVoyage-v4.8.1", "GreatVoyage-v4.9.0", "GreatVoyage-", false},
		{"mainnet-v1.70.0", "mainnet-v1.69.2", "mainnet-", true},
		{"v2025.04", "v2025.03", "", true},
		{"not-a-version", "v1.0.0", "", false},
		{"v1.0.0", "not-a-version", "", false},
	}
	for _, tc := range cases {
		got := IsNewer(tc.candidate, tc.current, tc.prefix)
		if got != tc.want {
			t.Errorf("IsNewer(%q, %q, %q) = %v, want %v", tc.candidate, tc.current, tc.prefix, got, tc.want)
		}
	}
}

func TestNormalizeTag(t *testing.T) {
	cases := []struct {
		tag, prefix, want string
	}{
		{"v28.0", "", "v28.0.0"},
		{"28.0", "", "v28.0.0"},
		{"GreatVoyage-v4.8.1", "GreatVoyage-", "v4.8.1"},
		{"mainnet-v1.69.2", "mainnet-", "v1.69.2"},
		{"v2025.04", "", "v2025.4.0"},
	}
	for _, tc := range cases {
		got := normalizeTag(tc.tag, tc.prefix)
		if got != tc.want {
			t.Errorf("normalizeTag(%q, %q) = %q, want %q", tc.tag, tc.prefix, got, tc.want)
		}
	}
}
