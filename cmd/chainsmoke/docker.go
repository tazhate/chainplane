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
	"strconv"
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

// imageSet tracks the images a level 1 run pulls. An image is removed once
// its last concurrent user is done, and never when it was on the host before
// the run first needed it, so images of other runs and of the user survive.
type imageSet struct {
	keep bool // --keep-images

	mu          sync.Mutex
	users       map[string]int
	preexisting map[string]bool
}

func newImageSet(keep bool) *imageSet {
	return &imageSet{keep: keep, users: map[string]int{}, preexisting: map[string]bool{}}
}

// acquire registers a user of ref and pulls it. Every acquire needs a
// release, also when the pull fails.
func (s *imageSet) acquire(ctx context.Context, ref string) error {
	s.mu.Lock()
	if _, seen := s.preexisting[ref]; !seen {
		_, err := docker(ctx, "image", "inspect", "--format", "{{.Id}}", ref)
		s.preexisting[ref] = err == nil
	}
	s.users[ref]++
	s.mu.Unlock()

	_, err := dockerRetry(ctx, "pull", "--quiet", "--platform", smokePlatform, ref)
	return err
}

// release drops a user of ref and removes the image when it was the last
// one. Removal runs under the lock so a concurrent acquire cannot lose its
// freshly pulled image; it fails harmlessly while a container of another
// process still uses the image.
func (s *imageSet) release(ctx context.Context, ref string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[ref]--
	if s.users[ref] > 0 || s.keep || s.preexisting[ref] {
		return
	}
	_, _ = docker(ctx, "rmi", ref)
}

// volumeSubpathSupported reports whether the docker engine accepts
// --mount ...,volume-subpath=, added in Docker Engine 26.
func volumeSubpathSupported(ctx context.Context) bool {
	out, err := docker(ctx, "version", "--format", "{{.Server.Version}}")
	if err != nil {
		log.Printf("docker engine version unknown, not using volume-subpath: %v", err)
		return false
	}
	return engineAtLeast(strings.TrimSpace(out), 26)
}

// engineAtLeast compares the major number of a docker engine version such
// as "29.8.1" or "26.0.0-rc1".
func engineAtLeast(version string, major int) bool {
	head, _, _ := strings.Cut(version, ".")
	n, err := strconv.Atoi(head)
	return err == nil && n >= major
}
