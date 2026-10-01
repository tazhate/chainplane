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
	"path"
	"path/filepath"
	"regexp"
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

// podHolderImage keeps the volumes of a multi-container pod mounted and owns
// its network namespace, the role of the pause container in Kubernetes.
// busybox rather than pause, because the holder also creates subPath
// directories, and docker refuses a volume-subpath that does not exist yet.
const podHolderImage = "busybox:1.36"

// podVolumesDir is where the holder mounts every pod volume.
const podVolumesDir = "/volumes"

// planInput is everything needed to turn a rendered pod template into
// docker run invocations.
type planInput struct {
	name          string // main container name; other pod containers and volumes derive from it
	pod           corev1.PodTemplateSpec
	configMapName string // name of the operator-rendered config ConfigMap
	configFile    string // key inside that ConfigMap (adapter.ConfigTemplate filename)
	configContent string
	workDir       string // host directory for the config bind mount
	tmpfsSize     string
	nofile        int    // nofile soft and hard limit, 0 keeps the docker default
	prefix        string // value of the chainsmoke.prefix label
	volumeSubpath bool   // the engine supports --mount volume-subpath (Docker 26+)
}

// runPlan is the docker run invocation of the main container plus the host
// files it expects and, for pods with sidecars, the rest of the pod.
type runPlan struct {
	image string
	args  []string          // arguments after "docker"
	files map[string]string // host path -> content, written before docker run
	notes []string          // parts of the pod that the plan could not reproduce
	pod   *podPlan          // nil when the main container runs alone
}

// podPlan is the docker state a pod with sidecars needs around the main
// container. Every container joins the network namespace of the holder, so
// localhost is shared the way it is in a pod.
type podPlan struct {
	holder   string         // holder container name
	volumes  []string       // tmpfs-backed docker volumes standing in for pod volumes
	setup    [][]string     // volume creation, holder start, subPath mkdir; run first, in order
	sidecars []containerRun // native sidecars, started in order before the main container
	others   []containerRun // regular sidecars, started after the main container
}

// containerRun is one pod container other than the main one.
type containerRun struct {
	container string // name in the pod spec
	name      string // docker container name
	image     string
	args      []string // arguments after "docker"
	probe     []string // exec probe to wait for before starting the next container
}

// images returns every image the plan runs, main image first.
func (p runPlan) images() []string {
	refs := []string{p.image}
	if p.pod == nil {
		return refs
	}
	for _, c := range slices.Concat(p.pod.sidecars, p.pod.others) {
		if !slices.Contains(refs, c.image) {
			refs = append(refs, c.image)
		}
	}
	if !slices.Contains(refs, podHolderImage) {
		refs = append(refs, podHolderImage)
	}
	return refs
}

// containers returns the docker container names of the plan in the order
// they should be removed: regular sidecars, main, native sidecars in reverse,
// holder last.
func (p runPlan) containers(main string) []string {
	if p.pod == nil {
		return []string{main}
	}
	var names []string
	for _, c := range p.pod.others {
		names = append(names, c.name)
	}
	names = append(names, main)
	for _, c := range slices.Backward(p.pod.sidecars) {
		names = append(names, c.name)
	}
	return append(names, p.pod.holder)
}

// buildRunPlan converts a rendered pod into docker run invocations.
// Kubernetes semantics are kept where they matter for flag and config
// errors: command/args map to entrypoint/cmd, $(VAR) references are
// expanded, the config ConfigMap is bind-mounted read-only and every other
// volume (the data PVC included) becomes a world-writable tmpfs.
//
// A pod with only the main container runs as a single container with a
// tmpfs per mount. A pod with sidecars gets a holder container, a shared
// tmpfs-backed docker volume per pod volume, native sidecars (init
// containers with restartPolicy Always) started before the main container
// and regular sidecars after it. Ordinary init containers are not run; they
// are listed in notes.
func buildRunPlan(in planInput) (runPlan, error) {
	spec := in.pod.Spec
	idx := slices.IndexFunc(spec.Containers, func(c corev1.Container) bool {
		return c.Name == controller.MainContainerName
	})
	if idx < 0 {
		return runPlan{}, fmt.Errorf("pod template has no %q container", controller.MainContainerName)
	}
	mc := spec.Containers[idx]
	others := slices.Delete(slices.Clone(spec.Containers), idx, idx+1)

	var native, inits []corev1.Container
	for _, c := range spec.InitContainers {
		if c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			native = append(native, c)
		} else {
			inits = append(inits, c)
		}
	}

	plan := runPlan{image: mc.Image}
	if len(inits) > 0 {
		plan.notes = append(plan.notes, "init containers skipped: "+containerNames(inits))
	}

	mp := newMountPlanner(in, len(native)+len(others) > 0)
	var network string
	if mp.pod {
		plan.pod = &podPlan{holder: in.name + "-pod"}
		network = "container:" + plan.pod.holder
	}

	sidecar := func(c corev1.Container) (containerRun, bool) {
		if c.Image == "" {
			plan.notes = append(plan.notes, "sidecar "+c.Name+" skipped: no image")
			return containerRun{}, false
		}
		name := in.name + "-" + c.Name
		if len(c.EnvFrom) > 0 {
			plan.notes = append(plan.notes, "envFrom ignored in "+c.Name)
		}
		return containerRun{
			container: c.Name,
			name:      name,
			image:     c.Image,
			args:      containerRunArgs(in, name, c, network, mp.mounts(c.VolumeMounts)),
		}, true
	}

	for _, c := range native {
		run, ok := sidecar(c)
		if !ok {
			continue
		}
		probe, isExec := execProbe(c)
		if !isExec {
			plan.notes = append(plan.notes, "sidecar "+c.Name+" probe is not exec, not waited for")
		}
		run.probe = probe
		plan.pod.sidecars = append(plan.pod.sidecars, run)
	}

	if len(mc.EnvFrom) > 0 {
		plan.notes = append(plan.notes, "envFrom ignored")
	}
	plan.args = containerRunArgs(in, in.name, mc, network, mp.mounts(mc.VolumeMounts))

	for _, c := range others {
		if run, ok := sidecar(c); ok {
			plan.pod.others = append(plan.pod.others, run)
		}
	}

	plan.files = mp.files
	plan.notes = append(plan.notes, mp.notes...)
	if mp.pod {
		plan.pod.volumes = mp.volumes
		plan.pod.setup = podSetup(in, plan.pod.holder, mp.volumes, mp.subdirs)
	}
	return plan, nil
}

// containerRunArgs renders `docker run -d` for one container of the pod.
// network is empty for the default bridge or container:<holder> to join the
// pod network namespace.
func containerRunArgs(in planInput, name string, c corev1.Container, network string, mountArgs []string) []string {
	spec := in.pod.Spec
	env, lookup := resolveEnv(c, in.name)
	command := expandAll(c.Command, lookup)
	cmdArgs := expandAll(c.Args, lookup)

	args := append([]string{"run", "-d", "--name", name, "--platform", smokePlatform}, labelArgs(in.prefix)...)
	if network != "" {
		args = append(args, "--network", network)
	}
	if in.nofile > 0 {
		args = append(args, "--ulimit", fmt.Sprintf("nofile=%d:%d", in.nofile, in.nofile))
	}
	if sc := spec.SecurityContext; sc != nil && sc.FSGroup != nil {
		args = append(args, "--group-add", strconv.FormatInt(*sc.FSGroup, 10))
	}
	if user := runAsUser(spec.SecurityContext, c.SecurityContext); user != "" {
		args = append(args, "--user", user)
	}
	if c.WorkingDir != "" {
		args = append(args, "--workdir", c.WorkingDir)
	}
	for _, kv := range env {
		args = append(args, "--env", kv)
	}
	args = append(args, mountArgs...)

	if len(command) > 0 {
		args = append(args, "--entrypoint", command[0])
		cmdArgs = append(slices.Clone(command[1:]), cmdArgs...)
	}
	args = append(args, c.Image)
	return append(args, cmdArgs...)
}

// labelArgs marks everything chainsmoke creates so leftovers of a hard kill
// can be found, per run prefix or all at once.
func labelArgs(prefix string) []string {
	args := []string{"--label", "chainsmoke=1"}
	if prefix != "" {
		args = append(args, "--label", "chainsmoke.prefix="+prefix)
	}
	return args
}

// podSetup returns the docker commands that prepare a pod before its first
// container: tmpfs-backed volumes, the holder that keeps them mounted (a
// tmpfs volume is unmounted, and emptied, once no container uses it) and
// the subPath directories, world-writable as with tmpfs mode=1777.
func podSetup(in planInput, holder string, volumes, subdirs []string) [][]string {
	setup := make([][]string, 0, len(volumes)+2)
	for _, v := range volumes {
		setup = append(setup, slices.Concat(
			[]string{"volume", "create", "--driver", "local",
				"--opt", "type=tmpfs", "--opt", "device=tmpfs", "--opt", "o=size=" + in.tmpfsSize + ",mode=1777"},
			labelArgs(in.prefix),
			[]string{v},
		))
	}
	run := append([]string{"run", "-d", "--name", holder, "--platform", smokePlatform}, labelArgs(in.prefix)...)
	for _, v := range volumes {
		run = append(run, "--mount", "type=volume,src="+v+",dst="+path.Join(podVolumesDir, v))
	}
	setup = append(setup, append(run, podHolderImage, "sleep", "2147483647"))
	if len(subdirs) > 0 {
		setup = append(setup, slices.Concat([]string{"exec", holder, "mkdir", "-p", "-m", "1777"}, subdirs))
	}
	return setup
}

// execProbe returns the command of the probe Kubernetes gates the next
// container on: the startup probe, else the readiness probe. isExec is false
// when that probe is not an exec probe and cannot be run with docker exec.
func execProbe(c corev1.Container) (cmd []string, isExec bool) {
	for _, p := range []*corev1.Probe{c.StartupProbe, c.ReadinessProbe} {
		if p == nil {
			continue
		}
		if p.Exec == nil {
			return nil, false
		}
		return p.Exec.Command, true
	}
	return nil, true
}

// mountPlanner maps volume mounts to docker mount flags. In pod mode every
// non-config volume is a docker volume shared by the containers that mount
// it; otherwise each mount is a private tmpfs.
type mountPlanner struct {
	in      planInput
	pod     bool
	decl    map[string]corev1.Volume
	files   map[string]string
	notes   []string
	volumes []string // docker volumes, in first-use order
	subdirs []string // holder paths backing volume-subpath mounts
}

func newMountPlanner(in planInput, pod bool) *mountPlanner {
	p := &mountPlanner{in: in, pod: pod, decl: map[string]corev1.Volume{}, files: map[string]string{}}
	for _, v := range in.pod.Spec.Volumes {
		p.decl[v.Name] = v
	}
	return p
}

func (p *mountPlanner) note(n string) {
	if !slices.Contains(p.notes, n) {
		p.notes = append(p.notes, n)
	}
}

// mounts returns the mount flags of one container.
func (p *mountPlanner) mounts(mounts []corev1.VolumeMount) []string {
	var args []string
	configDir := filepath.Join(p.in.workDir, "config")
	seen := map[string]bool{}

	for _, m := range mounts {
		if seen[m.MountPath] {
			p.note(fmt.Sprintf("duplicate mount %s skipped", m.MountPath))
			continue
		}
		seen[m.MountPath] = true

		vol, declared := p.decl[m.Name]
		if declared && vol.ConfigMap != nil && vol.ConfigMap.Name == p.in.configMapName {
			src := configDir
			p.files[filepath.Join(configDir, p.in.configFile)] = p.in.configContent
			if m.SubPath != "" {
				src = filepath.Join(configDir, m.SubPath)
				if m.SubPath != p.in.configFile {
					p.files[src] = ""
					p.note(fmt.Sprintf("config subPath %s is not %s, mounted empty", m.SubPath, p.in.configFile))
				}
			}
			args = append(args, "--mount", "type=bind,src="+src+",dst="+m.MountPath+",readonly")
			continue
		}

		// The data PVC (from volumeClaimTemplates, so not in Volumes),
		// emptyDirs, hostPaths and foreign ConfigMaps/Secrets all become
		// scratch space. mode=1777 stands in for fsGroup ownership.
		if declared && (vol.ConfigMap != nil || vol.Secret != nil || vol.Projected != nil) {
			p.note(fmt.Sprintf("volume %s mounted empty at %s", m.Name, m.MountPath))
		}
		if p.pod {
			args = append(args, "--mount", p.volumeMount(m))
		} else {
			args = append(args, "--tmpfs", m.MountPath+":rw,exec,mode=1777,size="+p.in.tmpfsSize)
		}
	}
	return args
}

// volumeMount renders a pod volume mount. A subPath uses volume-subpath when
// the engine has it. Older engines get a separate volume per subPath: the
// container sees the same thing, but a mount of the whole volume does not
// show the subdirectory.
func (p *mountPlanner) volumeMount(m corev1.VolumeMount) string {
	vol := p.in.name + "-" + m.Name
	var subpath string
	switch {
	case m.SubPath == "":
	case p.in.volumeSubpath:
		subpath = ",volume-subpath=" + m.SubPath
		if dir := path.Join(podVolumesDir, vol, m.SubPath); !slices.Contains(p.subdirs, dir) {
			p.subdirs = append(p.subdirs, dir)
		}
	default:
		vol += "-" + volumeNameRe.ReplaceAllString(m.SubPath, "-")
		p.note(fmt.Sprintf("subPath %s of %s is a separate volume (no volume-subpath before Docker 26)", m.SubPath, m.Name))
	}
	if !slices.Contains(p.volumes, vol) {
		p.volumes = append(p.volumes, vol)
	}
	spec := "type=volume,src=" + vol + ",dst=" + m.MountPath + subpath
	if m.ReadOnly {
		spec += ",readonly"
	}
	return spec
}

// volumeNameRe matches characters docker does not allow in volume names.
var volumeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

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
