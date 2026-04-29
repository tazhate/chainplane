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
	"fmt"
	"strings"

	"github.com/tazhate/chainplane/internal/adapters"
)

type TagEntry struct {
	Tag         string
	PublishedAt string // RFC3339 or empty
	Digest      string // image digest, may be empty
}

type Client interface {
	LatestTags(ctx context.Context, policy adapters.ChainVersionPolicy, maxResults int) ([]TagEntry, error)
}

func NewClient(reg string) (Client, error) {
	switch reg {
	case "docker.io", "":
		return &dockerHubClient{}, nil
	case "ghcr.io":
		return &ghcrClient{}, nil
	case "us-docker.pkg.dev", "public.ecr.aws":
		return &ociClient{host: reg}, nil
	default:
		return nil, fmt.Errorf("unsupported registry: %s", reg)
	}
}

func splitRepository(repository string) (owner, repo string) {
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "library", parts[0]
}
