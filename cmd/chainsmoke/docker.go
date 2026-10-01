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
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// rateLimitWait is how long to back off after a registry 429 before the
// single retry.
const rateLimitWait = 60 * time.Second

var rateLimitRe = regexp.MustCompile(`(?i)toomanyrequests|too many requests|\b429\b`)

// docker runs the docker CLI and returns stdout, with stderr folded into the
// error.
func docker(ctx context.Context, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("docker %s: %s", args[0], msg)
	}
	return stdout.String(), nil
}

// dockerRetry is docker with one retry after rateLimitWait when the registry
// answers 429.
func dockerRetry(ctx context.Context, args ...string) (string, error) {
	out, err := docker(ctx, args...)
	if err == nil || !rateLimitRe.MatchString(err.Error()) {
		return out, err
	}
	log.Printf("rate limited (%s), retrying in %s", strings.Join(args, " "), rateLimitWait)
	select {
	case <-ctx.Done():
		return out, err
	case <-time.After(rateLimitWait):
	}
	return docker(ctx, args...)
}

// forEach runs fn over items with at most n in flight and returns results in
// input order. Progress goes to the log as each item finishes.
func forEach[T any](ctx context.Context, n int, items []T, fn func(context.Context, T) result) []result {
	out := make([]result, len(items))
	sem := make(chan struct{}, n)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		done int
	)
	for i, item := range items {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			r := fn(ctx, item)
			out[i] = r

			mu.Lock()
			done++
			log.Printf("[%d/%d] %s %s: %s %s",
				done, len(items), r.chain, clientLabel(r.client), r.status, truncate(r.detail, 100))
			mu.Unlock()
		})
	}
	wg.Wait()
	return out
}
