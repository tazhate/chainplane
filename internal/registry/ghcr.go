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

// LatestTags returns every tag matching policy.TagPattern. maxResults is
// ignored: the full list is needed to pick the semver maximum.
func (c *ghcrClient) LatestTags(ctx context.Context, policy adapters.ChainVersionPolicy, _ int) ([]TagEntry, error) {
	owner, repo := splitRepository(policy.Repository)

	token, err := c.getToken(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	tags, err := fetchTagsList(ctx, c.httpClient(), fmt.Sprintf("%s/v2/%s/%s/tags/list", ghcrBase, owner, repo), token)
	if err != nil {
		return nil, err
	}
	return matchingTags(tags, policy.TagPattern)
}
