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

package main

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
)

// manifestCheck is the level 0 verdict for one image reference.
type manifestCheck struct {
	status string
	detail string
}

// runLevel0 inspects every distinct image reference once and fans the
// verdict out to all chain/client rows that use it.
func runLevel0(ctx context.Context, targets []target, parallel int) []result {
	var refs []string
	for _, t := range targets {
		if t.image != "" && !slices.Contains(refs, t.image) {
			refs = append(refs, t.image)
		}
	}

	checks := forEach(ctx, parallel, refs, func(ctx context.Context, ref string) result {
		c := inspectManifest(ctx, ref)
		return result{chain: "image", client: ref, status: c.status, detail: c.detail}
	})
	byRef := make(map[string]result, len(refs))
	for i, ref := range refs {
		byRef[ref] = checks[i]
	}

	out := make([]result, 0, len(targets))
	for _, t := range targets {
		r := result{chain: t.chain, client: t.client, image: t.image, level: 0}
		if t.image == "" {
			r.status, r.detail = statusSkip, "no default image"
		} else {
			c := byRef[t.image]
			r.status, r.detail = c.status, c.detail
		}
		out = append(out, r)
	}
	return out
}

// inspectManifest checks that ref exists and has a linux/amd64 image.
func inspectManifest(ctx context.Context, ref string) manifestCheck {
	if ctx.Err() != nil {
		return manifestCheck{statusSkip, "interrupted"}
	}
	raw, err := dockerRetry(ctx, "buildx", "imagetools", "inspect", "--raw", ref)
	if err != nil {
		return manifestCheck{statusFail, firstLine(err.Error())}
	}
	platforms, isIndex, err := platformsFromRaw([]byte(raw))
	if err != nil {
		return manifestCheck{statusFail, "unparseable manifest: " + err.Error()}
	}

	if !isIndex && len(platforms) == 0 {
		// Single-platform manifest: the platform lives in the image config.
		cfg, err := dockerRetry(ctx, "buildx", "imagetools", "inspect", "--format", "{{json .Image}}", ref)
		if err != nil {
			return manifestCheck{statusFail, firstLine(err.Error())}
		}
		var img struct {
			OS           string `json:"os"`
			Architecture string `json:"architecture"`
			Variant      string `json:"variant"`
		}
		if err := json.Unmarshal([]byte(cfg), &img); err != nil {
			return manifestCheck{statusFail, "unparseable image config: " + err.Error()}
		}
		platforms = []string{platformString(img.OS, img.Architecture, img.Variant)}
	}

	kind := "single"
	if isIndex {
		kind = "index"
	}
	detail := kind + ": " + strings.Join(platforms, ", ")
	if !slices.Contains(platforms, smokePlatform) {
		return manifestCheck{statusFail, "no " + smokePlatform + " (" + detail + ")"}
	}
	return manifestCheck{statusPass, detail}
}

// platformsFromRaw lists the real platforms of a raw manifest. isIndex is
// true for OCI indexes and Docker manifest lists; for a single manifest the
// platform is only known for legacy schema 1 manifests, so platforms is
// usually empty and the caller has to read the image config.
func platformsFromRaw(raw []byte) (platforms []string, isIndex bool, err error) {
	var m struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
		Architecture  string `json:"architecture"` // schema 1 only
		Manifests     []struct {
			Platform *struct {
				OS           string `json:"os"`
				Architecture string `json:"architecture"`
				Variant      string `json:"variant"`
			} `json:"platform"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false, err
	}

	switch {
	case m.Manifests != nil || strings.Contains(m.MediaType, "index") || strings.Contains(m.MediaType, "manifest.list"):
		for _, d := range m.Manifests {
			p := d.Platform
			// Attestation manifests carry unknown/unknown.
			if p == nil || p.OS == "unknown" || p.OS == "" {
				continue
			}
			s := platformString(p.OS, p.Architecture, p.Variant)
			if !slices.Contains(platforms, s) {
				platforms = append(platforms, s)
			}
		}
		return platforms, true, nil
	case m.SchemaVersion == 1:
		return []string{platformString("linux", m.Architecture, "")}, false, nil
	default:
		return nil, false, nil
	}
}

func platformString(os, arch, variant string) string {
	s := os + "/" + arch
	if variant != "" {
		s += "/" + variant
	}
	return s
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
