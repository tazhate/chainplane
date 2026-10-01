/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/
package main

import (
	"errors"
	"testing"

	"github.com/tazhate/chainplane/internal/adapters"
	"github.com/tazhate/chainplane/internal/registry"
)

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

func tagEntries(tags ...string) []registry.TagEntry {
	out := make([]registry.TagEntry, 0, len(tags))
	for _, t := range tags {
		out = append(out, registry.TagEntry{Tag: t})
	}
	return out
}

func TestPickLatest(t *testing.T) {
	semverPolicy := adapters.ChainVersionPolicy{TagPattern: `^v\d+\.\d+\.\d+$`}
	stellarPolicy := adapters.ChainVersionPolicy{
		TagPattern: `^(?P<version>\d+\.\d+\.\d+)-\d+\.[0-9a-f]+\.noble$`,
	}
	kavaPolicy := adapters.ChainVersionPolicy{TagPattern: `^(?P<version>v\d+\.\d+\.\d+)-goleveldb$`}

	tests := []struct {
		name       string
		policy     adapters.ChainVersionPolicy
		current    string
		tags       []registry.TagEntry
		wantLatest string
		wantNewer  bool
		wantMajor  bool
		wantSame   string
	}{
		{
			name: "minor bump", policy: semverPolicy, current: "v1.4.7",
			tags:       tagEntries("v1.4.7", "v1.5.5", "v1.5.0-rc.1"),
			wantLatest: "v1.5.5", wantNewer: true,
		},
		{
			name: "major bump", policy: semverPolicy, current: "v29.17.0",
			tags:       tagEntries("v29.17.0", "v31.1.0"),
			wantLatest: "v31.1.0", wantNewer: true, wantMajor: true,
		},
		{
			name: "major with same-line update", policy: semverPolicy, current: "v1.36.1",
			tags:       tagEntries("v1.36.1", "v1.38.0", "v1.39.3", "v2.0.0"),
			wantLatest: "v2.0.0", wantNewer: true, wantMajor: true, wantSame: "v1.39.3",
		},
		{
			name: "zero-x minor is major", policy: semverPolicy, current: "v0.5.7",
			tags:       tagEntries("v0.5.7", "v0.6.3"),
			wantLatest: "v0.6.3", wantNewer: true, wantMajor: true,
		},
		{
			name: "up to date", policy: semverPolicy, current: "v2.0.0",
			tags:       tagEntries("v1.9.0", "v2.0.0"),
			wantLatest: "v2.0.0",
		},
		{
			name: "no match", policy: semverPolicy, current: "v1.0.0",
			tags: tagEntries("latest", "v1.1.0-beta.1"),
		},
		{
			name: "version group with build suffix", policy: stellarPolicy, current: "28.1.0-3001.abc1234.noble",
			tags: tagEntries(
				"28.1.0-3001.abc1234.noble", "29.0.0-3589.4eb833373.noble",
				"29.0.0-3589.4eb833373.jammy", "29.1.0rc1-3600.1234abc.noble", "29",
			),
			wantLatest: "29.0.0-3589.4eb833373.noble", wantNewer: true, wantMajor: true,
		},
		{
			name: "version group with legacy pin", policy: stellarPolicy, current: "v19.12.0",
			tags:       tagEntries("29.0.0-3589.4eb833373.noble"),
			wantLatest: "29.0.0-3589.4eb833373.noble", wantNewer: true, wantMajor: true,
		},
		{
			name: "version group suffix pin", policy: kavaPolicy, current: "v0.28.2-goleveldb",
			tags:       tagEntries("v0.28.2-goleveldb", "v0.28.3-goleveldb", "v0.28.3"),
			wantLatest: "v0.28.3-goleveldb", wantNewer: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pickLatest(tt.policy, tt.current, tt.tags)
			if err != nil {
				t.Fatalf("pickLatest: %v", err)
			}
			want := pick{latest: tt.wantLatest, newer: tt.wantNewer, major: tt.wantMajor, sameMajor: tt.wantSame}
			if got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestFilterNewerHoldsMajors(t *testing.T) {
	results := []versionResult{
		{Chain: "a", IsNewer: true},
		{Chain: "b", IsNewer: true, IsMajor: true, LatestTag: "v2.0.0"},
		{Chain: "c"},
		{Chain: "d", IsNewer: true, IsMajor: true, LatestTag: "v2.0.0", SameMajorTag: "v1.9.0"},
	}
	got := filterNewer(results, false)
	if len(got) != 2 || got[0].Chain != "a" || got[1].Chain != "d" || got[1].LatestTag != "v1.9.0" {
		t.Fatalf("without allow-major got %+v", got)
	}
	got = filterNewer(results, true)
	if len(got) != 3 || got[2].LatestTag != "v2.0.0" {
		t.Fatalf("with allow-major got %+v", got)
	}
}

func TestResultStatusAndFailures(t *testing.T) {
	results := []versionResult{
		{Chain: "ok"},
		{Chain: "nomatch", NoMatch: true},
		{Chain: "err", Err: errors.New("boom")},
		{Chain: "major", IsNewer: true, IsMajor: true},
	}
	want := []string{"up to date", "NO MATCH", "ERROR: boom", "MAJOR AVAILABLE"}
	for i, r := range results {
		if got := resultStatus(r); got != want[i] {
			t.Errorf("%s: status %q, want %q", r.Chain, got, want[i])
		}
	}
	if got := countFailures(results); got != 2 {
		t.Fatalf("countFailures = %d, want 2", got)
	}
}
