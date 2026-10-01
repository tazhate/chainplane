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
	"fmt"
	"log"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
	"github.com/tazhate/chainplane/internal/controller"
)

// smokeJob is one level 1 run: a sample plus the file/container key that
// identifies it in the output directory.
type smokeJob struct {
	key    string
	sample sample
}

// runLevel1 smoke-runs every selected sample. Chains that have a default
// image but no sample are reported as SKIP so coverage gaps stay visible.
func runLevel1(ctx context.Context, opts options, samples []sample, selected func(chainsv1alpha2.Chain) bool) []result {
	var jobs []smokeJob
	perChain := map[chainsv1alpha2.Chain]int{}
	for _, s := range samples {
		if selected(s.chain) {
			perChain[s.chain]++
		}
	}
	for _, s := range samples {
		if !selected(s.chain) {
			continue
		}
		key := string(s.chain)
		if perChain[s.chain] > 1 {
			key += "-" + strings.TrimSuffix(filepath.Base(s.path), ".yaml")
		}
		jobs = append(jobs, smokeJob{key: key, sample: s})
	}

	if err := os.MkdirAll(opts.out, 0o755); err != nil {
		return []result{{chain: "-", status: statusFail, level: 1, detail: err.Error()}}
	}
	results := forEach(ctx, opts.parallel, jobs, func(ctx context.Context, j smokeJob) result {
		return smokeSample(ctx, opts, j)
	})

	for _, chain := range slices.Sorted(maps.Keys(adapters.DefaultImages())) {
		if selected(chain) && perChain[chain] == 0 {
			results = append(results, result{chain: chain, level: 1, status: statusSkip, detail: "no sample in " + sampleGlob})
		}
	}
	return results
}

// smokeSample renders the sample, runs its main container for opts.duration
// and classifies the outcome. The container is always removed.
func smokeSample(ctx context.Context, opts options, j smokeJob) result {
	r := result{chain: j.sample.chain, level: 1}
	if ctx.Err() != nil {
		r.status, r.detail = statusSkip, "interrupted"
		return r
	}
	if j.sample.err != nil {
		r.status, r.detail = statusFail, "decoding "+filepath.Base(j.sample.path)+": "+j.sample.err.Error()
		return r
	}

	node := j.sample.node.DeepCopy()
	r.client = node.Spec.Client
	adapter, ok := adapters.Get(node.Spec.Chain)
	if !ok {
		r.status, r.detail = statusFail, "no adapter registered"
		return r
	}
	cfgFile, cfgContent, err := adapter.ConfigTemplate(node.Spec)
	if err != nil {
		r.status, r.detail = statusFail, "rendering config: "+err.Error()
		return r
	}
	pod := controller.RenderPodTemplate(node, adapter, "")
	if ref, ok := opts.overrides.lookup(node.Spec.Chain, node.Spec.Client); ok {
		for i := range pod.Spec.Containers {
			if pod.Spec.Containers[i].Name == controller.MainContainerName {
				pod.Spec.Containers[i].Image = ref
			}
		}
	}

	workDir, err := os.MkdirTemp("", "chainsmoke-"+j.key+"-")
	if err != nil {
		r.status, r.detail = statusFail, err.Error()
		return r
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	name := "chainsmoke-" + j.key
	plan, err := buildRunPlan(planInput{
		name:          name,
		pod:           pod,
		configMapName: node.Name + "-config",
		configFile:    cfgFile,
		configContent: cfgContent,
		workDir:       workDir,
		tmpfsSize:     opts.tmpfsSize,
	})
	if err != nil {
		r.status, r.detail = statusFail, err.Error()
		return r
	}
	r.image, r.notes = plan.image, plan.notes
	if err := writePlanFiles(plan.files); err != nil {
		r.status, r.detail = statusFail, err.Error()
		return r
	}

	if _, err := dockerRetry(ctx, "pull", "--quiet", "--platform", smokePlatform, plan.image); err != nil {
		r.status, r.detail = statusFail, firstLine(err.Error())
		return r
	}

	// Cleanup must survive Ctrl-C, so it runs on a context detached from ctx.
	cleanupCtx := context.WithoutCancel(ctx)
	_, _ = docker(cleanupCtx, "rm", "-f", "-v", name)
	defer func() {
		_, _ = docker(cleanupCtx, "rm", "-f", "-v", name)
		if !opts.keepImages {
			// Fails harmlessly while another run still uses the image.
			_, _ = docker(cleanupCtx, "rmi", plan.image)
		}
	}()

	start := time.Now()
	if _, err := docker(ctx, plan.args...); err != nil {
		r.status, r.detail = statusFail, runErrorSummary(err.Error())
		saveLog(opts.out, j.key, plan.args, "", err)
		return r
	}

	waitCtx, cancel := context.WithTimeout(ctx, opts.duration)
	_, _ = docker(waitCtx, "wait", name)
	cancel()
	ranFor := time.Since(start)
	if ctx.Err() != nil {
		r.status, r.detail = statusSkip, "interrupted"
		return r
	}

	stateJSON, err := docker(cleanupCtx, "inspect", "--format", "{{json .State}}", name)
	if err != nil {
		r.status, r.detail = statusFail, firstLine(err.Error())
		return r
	}
	var st containerState
	if err := json.Unmarshal([]byte(stateJSON), &st); err != nil {
		r.status, r.detail = statusFail, "parsing container state: "+err.Error()
		return r
	}

	logs, logErr := dockerLogs(cleanupCtx, name)
	saveLog(opts.out, j.key, plan.args, logs, logErr)
	r.status, r.detail = verdict(st, ranFor, scanLog(logs))
	return r
}

// runErrorSummary keeps the actionable tail of a failed docker run, e.g.
// `exec: "dashd": executable file not found in $PATH`, instead of the
// daemon/runc preamble that would fill the report column.
func runErrorSummary(msg string) string {
	msg = firstLine(msg)
	if _, tail, ok := strings.Cut(msg, "error during container init: "); ok {
		return "container init: " + tail
	}
	if _, tail, ok := strings.Cut(msg, "Error response from daemon: "); ok {
		return tail
	}
	return msg
}

// writePlanFiles writes the bind-mounted config files. Directories and files
// are world-readable so non-root node images can read them.
func writePlanFiles(files map[string]string) error {
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// dockerLogs returns stdout and stderr of the container interleaved.
func dockerLogs(ctx context.Context, name string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", "logs", name).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker logs: %w", err)
	}
	return string(out), nil
}

// saveLog writes the docker run command and container output to
// <out>/<key>.log for post-mortem.
func saveLog(out, key string, args []string, logs string, runErr error) {
	var b strings.Builder
	b.WriteString("# docker " + shellJoin(args) + "\n")
	if runErr != nil {
		b.WriteString("# error: " + runErr.Error() + "\n")
	}
	b.WriteString(logs)
	path := filepath.Join(out, key+".log")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		log.Printf("writing %s: %v", path, err)
	}
}

// shellJoin quotes args for copy-pasting the command from a log header.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		if a != "" && !strings.ContainsAny(a, " \t\n'\"$`\\|&;()<>*?[]{}!#~") {
			quoted[i] = a
			continue
		}
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " ")
}
