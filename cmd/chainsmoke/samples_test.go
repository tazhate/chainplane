/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
	"github.com/tazhate/chainplane/internal/controller"
)

const samplesDir = "../../config/samples"

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecodeRealSamples(t *testing.T) {
	tests := []struct {
		file   string
		name   string
		chain  chainsv1alpha2.Chain
		client string
	}{
		{"chains_v1alpha2_chaininstance_dash.yaml", "dash-mainnet", chainsv1alpha2.ChainDash, ""},
		{"chains_v1alpha2_chaininstance_ethereum.yaml", "ethereum-mainnet", chainsv1alpha2.ChainEthereum, "nethermind"},
	}
	for _, tt := range tests {
		node, err := decodeSample(mustRead(t, filepath.Join(samplesDir, tt.file)))
		if err != nil {
			t.Fatalf("%s: %v", tt.file, err)
		}
		if node.Name != tt.name || node.Namespace != smokeNamespace ||
			node.Spec.Chain != tt.chain || node.Spec.Client != tt.client {
			t.Errorf("%s: got %s/%s chain=%s client=%q", tt.file, node.Namespace, node.Name, node.Spec.Chain, node.Spec.Client)
		}
	}
}

func TestDecodeSampleRejects(t *testing.T) {
	tests := map[string]string{
		"unknown field": "kind: ChainInstance\nspec:\n  chain: dash\n  chian: typo\n",
		"wrong kind":    "apiVersion: v1\nkind: ConfigMap\nspec:\n  chain: dash\n",
		"no chain":      "apiVersion: chains.chainplane.io/v1alpha2\nkind: ChainInstance\nspec: {}\n",
	}
	for name, doc := range tests {
		if _, err := decodeSample([]byte(doc)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLoadSamplesCoversAllChains(t *testing.T) {
	samples, err := loadSamples(samplesDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range samples {
		if s.err != nil {
			t.Errorf("%s: %v", filepath.Base(s.path), s.err)
		}
	}
	if len(samples) < 100 {
		t.Errorf("loaded %d samples, expected one per chain", len(samples))
	}
}

func TestChainFromSamplePath(t *testing.T) {
	if got := chainFromSamplePath("config/samples/chains_v1alpha2_chaininstance_boba_eth.yaml"); got != "boba-eth" {
		t.Errorf("chainFromSamplePath = %q", got)
	}
}

// TestDashSampleRunPlan renders the real dash sample through the operator's
// pod template and checks the resulting docker invocation end to end.
func TestDashSampleRunPlan(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "")
	node, err := decodeSample(mustRead(t, filepath.Join(samplesDir, "chains_v1alpha2_chaininstance_dash.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	adapter := adapters.MustGet(node.Spec.Chain)
	cfgFile, cfgContent, err := adapter.ConfigTemplate(node.Spec)
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	plan, err := buildRunPlan(planInput{
		name:          "chainsmoke-dash",
		pod:           controller.RenderPodTemplate(node, adapter, ""),
		configMapName: node.Name + "-config",
		configFile:    cfgFile,
		configContent: cfgContent,
		workDir:       work,
		tmpfsSize:     "4g",
	})
	if err != nil {
		t.Fatal(err)
	}

	if plan.image != adapters.DefaultImageFor(chainsv1alpha2.ChainDash, "") {
		t.Errorf("image = %q, want the dash default", plan.image)
	}
	joined := strings.Join(plan.args, " ")
	for _, want := range []string{
		"--env DASH_RPC_USER=smoke",
		"--tmpfs /data:rw,exec,mode=1777,size=4g",
		"--mount type=bind,src=" + filepath.Join(work, "config") + ",dst=/config,readonly",
		plan.image + " dashd -conf=/config/dash.conf",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("docker args %q missing %q", joined, want)
		}
	}
	if slices.Contains(plan.args, "--entrypoint") {
		t.Error("dash keeps the image entrypoint")
	}
	if !strings.Contains(plan.files[filepath.Join(work, "config", "dash.conf")], "datadir=/data") {
		t.Errorf("dash.conf not planned: %v", plan.files)
	}
}
