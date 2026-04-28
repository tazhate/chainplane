package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tazhate/chainplane/internal/adapters"
)

// ociClient speaks the Docker Distribution / OCI v2 tag-listing protocol.
// It handles any public registry that issues anonymous Bearer tokens, including
// Google Artifact Registry (us-docker.pkg.dev) and Amazon ECR Public (public.ecr.aws).
type ociClient struct {
	host string
	http *http.Client
}

func (c *ociClient) httpClient() *http.Client {
	if c.http == nil {
		c.http = &http.Client{Timeout: 15 * time.Second}
	}
	return c.http
}

type ociTokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"` // ECR Public uses this field
}

// getToken obtains an anonymous pull token via the standard OAuth2 scope URL.
// Both GAR and ECR Public follow the same www-authenticate/token pattern.
func (c *ociClient) getToken(ctx context.Context, repo string) (string, error) {
	tokenURL := fmt.Sprintf("https://%s/token?scope=repository:%s:pull&service=%s",
		c.host, url.QueryEscape(repo), c.host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch token from %s: %w", c.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint %s returned %d", c.host, resp.StatusCode)
	}

	var tr ociTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tr.Token != "" {
		return tr.Token, nil
	}
	return tr.AccessToken, nil
}

type ociTagsResponse struct {
	Tags []string `json:"tags"`
	// next page link is in the Link response header — ignored for now since
	// we request enough tags in one shot via ?n=
}

func (c *ociClient) LatestTags(ctx context.Context, policy adapters.ChainVersionPolicy, maxResults int) ([]TagEntry, error) {
	token, err := c.getToken(ctx, policy.Repository)
	if err != nil {
		return nil, err
	}

	// Request more than maxResults so we have room to filter by pattern.
	fetchN := maxResults * 10
	if fetchN < 100 {
		fetchN = 100
	}

	// Normalize repo path: strip leading "us-docker.pkg.dev/" or host prefix
	// if the caller accidentally included it.
	repo := policy.Repository
	if strings.HasPrefix(repo, c.host+"/") {
		repo = strings.TrimPrefix(repo, c.host+"/")
	}

	tagsURL := fmt.Sprintf("https://%s/v2/%s/tags/list?n=%d", c.host, repo, fetchN)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build tags request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("tags request to %s: %w", c.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d for %s", c.host, resp.StatusCode, repo)
	}

	var result ociTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode tags response: %w", err)
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
