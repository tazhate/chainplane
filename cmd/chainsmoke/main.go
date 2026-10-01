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
// chainsmoke smoke-tests chain node images on a workstation without syncing.
//
// Level 0 checks that every default image in internal/adapters/versions_gen.go
// exists in its registry and ships a linux/amd64 variant. Level 1 renders the
// pod template of every config/samples ChainInstance exactly as the operator
// would, starts the main node container under docker for a short while and
// scans its logs for flag and config errors.
package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

// options holds the parsed command line.
type options struct {
	chains       *regexp.Regexp
	level        int
	parallel     int
	duration     time.Duration
	overrides    overrideList
	changedSince string
	out          string
	samples      string
	keepImages   bool
	tmpfsSize    string
	repoRoot     string
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("chainsmoke: ")

	opts, err := parseFlags(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	results, err := run(ctx, opts)
	if err != nil {
		log.Fatal(err)
	}

	report := renderReport(results)
	fmt.Print(report)
	if err := os.MkdirAll(opts.out, 0o755); err != nil {
		log.Fatal(err)
	}
	reportPath := filepath.Join(opts.out, "smoke-report.md")
	if err := os.WriteFile(reportPath, []byte(report), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("report written to %s", reportPath)

	if anyFailed(results) {
		os.Exit(1)
	}
}

func parseFlags(args []string, stderr io.Writer) (options, error) {
	var (
		opts     options
		chainsRe string
	)
	fs := flag.NewFlagSet("chainsmoke", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&chainsRe, "chains", "", "regexp selecting chains by name (default: all)")
	fs.IntVar(&opts.level, "level", 0, "0 = registry manifest check, 1 = run the node container under docker")
	fs.IntVar(&opts.parallel, "parallel", 0, "concurrent checks (default 6 for level 0, 3 for level 1)")
	fs.DurationVar(&opts.duration, "duration", 90*time.Second, "level 1: how long the container must stay up")
	fs.Var(&opts.overrides, "image", "override image as chain[/client]=ref (repeatable)")
	fs.StringVar(&opts.changedSince, "changed-since", "",
		"only chains whose default image changed in versions_gen.go since this git ref")
	fs.StringVar(&opts.out, "out", "smoke-out", "directory for logs and smoke-report.md")
	fs.StringVar(&opts.samples, "samples", "", "ChainInstance samples directory (default <repo>/config/samples)")
	fs.BoolVar(&opts.keepImages, "keep-images", false,
		"level 1: keep pulled images instead of removing them after each run")
	fs.StringVar(&opts.tmpfsSize, "tmpfs-size", "4g", "level 1: size cap of the tmpfs that stands in for the data PVC")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() > 0 {
		return opts, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	re, err := regexp.Compile(chainsRe)
	if err != nil {
		return opts, fmt.Errorf("--chains: %w", err)
	}
	opts.chains = re

	switch opts.level {
	case 0:
		opts.parallel = cmp.Or(opts.parallel, 6)
	case 1:
		opts.parallel = cmp.Or(opts.parallel, 3)
	default:
		return opts, fmt.Errorf("--level must be 0 or 1, got %d", opts.level)
	}
	if opts.parallel < 1 {
		return opts, fmt.Errorf("--parallel must be positive")
	}
	if opts.duration <= 0 {
		return opts, fmt.Errorf("--duration must be positive")
	}

	opts.repoRoot = repoRoot()
	if opts.samples == "" {
		opts.samples = filepath.Join(opts.repoRoot, "config", "samples")
	}
	return opts, nil
}

// repoRoot returns the git top-level directory, or the working directory when
// git is unavailable.
func repoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	wd, _ := os.Getwd()
	return wd
}

// run selects targets and executes the requested level.
func run(ctx context.Context, opts options) ([]result, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker CLI not found: %w", err)
	}

	selected := func(chain chainsv1alpha2.Chain) bool { return opts.chains.MatchString(string(chain)) }
	if opts.changedSince != "" {
		changed, err := changedSince(opts.repoRoot, opts.changedSince, adapters.DefaultImages())
		if err != nil {
			return nil, err
		}
		log.Printf("%d chain(s) changed since %s", len(changed), opts.changedSince)
		bySelector := selected
		selected = func(chain chainsv1alpha2.Chain) bool { return changed[chain] && bySelector(chain) }
	}

	switch opts.level {
	case 0:
		targets := level0Targets(adapters.DefaultImages(), opts.overrides, selected)
		log.Printf("level 0: %d image(s), parallel %d", len(targets), opts.parallel)
		return runLevel0(ctx, targets, opts.parallel), nil
	default:
		samples, err := loadSamples(opts.samples)
		if err != nil {
			return nil, err
		}
		log.Printf("level 1: %d sample(s) available, duration %s, parallel %d", len(samples), opts.duration, opts.parallel)
		return runLevel1(ctx, opts, samples, selected), nil
	}
}
