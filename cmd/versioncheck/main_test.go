/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/
package main

import "testing"

func TestIsStableTag(t *testing.T) {
	tests := []struct {
		name   string
		tag    string
		prefix string
		want   bool
	}{
		// Stable releases.
		{"plain v28.0", "v28.0", "", true},
		{"op-stack clean", "v1.101411.2", "", true},
		{"tron with prefix", "GreatVoyage-v4.8.1", "GreatVoyage-", true},
		{"tron prefix in tag no policy prefix", "GreatVoyage-v4.8.1", "", true},
		{"mainnet network prefix", "mainnet-v1.8.0", "", true},
		{"bare semver no v", "1.37.2", "", true},
		{"bare semver two-level", "10.6.2", "", true},
		{"single major", "v5", "", true},

		// Unstable / service builds.
		{"op-stack cdfpl", "v1.101511.1-cdfpl.1", "", false},
		{"op-stack synctest", "v1.101605.0-synctest.0", "", false},
		{"op-stack overrides", "v1.101308.2-overrides.1", "", false},
		{"rc dotted", "v1.2.3-rc.1", "", false},
		{"floating nightly", "nightly", "", false},
		{"floating latest", "latest", "", false},
		{"beta suffix", "v1.0.0-beta", "", false},
		{"debug suffix", "v1.2.3-debug", "", false},
		{"fork suffix", "v1.2.3-fork", "", false},
		{"untagged suffix", "v1.2.3-untagged", "", false},
		{"floating main", "main", "", false},
		{"floating master", "master", "", false},
		{"empty tag", "", "", false},
		{"non-version word", "stable", "", false},
		{"sha-like", "sha256-abc", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isStableTag(tt.tag, tt.prefix); got != tt.want {
				t.Errorf("isStableTag(%q, %q) = %v, want %v", tt.tag, tt.prefix, got, tt.want)
			}
		})
	}
}
