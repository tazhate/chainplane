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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// tagsListPageSize is the n= value sent to Docker Distribution tag listings.
// GHCR returns roughly 100 tags per page without it, oldest first, so the
// newest releases of busy repositories sit on later pages.
const tagsListPageSize = 1000

// tagsListMaxPages bounds how many Link-header pages one listing may follow.
const tagsListMaxPages = 20

// fetchTagsList walks a Docker Distribution /v2/<repo>/tags/list endpoint,
// following RFC 5988 Link rel="next" headers until the registry stops sending
// them or tagsListMaxPages is reached. An empty token sends no Authorization.
func fetchTagsList(ctx context.Context, client *http.Client, firstURL, token string) ([]string, error) {
	var tags []string
	next := withPageSize(firstURL)

	for range tagsListMaxPages {
		if next == "" {
			break
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, fmt.Errorf("build tags request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		page, link, err := doTagsListRequest(client, req)
		if err != nil {
			return nil, err
		}
		tags = append(tags, page...)

		next = resolveNextLink(next, link)
	}
	return tags, nil
}

func doTagsListRequest(client *http.Client, req *http.Request) ([]string, string, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("tags request to %s: %w", req.URL.Host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%s returned %d for %s", req.URL.Host, resp.StatusCode, req.URL.Path)
	}

	var result ociTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("decode tags response: %w", err)
	}
	return result.Tags, resp.Header.Get("Link"), nil
}

// withPageSize sets n=tagsListPageSize unless the URL already asks for a
// positive page size. GHCR's own next links carry n=0, which would fall back
// to its small default page.
func withPageSize(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if n := q.Get("n"); n == "" || n == "0" {
		q.Set("n", fmt.Sprint(tagsListPageSize))
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// resolveNextLink extracts the rel="next" target from a Link header and
// resolves it against the current page URL. It returns "" when there is no
// next page.
func resolveNextLink(current, header string) string {
	for part := range strings.SplitSeq(header, ",") {
		target, params, ok := strings.Cut(strings.TrimSpace(part), ";")
		if !ok || !strings.Contains(params, `rel="next"`) {
			continue
		}
		target = strings.Trim(strings.TrimSpace(target), "<>")
		base, err := url.Parse(current)
		if err != nil {
			return ""
		}
		ref, err := url.Parse(target)
		if err != nil {
			return ""
		}
		return withPageSize(base.ResolveReference(ref).String())
	}
	return ""
}

// matchingTags returns every tag that matches pattern, in registry order.
// Callers pick the newest by semver, so nothing is cut here: registries list
// tags oldest-first (GHCR) or in random map order (GAR), and truncating before
// the semver pick used to hide the real latest release.
func matchingTags(tags []string, tagPattern string) ([]TagEntry, error) {
	pattern, err := regexp.Compile(tagPattern)
	if err != nil {
		return nil, fmt.Errorf("compile tag pattern %q: %w", tagPattern, err)
	}
	entries := make([]TagEntry, 0, len(tags))
	for _, tag := range tags {
		if pattern.MatchString(tag) {
			entries = append(entries, TagEntry{Tag: tag})
		}
	}
	return entries, nil
}
