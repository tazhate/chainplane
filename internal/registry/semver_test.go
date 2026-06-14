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

func TestNewest(t *testing.T) {
	cases := []struct {
		name   string
		tags   []string
		prefix string
		want   string
	}{
		{
			name: "registry returns mixed order picks max",
			tags: []string{"v27.2", "v28.0", "v27.1", "v26.5"},
			want: "v28.0",
		},
		{
			name: "CalVer year-month",
			tags: []string{"v2024.08-1", "v2026.02-1", "v2025.06-1"},
			want: "v2026.02-1",
		},
		{
			name:   "with prefix",
			tags:   []string{"GreatVoyage-v4.8.0", "GreatVoyage-v4.8.1", "GreatVoyage-v4.7.9"},
			prefix: "GreatVoyage-",
			want:   "GreatVoyage-v4.8.1",
		},
		{
			name: "skips garbage tags",
			tags: []string{"nightly", "v1.2.3", "stable"},
			want: "v1.2.3",
		},
		{
			name: "empty input",
			tags: nil,
			want: "",
		},
		{
			name: "all invalid",
			tags: []string{"foo", "bar"},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Newest(tc.tags, tc.prefix)
			if got != tc.want {
				t.Errorf("Newest(%v, %q) = %q, want %q", tc.tags, tc.prefix, got, tc.want)
			}
		})
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
