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
	plan, err := buildRunPlan(planInput{
		name:          "chainsmoke-x",
		pod:           testPod(),
		configMapName: "x-mainnet-config",
		configFile:    "node.toml",
		configContent: "network = \"mainnet\"\n",
		workDir:       work,
		tmpfsSize:     "1g",
	})
	if err != nil {
		t.Fatal(err)
	}

	cfgDir := filepath.Join(work, "config")
	want := []string{
		"run", "-d", "--name", "chainsmoke-x", "--platform", "linux/amd64", "--label", "chainsmoke=1",
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
	if got := plan.files[filepath.Join(cfgDir, "node.toml")]; got != "network = \"mainnet\"\n" {
		t.Errorf("config file content = %q", got)
	}

	notes := strings.Join(plan.notes, "\n")
	for _, n := range []string{
		"init containers skipped: snapshot-restore",
		"sidecars skipped: metrics-exporter",
		"volume keys mounted empty at /keys",
		"duplicate mount /data skipped",
	} {
		if !strings.Contains(notes, n) {
			t.Errorf("notes %q missing %q", plan.notes, n)
		}
	}
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
