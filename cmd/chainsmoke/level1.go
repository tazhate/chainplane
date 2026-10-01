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
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

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
	env := level1Env{images: newImageSet(opts.keepImages), volumeSubpath: volumeSubpathSupported(ctx)}
	results := forEach(ctx, opts.parallel, jobs, func(ctx context.Context, j smokeJob) result {
		return smokeSample(ctx, opts, env, j)
	})

	for _, chain := range slices.Sorted(maps.Keys(adapters.DefaultImages())) {
		if selected(chain) && perChain[chain] == 0 {
			results = append(results, result{chain: chain, level: 1, status: statusSkip, detail: "no sample in " + sampleGlob})
		}
	}
	return results
}

// level1Env is the state level 1 jobs share.
type level1Env struct {
	images        *imageSet
	volumeSubpath bool
}

// sidecarReadyTimeout bounds the wait for a native sidecar's probe.
const sidecarReadyTimeout = 60 * time.Second

// smokeSample renders the sample, runs its pod for opts.duration and
// classifies the outcome by the main container. Containers and volumes are
// always removed.
func smokeSample(ctx context.Context, opts options, env level1Env, j smokeJob) result {
	r := result{chain: j.sample.chain, level: 1}
	if ctx.Err() != nil {
		r.status, r.detail = statusSkip, detailInterrupted
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

	workDir, err := os.MkdirTemp("", opts.namePrefix+"-"+j.key+"-")
	if err != nil {
		r.status, r.detail = statusFail, err.Error()
		return r
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	name := opts.namePrefix + "-" + j.key
	plan, err := buildRunPlan(planInput{
		name:          name,
		pod:           pod,
		configMapName: node.Name + "-config",
		configFile:    cfgFile,
		configContent: cfgContent,
		workDir:       workDir,
		tmpfsSize:     opts.tmpfsSize,
		nofile:        opts.nofile,
		prefix:        opts.namePrefix,
		volumeSubpath: env.volumeSubpath,
	})
	if err != nil {
		r.status, r.detail = statusFail, err.Error()
		return r
	}
	if plan.image == "" {
		// Image-required chains have no default; level 0 skips them too.
		r.status, r.detail = statusSkip, "no default image (spec.image required)"
		return r
	}
	r.image, r.notes = plan.image, plan.notes
	if err := writePlanFiles(plan.files); err != nil {
		r.status, r.detail = statusFail, err.Error()
		return r
	}

	// Cleanup must survive Ctrl-C, so it runs on a context detached from ctx.
	// Images are released after the containers using them are gone.
	cleanupCtx := context.WithoutCancel(ctx)
	var acquired []string
	defer func() {
		for _, ref := range acquired {
			env.images.release(cleanupCtx, ref)
		}
	}()
	for _, ref := range plan.images() {
		acquired = append(acquired, ref)
		if err := env.images.acquire(ctx, ref); err != nil {
			r.status, r.detail = statusFail, firstLine(err.Error())
			return r
		}
	}

	containers := plan.containers(name)
	removePod := func() {
		_, _ = docker(cleanupCtx, append([]string{"rm", "-f", "-v"}, containers...)...)
		if plan.pod != nil && len(plan.pod.volumes) > 0 {
			_, _ = docker(cleanupCtx, append([]string{"volume", "rm", "-f"}, plan.pod.volumes...)...)
		}
	}
	removePod()
	defer removePod()

	// cmds is every docker command run so far, the header of the log.
	var cmds [][]string
	run := func(args []string) error {
		cmds = append(cmds, args)
		_, err := docker(ctx, args...)
		return err
	}

	if plan.pod != nil {
		if err := startPod(ctx, cleanupCtx, opts.out, j.key, plan.pod, run); err != nil {
			if ctx.Err() != nil {
				r.status, r.detail = statusSkip, detailInterrupted
				return r
			}
			r.status, r.detail = statusFail, err.Error()
			saveLog(opts.out, j.key, cmds, "", err)
			return r
		}
	}

	start := time.Now()
	if err := run(plan.args); err != nil {
		r.status, r.detail = statusFail, runErrorSummary(err.Error())
		saveLog(opts.out, j.key, cmds, "", err)
		return r
	}
	if plan.pod != nil {
		for _, c := range plan.pod.others {
			if err := run(c.args); err != nil {
				r.notes = append(r.notes, "sidecar "+c.container+" not started: "+runErrorSummary(err.Error()))
			}
		}
	}

	waitCtx, cancel := context.WithTimeout(ctx, opts.duration)
	_, _ = docker(waitCtx, "wait", name)
	cancel()
	ranFor := time.Since(start)
	if ctx.Err() != nil {
		r.status, r.detail = statusSkip, detailInterrupted
		return r
	}

	st, err := inspectState(cleanupCtx, name)
	if err != nil {
		r.status, r.detail = statusFail, err.Error()
		return r
	}
	if plan.pod != nil {
		r.notes = append(r.notes, collectSidecars(cleanupCtx, opts.out, j.key, plan.pod)...)
	}

	logs, logErr := dockerLogs(cleanupCtx, name)
	saveLog(opts.out, j.key, cmds, logs, logErr)
	r.status, r.detail = verdict(st, ranFor, scanLog(logs))
	if !st.Running {
		return r
	}

	netOwner := name
	if plan.pod != nil {
		netOwner = plan.pod.holder
	}
	mc := pod.Spec.Containers[slices.IndexFunc(pod.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == controller.MainContainerName
	})]
	found := checkIdentity(ctx, newIdentityTarget(node.Spec, mc, name, netOwner, opts.namePrefix))
	if ctx.Err() != nil {
		r.status, r.detail = statusSkip, detailInterrupted
		return r
	}
	var notes []string
	r.status, r.detail, notes = applyIdentity(r.status, r.detail, found)
	r.notes = append(r.notes, notes...)
	return r
}

// startPod runs the pod setup commands and starts the native sidecars in
// order, each one ready before the next. The log of a sidecar that fails is
// saved right away.
func startPod(ctx, cleanupCtx context.Context, out, key string, p *podPlan, run func([]string) error) error {
	for _, args := range p.setup {
		if err := run(args); err != nil {
			return errors.New("pod setup: " + runErrorSummary(err.Error()))
		}
	}
	for _, c := range p.sidecars {
		err := run(c.args)
		if err == nil {
			err = waitReady(ctx, c)
		}
		if err != nil {
			saveSidecarLog(cleanupCtx, out, key, c)
			return errors.New("sidecar " + c.container + ": " + runErrorSummary(err.Error()))
		}
	}
	return nil
}

// collectSidecars saves the sidecar logs and returns a note for each
// sidecar that is no longer running.
func collectSidecars(ctx context.Context, out, key string, p *podPlan) []string {
	var notes []string
	for _, c := range slices.Concat(p.sidecars, p.others) {
		saveSidecarLog(ctx, out, key, c)
		if st, err := inspectState(ctx, c.name); err == nil && !st.Running {
			notes = append(notes, fmt.Sprintf("sidecar %s exited with code %d", c.container, st.ExitCode))
		}
	}
	return notes
}

// waitReady runs the exec probe of a native sidecar until it succeeds, the
// container exits or sidecarReadyTimeout passes. Kubernetes holds the next
// container back the same way.
func waitReady(ctx context.Context, c containerRun) error {
	if len(c.probe) == 0 {
		return nil
	}
	deadline := time.Now().Add(sidecarReadyTimeout)
	for {
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := docker(probeCtx, append([]string{"exec", c.name}, c.probe...)...)
		cancel()
		switch {
		case err == nil:
			return nil
		case ctx.Err() != nil:
			return ctx.Err()
		}
		if st, err := inspectState(ctx, c.name); err == nil && !st.Running {
			return fmt.Errorf("exited with code %d", st.ExitCode)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("not ready after %s: %s", sidecarReadyTimeout, firstLine(err.Error()))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// inspectState returns the docker state of a container.
func inspectState(ctx context.Context, name string) (containerState, error) {
	var st containerState
	stateJSON, err := docker(ctx, "inspect", "--format", "{{json .State}}", name)
	if err != nil {
		return st, errors.New(firstLine(err.Error()))
	}
	if err := json.Unmarshal([]byte(stateJSON), &st); err != nil {
		return st, fmt.Errorf("parsing container state: %w", err)
	}
	return st, nil
}

// saveSidecarLog writes the output of a sidecar to <out>/<key>.<container>.log.
func saveSidecarLog(ctx context.Context, out, key string, c containerRun) {
	logs, err := dockerLogs(ctx, c.name)
	saveLog(out, key+"."+c.container, [][]string{c.args}, logs, err)
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

// saveLog writes the docker commands that built the container and its
// output to <out>/<key>.log for post-mortem.
func saveLog(out, key string, cmds [][]string, logs string, runErr error) {
	var b strings.Builder
	for _, args := range cmds {
		b.WriteString("# docker " + shellJoin(args) + "\n")
	}
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
