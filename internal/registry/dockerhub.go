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

const dockerHubAPIBase = "https://hub.docker.com/v2/repositories"

type dockerHubClient struct {
	http *http.Client
}

func (c *dockerHubClient) httpClient() *http.Client {
	if c.http == nil {
		c.http = &http.Client{Timeout: 15 * time.Second}
	}
	return c.http
}

type dockerHubTagsResponse struct {
	Results []dockerHubTag `json:"results"`
}

type dockerHubTag struct {
	Name          string `json:"name"`
	TagLastPushed string `json:"tag_last_pushed"`
}

func (c *dockerHubClient) LatestTags(ctx context.Context, policy adapters.ChainVersionPolicy, maxResults int) ([]TagEntry, error) {
	owner, repo := splitRepository(policy.Repository)
	url := fmt.Sprintf("%s/%s/%s/tags?page_size=50&ordering=last_updated", dockerHubAPIBase, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker hub request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker hub returned %d for %s/%s", resp.StatusCode, owner, repo)
	}

	var result dockerHubTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	pattern, err := regexp.Compile(policy.TagPattern)
	if err != nil {
		return nil, fmt.Errorf("compile tag pattern %q: %w", policy.TagPattern, err)
	}

	entries := make([]TagEntry, 0, maxResults)
	for _, t := range result.Results {
		if !pattern.MatchString(t.Name) {
			continue
		}
		entries = append(entries, TagEntry{
			Tag:         t.Name,
			PublishedAt: t.TagLastPushed,
		})
		if len(entries) >= maxResults {
			break
		}
	}
	return entries, nil
}
