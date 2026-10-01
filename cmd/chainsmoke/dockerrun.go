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
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/tazhate/chainplane/internal/controller"
)

// smokePlatform is the platform every level 1 container runs as, matching the
// production node pools.
const smokePlatform = "linux/amd64"

const (
	// placeholderValue stands in for Secret and ConfigMap backed env vars.
	placeholderValue = "smoke"
	// smokeNamespace is the namespace samples without one are rendered in.
	smokeNamespace = "default"
)

// planInput is everything needed to turn a rendered pod template into a
// docker run invocation.
type planInput struct {
	name          string // docker container name
	pod           corev1.PodTemplateSpec
	configMapName string // name of the operator-rendered config ConfigMap
	configFile    string // key inside that ConfigMap (adapter.ConfigTemplate filename)
	configContent string
	workDir       string // host directory for the config bind mount
	tmpfsSize     string
}

// runPlan is a docker run invocation plus the host files it expects.
type runPlan struct {
	image string
	args  []string          // arguments after "docker"
	files map[string]string // host path -> content, written before docker run
	notes []string          // parts of the pod that the plan could not reproduce
}

// buildRunPlan converts the main container of a rendered pod into
// `docker run -d`. Kubernetes semantics are kept where they matter for flag
// and config errors: command/args map to entrypoint/cmd, $(VAR) references
// are expanded, the config ConfigMap is bind-mounted read-only and every
// other volume (the data PVC included) becomes a world-writable tmpfs.
// Init containers and sidecars are not run; they are listed in notes.
func buildRunPlan(in planInput) (runPlan, error) {
	spec := in.pod.Spec
	idx := slices.IndexFunc(spec.Containers, func(c corev1.Container) bool {
		return c.Name == controller.MainContainerName
	})
	if idx < 0 {
		return runPlan{}, fmt.Errorf("pod template has no %q container", controller.MainContainerName)
	}
	mc := spec.Containers[idx]

	plan := runPlan{image: mc.Image}
	if len(spec.InitContainers) > 0 {
		plan.notes = append(plan.notes, "init containers skipped: "+containerNames(spec.InitContainers))
	}
	if len(spec.Containers) > 1 {
		others := slices.Delete(slices.Clone(spec.Containers), idx, idx+1)
		plan.notes = append(plan.notes, "sidecars skipped: "+containerNames(others))
	}
	if len(mc.EnvFrom) > 0 {
		plan.notes = append(plan.notes, "envFrom ignored")
	}

	env, lookup := resolveEnv(mc, in.name)
	command := expandAll(mc.Command, lookup)
	cmdArgs := expandAll(mc.Args, lookup)

	args := []string{"run", "-d", "--name", in.name, "--platform", smokePlatform, "--label", "chainsmoke=1"}
	if sc := spec.SecurityContext; sc != nil && sc.FSGroup != nil {
		args = append(args, "--group-add", strconv.FormatInt(*sc.FSGroup, 10))
	}
	if user := runAsUser(spec.SecurityContext, mc.SecurityContext); user != "" {
		args = append(args, "--user", user)
	}
	if mc.WorkingDir != "" {
		args = append(args, "--workdir", mc.WorkingDir)
	}
	for _, kv := range env {
		args = append(args, "--env", kv)
	}

	mountArgs, files, notes := planMounts(in, mc.VolumeMounts)
	args = append(args, mountArgs...)
	plan.files = files
	plan.notes = append(plan.notes, notes...)

	if len(command) > 0 {
		args = append(args, "--entrypoint", command[0])
		cmdArgs = append(slices.Clone(command[1:]), cmdArgs...)
	}
	args = append(args, mc.Image)
	args = append(args, cmdArgs...)
	plan.args = args
	return plan, nil
}

// planMounts maps volume mounts to docker mount flags.
func planMounts(in planInput, mounts []corev1.VolumeMount) (args []string, files map[string]string, notes []string) {
	files = map[string]string{}
	volumes := map[string]corev1.Volume{}
	for _, v := range in.pod.Spec.Volumes {
		volumes[v.Name] = v
	}
	configDir := filepath.Join(in.workDir, "config")
	seen := map[string]bool{}

	for _, m := range mounts {
		if seen[m.MountPath] {
			notes = append(notes, fmt.Sprintf("duplicate mount %s skipped", m.MountPath))
			continue
		}
		seen[m.MountPath] = true

		vol, declared := volumes[m.Name]
		switch {
		case declared && vol.ConfigMap != nil && vol.ConfigMap.Name == in.configMapName:
			src := configDir
			files[filepath.Join(configDir, in.configFile)] = in.configContent
			if m.SubPath != "" {
				src = filepath.Join(configDir, m.SubPath)
				if m.SubPath != in.configFile {
					files[src] = ""
					notes = append(notes, fmt.Sprintf("config subPath %s is not %s, mounted empty", m.SubPath, in.configFile))
				}
			}
			args = append(args, "--mount", "type=bind,src="+src+",dst="+m.MountPath+",readonly")
		default:
			// The data PVC (from volumeClaimTemplates, so not in Volumes),
			// emptyDirs, hostPaths and foreign ConfigMaps/Secrets all become
			// a scratch tmpfs. mode=1777 stands in for fsGroup ownership.
			if declared && (vol.ConfigMap != nil || vol.Secret != nil || vol.Projected != nil) {
				notes = append(notes, fmt.Sprintf("volume %s mounted empty at %s", m.Name, m.MountPath))
			}
			args = append(args, "--tmpfs", m.MountPath+":rw,exec,mode=1777,size="+in.tmpfsSize)
		}
	}
	return args, files, notes
}

// resolveEnv returns the container env as KEY=VALUE pairs in declaration
// order, with Kubernetes $(VAR) expansion against earlier variables, plus a
// lookup over the final values for command/args expansion. Secret and
// ConfigMap references resolve to placeholderValue.
func resolveEnv(c corev1.Container, podName string) ([]string, func(string) (string, bool)) {
	values := map[string]string{}
	var order []string
	lookup := func(name string) (string, bool) {
		v, ok := values[name]
		return v, ok
	}
	for _, e := range c.Env {
		var v string
		if e.ValueFrom != nil {
			v = envSourceValue(e.ValueFrom, c, podName)
		} else {
			v = expand(e.Value, lookup)
		}
		if _, ok := values[e.Name]; !ok {
			order = append(order, e.Name)
		}
		values[e.Name] = v
	}
	out := make([]string, 0, len(order))
	for _, name := range order {
		out = append(out, name+"="+values[name])
	}
	return out, lookup
}

// envSourceValue fakes a valueFrom source the way a pod would see it on a
// single-node cluster.
func envSourceValue(src *corev1.EnvVarSource, c corev1.Container, podName string) string {
	switch {
	case src.FieldRef != nil:
		switch src.FieldRef.FieldPath {
		case "metadata.name":
			return podName
		case "metadata.namespace":
			return smokeNamespace
		case "status.podIP", "status.hostIP":
			return "127.0.0.1"
		}
	case src.ResourceFieldRef != nil:
		return resourceFieldValue(src.ResourceFieldRef, c)
	}
	return placeholderValue
}

// resourceFieldValue mirrors the downward API for limits.* and requests.*.
func resourceFieldValue(ref *corev1.ResourceFieldSelector, c corev1.Container) string {
	kind, name, _ := strings.Cut(ref.Resource, ".")
	list := c.Resources.Limits
	if kind == "requests" || list == nil {
		list = c.Resources.Requests
	}
	q, ok := list[corev1.ResourceName(name)]
	if !ok {
		return "1"
	}
	divisor := ref.Divisor
	if divisor.IsZero() {
		divisor = resource.MustParse("1")
	}
	// Ceiling division, as the kubelet does.
	n := (q.MilliValue() + divisor.MilliValue() - 1) / divisor.MilliValue()
	return strconv.FormatInt(n, 10)
}

// expand implements Kubernetes $(VAR) expansion: $(NAME) is replaced when
// NAME resolves, left verbatim otherwise; $$ escapes a literal $.
func expand(s string, lookup func(string) (string, bool)) string {
	if !strings.Contains(s, "$") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '$' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		switch s[i+1] {
		case '$':
			b.WriteByte('$')
			i++
		case '(':
			end := strings.IndexByte(s[i+2:], ')')
			if end < 0 {
				b.WriteString(s[i:])
				return b.String()
			}
			name := s[i+2 : i+2+end]
			if v, ok := lookup(name); ok {
				b.WriteString(v)
			} else {
				b.WriteString("$(" + name + ")")
			}
			i += 2 + end
		default:
			b.WriteByte('$')
		}
	}
	return b.String()
}

func expandAll(in []string, lookup func(string) (string, bool)) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = expand(s, lookup)
	}
	return out
}

// runAsUser renders --user from the pod and container security contexts,
// container fields taking precedence.
func runAsUser(pod *corev1.PodSecurityContext, c *corev1.SecurityContext) string {
	var uid, gid *int64
	if pod != nil {
		uid, gid = pod.RunAsUser, pod.RunAsGroup
	}
	if c != nil {
		if c.RunAsUser != nil {
			uid = c.RunAsUser
		}
		if c.RunAsGroup != nil {
			gid = c.RunAsGroup
		}
	}
	if uid == nil {
		return ""
	}
	user := strconv.FormatInt(*uid, 10)
	if gid != nil {
		user += ":" + strconv.FormatInt(*gid, 10)
	}
	return user
}

func containerNames(cs []corev1.Container) string {
	names := make([]string, 0, len(cs))
	for _, c := range cs {
		names = append(names, c.Name)
	}
	return strings.Join(names, ", ")
}
