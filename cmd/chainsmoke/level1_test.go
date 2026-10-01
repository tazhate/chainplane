/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

// TestSmokeSampleSkipsEmptyImage runs a real image-required sample: it must
// be a SKIP before any docker call, not a failed pull of an empty reference.
func TestSmokeSampleSkipsEmptyImage(t *testing.T) {
	if img := adapters.DefaultImages()[chainsv1alpha2.ChainAurora][""]; img != "" {
		t.Skipf("aurora has a default image now (%s)", img)
	}
	samples, err := loadSamples(samplesDir)
	if err != nil {
		t.Fatal(err)
	}
	var aurora *sample
	for i := range samples {
		if samples[i].chain == chainsv1alpha2.ChainAurora {
			aurora = &samples[i]
		}
	}
	if aurora == nil {
		t.Fatal("no aurora sample")
	}

	opts := options{namePrefix: "smoketest", tmpfsSize: "1g", out: t.TempDir()}
	r := smokeSample(t.Context(), opts, level1Env{images: newImageSet(false)}, smokeJob{key: "aurora", sample: *aurora})
	if r.status != statusSkip || r.detail != "no default image (spec.image required)" {
		t.Errorf("result = %s %q, want SKIP no default image", r.status, r.detail)
	}
}

func TestEngineAtLeast(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"29.8.1", true},
		{"26.0.0", true},
		{"26.0.0-rc1", true},
		{"25.0.5", false},
		{"", false},
		{"dev", false},
	}
	for _, tt := range tests {
		if got := engineAtLeast(tt.version, 26); got != tt.want {
			t.Errorf("engineAtLeast(%q, 26) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestParseFlagsLevel1Limits(t *testing.T) {
	opts, err := parseFlags([]string{"--level", "1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opts.nofile != 1048576 || opts.namePrefix != "chainsmoke" {
		t.Errorf("defaults: nofile %d, name-prefix %q", opts.nofile, opts.namePrefix)
	}
	for _, args := range [][]string{
		{"--nofile", "-1"},
		{"--name-prefix", "-bad"},
		{"--name-prefix", "a/b"},
	} {
		if _, err := parseFlags(args, nil); err == nil {
			t.Errorf("parseFlags(%q) accepted", args)
		}
	}
}
