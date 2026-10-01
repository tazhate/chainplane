/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestExpand(t *testing.T) {
	vars := map[string]string{"HOME": "/data", "NET": "mainnet", "EMPTY": ""}
	lookup := func(k string) (string, bool) { v, ok := vars[k]; return v, ok }
	tests := []struct{ in, want string }{
		{"plain", "plain"},
		{"$(HOME)/db", "/data/db"},
		{"--net=$(NET) --home=$(HOME)", "--net=mainnet --home=/data"},
		{"$(MISSING)", "$(MISSING)"},
		{"$$(HOME)", "$(HOME)"},
		{"cost $5", "cost $5"},
		{"trailing $", "trailing $"},
		{"$(HOME", "$(HOME"},
		{"x$(EMPTY)y", "xy"},
		{"$$$(NET)", "$mainnet"},
	}
	for _, tt := range tests {
		if got := expand(tt.in, lookup); got != tt.want {
			t.Errorf("expand(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveEnv(t *testing.T) {
	c := corev1.Container{
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")},
		},
		Env: []corev1.EnvVar{
			{Name: "BASE", Value: "/data"},
			{Name: "DB", Value: "$(BASE)/db"},
			{Name: "LATER", Value: "$(DEFINED_AFTER)"},
			{Name: "DEFINED_AFTER", Value: "x"},
			{Name: "PASS", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "k"}}},
			{Name: "CM", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{Key: "k"}}},
			{Name: "POD", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
			{Name: "IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
			{Name: "MEM_MI", ValueFrom: &corev1.EnvVarSource{ResourceFieldRef: &corev1.ResourceFieldSelector{
				Resource: "limits.memory", Divisor: resource.MustParse("1Mi"),
			}}},
			{Name: "BASE", Value: "/override"},
		},
	}
	env, lookup := resolveEnv(c, "chainsmoke-x")
	want := []string{
		"BASE=/override",
		"DB=/data/db",
		"LATER=$(DEFINED_AFTER)",
		"DEFINED_AFTER=x",
		"PASS=smoke",
		"CM=smoke",
		"POD=chainsmoke-x",
		"IP=127.0.0.1",
		"MEM_MI=8192",
	}
	if !slices.Equal(env, want) {
		t.Errorf("env =\n%q\nwant\n%q", env, want)
	}
	if v, _ := lookup("BASE"); v != "/override" {
		t.Errorf("lookup(BASE) = %q, want the last definition", v)
	}
}

func testPod() corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{Spec: corev1.PodSpec{
		SecurityContext: &corev1.PodSecurityContext{FSGroup: new(int64(1000))},
		InitContainers:  []corev1.Container{{Name: "snapshot-restore"}},
		Containers: []corev1.Container{
			{
				Name:    "node",
				Image:   "example/node:v1",
				Command: []string{"/bin/node", "--home=$(HOME)"},
				Args:    []string{"--config=/config/node.toml", "--network=$(NETWORK)"},
				Env: []corev1.EnvVar{
					{Name: "HOME", Value: "/data"},
					{Name: "NETWORK", Value: "mainnet"},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "data", MountPath: "/data"},
					{Name: "config", MountPath: "/config", ReadOnly: true},
					{Name: "config", MountPath: "/etc/node/node.toml", SubPath: "node.toml"},
					{Name: "keys", MountPath: "/keys"},
					{Name: "scratch", MountPath: "/data"},
				},
			},
			{Name: "metrics-exporter", Image: "example/exporter:v1"},
		},
		Volumes: []corev1.Volume{
			{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "x-mainnet-config"},
			}}},
			{Name: "keys", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "keys"}}},
			{Name: "scratch", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		},
	}}
}

func TestBuildRunPlan(t *testing.T) {
	work := t.TempDir()
	pod := testPod()
	pod.Spec.Containers = pod.Spec.Containers[:1]
	plan, err := buildRunPlan(planInput{
		name:          "chainsmoke-x",
		pod:           pod,
		configMapName: "x-mainnet-config",
		configFile:    "node.toml",
		configContent: "network = \"mainnet\"\n",
		workDir:       work,
		tmpfsSize:     "1g",
		nofile:        1048576,
		prefix:        "chainsmoke",
	})
	if err != nil {
		t.Fatal(err)
	}

	cfgDir := filepath.Join(work, "config")
	want := []string{
		"run", "-d", "--name", "chainsmoke-x", "--platform", "linux/amd64",
		"--label", "chainsmoke=1", "--label", "chainsmoke.prefix=chainsmoke",
		"--ulimit", "nofile=1048576:1048576",
		"--group-add", "1000",
		"--env", "HOME=/data",
		"--env", "NETWORK=mainnet",
		"--tmpfs", "/data:rw,exec,mode=1777,size=1g",
		"--mount", "type=bind,src=" + cfgDir + ",dst=/config,readonly",
		"--mount", "type=bind,src=" + filepath.Join(cfgDir, "node.toml") + ",dst=/etc/node/node.toml,readonly",
		"--tmpfs", "/keys:rw,exec,mode=1777,size=1g",
		"--entrypoint", "/bin/node",
		"example/node:v1",
		"--home=/data", "--config=/config/node.toml", "--network=mainnet",
	}
	if !slices.Equal(plan.args, want) {
		t.Errorf("args =\n%q\nwant\n%q", plan.args, want)
	}
	if plan.image != "example/node:v1" {
		t.Errorf("image = %q", plan.image)
	}
	if plan.pod != nil {
		t.Errorf("a pod without sidecars must run as a single container, got pod %+v", plan.pod)
	}
	if got := plan.containers("chainsmoke-x"); !slices.Equal(got, []string{"chainsmoke-x"}) {
		t.Errorf("containers = %q", got)
	}
	if got := plan.files[filepath.Join(cfgDir, "node.toml")]; got != "network = \"mainnet\"\n" {
		t.Errorf("config file content = %q", got)
	}

	notes := strings.Join(plan.notes, "\n")
	for _, n := range []string{
		"init containers skipped: snapshot-restore",
		"volume keys mounted empty at /keys",
		"duplicate mount /data skipped",
	} {
		if !strings.Contains(notes, n) {
			t.Errorf("notes %q missing %q", plan.notes, n)
		}
	}
}

func TestBuildRunPlanNoUlimit(t *testing.T) {
	pod := testPod()
	pod.Spec.Containers = pod.Spec.Containers[:1]
	plan, err := buildRunPlan(planInput{name: "n", pod: pod, tmpfsSize: "1g"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(plan.args, "--ulimit") {
		t.Errorf("nofile 0 must keep the docker default: %q", plan.args)
	}
	if slices.Contains(plan.args, "--network") {
		t.Errorf("a single container stays on the default network: %q", plan.args)
	}
}

// zkStackLikePod mirrors the ZK Stack pod: a Postgres native sidecar on a
// subPath of the data volume, the node on the whole volume, and a regular
// sidecar reading the data volume read-only.
func zkStackLikePod() corev1.PodTemplateSpec {
	pod := testPod()
	pod.Spec.InitContainers = []corev1.Container{
		{Name: "snapshot-restore", Image: "example/restore:v1"},
		{
			Name:          "postgres",
			Image:         "postgres:16.15",
			RestartPolicy: new(corev1.ContainerRestartPolicyAlways),
			Args:          []string{"-c", "listen_addresses=127.0.0.1"},
			StartupProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{Command: []string{"pg_isready", "-h", "127.0.0.1"}},
			}},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data", MountPath: "/var/lib/postgresql/data", SubPath: "postgres"},
			},
		},
		{
			Name:          "proxy",
			Image:         "example/proxy:v1",
			RestartPolicy: new(corev1.ContainerRestartPolicyAlways),
			ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{},
			}},
		},
	}
	pod.Spec.Containers = []corev1.Container{
		{
			Name:  "node",
			Image: "example/node:v1",
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data", MountPath: "/data"},
				{Name: "config", MountPath: "/config", ReadOnly: true},
			},
		},
		{
			Name:         "metrics-exporter",
			Image:        "example/exporter:v1",
			VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/chain", ReadOnly: true}},
		},
	}
	return pod
}

func TestBuildRunPlanPod(t *testing.T) {
	plan, err := buildRunPlan(planInput{
		name:          "smokefix-zk",
		pod:           zkStackLikePod(),
		configMapName: "x-mainnet-config",
		configFile:    "node.toml",
		workDir:       t.TempDir(),
		tmpfsSize:     "1g",
		nofile:        4096,
		prefix:        "smokefix",
		volumeSubpath: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.pod
	if p == nil {
		t.Fatal("sidecars must turn the plan into a pod")
	}

	// Native sidecars in spec order, the ordinary init container skipped.
	if got := []string{p.sidecars[0].name, p.sidecars[1].name}; len(p.sidecars) != 2 ||
		!slices.Equal(got, []string{"smokefix-zk-postgres", "smokefix-zk-proxy"}) {
		t.Fatalf("sidecars = %+v", p.sidecars)
	}
	if len(p.others) != 1 || p.others[0].name != "smokefix-zk-metrics-exporter" {
		t.Fatalf("others = %+v", p.others)
	}
	if !slices.Equal(p.sidecars[0].probe, []string{"pg_isready", "-h", "127.0.0.1"}) {
		t.Errorf("postgres probe = %q", p.sidecars[0].probe)
	}
	if p.sidecars[1].probe != nil {
		t.Errorf("tcp probe must not be exec'd: %q", p.sidecars[1].probe)
	}

	// Every container joins the holder network namespace and gets the ulimit.
	for _, args := range [][]string{p.sidecars[0].args, p.sidecars[1].args, plan.args, p.others[0].args} {
		if !hasPair(args, "--network", "container:smokefix-zk-pod") {
			t.Errorf("args %q do not join the holder network", args)
		}
		if !hasPair(args, "--ulimit", "nofile=4096:4096") {
			t.Errorf("args %q lack the nofile ulimit", args)
		}
	}

	// One shared tmpfs volume for data; subPath through volume-subpath.
	if !slices.Equal(p.volumes, []string{"smokefix-zk-data"}) {
		t.Errorf("volumes = %q", p.volumes)
	}
	mounts := map[string]string{
		"postgres": "type=volume,src=smokefix-zk-data,dst=/var/lib/postgresql/data,volume-subpath=postgres",
		"node":     "type=volume,src=smokefix-zk-data,dst=/data",
		"exporter": "type=volume,src=smokefix-zk-data,dst=/chain,readonly",
	}
	containerArgs := map[string][]string{"postgres": p.sidecars[0].args, "node": plan.args, "exporter": p.others[0].args}
	for who, args := range containerArgs {
		if !hasPair(args, "--mount", mounts[who]) {
			t.Errorf("%s args %q lack mount %q", who, args, mounts[who])
		}
	}

	wantSetup := [][]string{
		{"volume", "create", "--driver", "local", "--opt", "type=tmpfs", "--opt", "device=tmpfs",
			"--opt", "o=size=1g,mode=1777", "--label", "chainsmoke=1", "--label", "chainsmoke.prefix=smokefix",
			"smokefix-zk-data"},
		{"run", "-d", "--name", "smokefix-zk-pod", "--platform", "linux/amd64",
			"--label", "chainsmoke=1", "--label", "chainsmoke.prefix=smokefix",
			"--mount", "type=volume,src=smokefix-zk-data,dst=/volumes/smokefix-zk-data",
			podHolderImage, "sleep", "2147483647"},
		{"exec", "smokefix-zk-pod", "mkdir", "-p", "-m", "1777", "/volumes/smokefix-zk-data/postgres"},
	}
	if !slices.EqualFunc(p.setup, wantSetup, slices.Equal) {
		t.Errorf("setup =\n%q\nwant\n%q", p.setup, wantSetup)
	}

	wantContainers := []string{
		"smokefix-zk-metrics-exporter", "smokefix-zk", "smokefix-zk-proxy", "smokefix-zk-postgres", "smokefix-zk-pod",
	}
	if got := plan.containers("smokefix-zk"); !slices.Equal(got, wantContainers) {
		t.Errorf("containers = %q, want %q", got, wantContainers)
	}
	wantImages := []string{"example/node:v1", "postgres:16.15", "example/proxy:v1", "example/exporter:v1", podHolderImage}
	if got := plan.images(); !slices.Equal(got, wantImages) {
		t.Errorf("images = %q, want %q", got, wantImages)
	}

	notes := strings.Join(plan.notes, "\n")
	for _, n := range []string{"init containers skipped: snapshot-restore", "sidecar proxy probe is not exec"} {
		if !strings.Contains(notes, n) {
			t.Errorf("notes %q missing %q", plan.notes, n)
		}
	}
}

func TestBuildRunPlanPodWithoutVolumeSubpath(t *testing.T) {
	plan, err := buildRunPlan(planInput{
		name: "smokefix-zk", pod: zkStackLikePod(), configMapName: "x-mainnet-config",
		workDir: t.TempDir(), tmpfsSize: "1g",
	})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.pod
	want := "type=volume,src=smokefix-zk-data-postgres,dst=/var/lib/postgresql/data"
	if !hasPair(p.sidecars[0].args, "--mount", want) {
		t.Errorf("postgres args %q lack mount %q", p.sidecars[0].args, want)
	}
	if !slices.Equal(p.volumes, []string{"smokefix-zk-data-postgres", "smokefix-zk-data"}) {
		t.Errorf("volumes = %q", p.volumes)
	}
	for _, cmd := range p.setup {
		if cmd[0] == "exec" {
			t.Errorf("no subPath directories to create without volume-subpath: %q", cmd)
		}
	}
}

// hasPair reports whether flag is directly followed by value in args.
func hasPair(args []string, flag, value string) bool {
	for i := range len(args) - 1 {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func TestBuildRunPlanArgsOnly(t *testing.T) {
	pod := corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
		Name:  "node",
		Image: "example/node:v1",
		Args:  []string{"node", "-datadir=/data"},
	}}}}
	plan, err := buildRunPlan(planInput{name: "n", pod: pod, tmpfsSize: "1g"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(plan.args, "--entrypoint") {
		t.Errorf("args-only container must keep the image entrypoint: %q", plan.args)
	}
	if got := plan.args[len(plan.args)-3:]; !slices.Equal(got, []string{"example/node:v1", "node", "-datadir=/data"}) {
		t.Errorf("tail of args = %q", got)
	}
}

func TestBuildRunPlanNoMainContainer(t *testing.T) {
	pod := corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "other"}}}}
	if _, err := buildRunPlan(planInput{pod: pod}); err == nil {
		t.Error("expected an error without a node container")
	}
}

func TestRunAsUser(t *testing.T) {
	type podSC = corev1.PodSecurityContext
	tests := []struct {
		name string
		pod  *podSC
		c    *corev1.SecurityContext
		want string
	}{
		{"none", nil, nil, ""},
		{"pod uid", &podSC{RunAsUser: new(int64(1000))}, nil, "1000"},
		{"pod uid gid", &podSC{RunAsUser: new(int64(1000)), RunAsGroup: new(int64(2000))}, nil, "1000:2000"},
		{"container wins", &podSC{RunAsUser: new(int64(1000))}, &corev1.SecurityContext{RunAsUser: new(int64(0))}, "0"},
	}
	for _, tt := range tests {
		if got := runAsUser(tt.pod, tt.c); got != tt.want {
			t.Errorf("%s: runAsUser = %q, want %q", tt.name, got, tt.want)
		}
	}
}
