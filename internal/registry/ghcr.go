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
	"regexp"
	"time"

	"github.com/tazhate/chainplane/internal/adapters"
)

const ghcrBase = "https://ghcr.io"

type ghcrClient struct {
	http *http.Client
}

func (c *ghcrClient) httpClient() *http.Client {
	if c.http == nil {
		c.http = &http.Client{Timeout: 15 * time.Second}
	}
	return c.http
}

type ghcrTokenResponse struct {
	Token string `json:"token"`
}

type ghcrTagsResponse struct {
	Tags []string `json:"tags"`
}

func (c *ghcrClient) getToken(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("https://ghcr.io/token?service=ghcr.io&scope=repository:%s/%s:pull", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("get ghcr token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ghcr token returned %d", resp.StatusCode)
	}
	var tr ghcrTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	return tr.Token, nil
}

func (c *ghcrClient) LatestTags(ctx context.Context, policy adapters.ChainVersionPolicy, maxResults int) ([]TagEntry, error) {
	owner, repo := splitRepository(policy.Repository)

	token, err := c.getToken(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/v2/%s/%s/tags/list", ghcrBase, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("ghcr tags request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ghcr returned %d for %s/%s", resp.StatusCode, owner, repo)
	}

	var result ghcrTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode ghcr tags: %w", err)
	}

	pattern, err := regexp.Compile(policy.TagPattern)
	if err != nil {
		return nil, fmt.Errorf("compile tag pattern %q: %w", policy.TagPattern, err)
	}

	entries := make([]TagEntry, 0, maxResults)
	for _, tag := range result.Tags {
		if !pattern.MatchString(tag) {
			continue
		}
		entries = append(entries, TagEntry{Tag: tag})
		if len(entries) >= maxResults {
			break
		}
	}
	return entries, nil
}
