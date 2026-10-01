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
package adapters

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

var cosmosNodes = map[string]cosmosNode{
	"axelar":    axelarNode,
	"cosmos":    cosmosHubNode,
	"dymension": dymensionNode,
	"evmos":     evmosNode,
	"haqq":      haqqNode,
	"kava":      kavaNode,
	"mezo":      mezoNode,
	"moca":      mocaNode,
	"osmosis":   osmosisNode,
	"sei":       seiNode,
}

var sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)

// TestCosmosNodesBootstrap checks every Cosmos-family node initializes
// with its chain ID, verifies a pinned genesis and starts with --home /data.
func TestCosmosNodesBootstrap(t *testing.T) {
	for name, n := range cosmosNodes {
		t.Run(name, func(t *testing.T) {
			if !sha256Re.MatchString(n.GenesisSHA256) {
				t.Errorf("GenesisSHA256 %q is not a hex SHA-256", n.GenesisSHA256)
			}
			if !strings.HasPrefix(n.GenesisURL, "https://") {
				t.Errorf("GenesisURL %q is not https", n.GenesisURL)
			}
			if l := len(n.StateSyncRPC); l == 1 {
				t.Errorf("state sync needs two RPC servers, got %v", n.StateSyncRPC)
			}
			script := n.script()
			for _, want := range []string{
				"init \"${HOSTNAME:-chainplane}\" --chain-id " + n.ChainID + " --home $H",
				`echo "` + n.GenesisSHA256 + `  $C/genesis.dl" | sha256sum -c -`,
				"get " + n.GenesisURL + " $C/genesis.dl",
				`exec $B start --home $H "$@"`,
			} {
				if !strings.Contains(script, want) {
					t.Errorf("script lacks %q", want)
				}
			}

			cmd := n.Command()
			inits := n.InitContainers()
			if !n.Toolbox {
				if cmd[0] != "sh" || inits != nil {
					t.Errorf("without toolbox: command %q, init containers %d", cmd[0], len(inits))
				}
				return
			}
			if cmd[0] != cosmosToolboxDir+"/busybox" {
				t.Errorf("toolbox command starts with %q", cmd[0])
			}
			if len(inits) != 1 || inits[0].RestartPolicy == nil ||
				*inits[0].RestartPolicy != corev1.ContainerRestartPolicyAlways || inits[0].StartupProbe == nil {
				t.Errorf("toolbox is not a native sidecar with a startup probe: %+v", inits)
			}
		})
	}
}

// cosmosScriptHelpers returns the shell function definitions of the init
// script, ahead of its first-start block.
func cosmosScriptHelpers(t *testing.T) string {
	t.Helper()
	script := kavaNode.script()
	start := strings.Index(script, "get() {")
	end := strings.Index(script, "if [ ! -f $C/genesis.json ]")
	if start < 0 || end < start {
		t.Fatal("cannot locate the helper functions in the script")
	}
	return script[start:end]
}

func runShell(t *testing.T, script string) {
	t.Helper()
	for _, tool := range []string{"sh", "awk"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}
	out, err := exec.Command("sh", "-c", "set -e\n"+script).CombinedOutput()
	if err != nil {
		t.Fatalf("sh: %v\n%s", err, out)
	}
}

func TestCosmosScriptTset(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	in := `proxy_app = "tcp://127.0.0.1:26658"

[rpc]
laddr = "tcp://127.0.0.1:26657"

[p2p]
laddr = "tcp://0.0.0.0:26656"
seeds = ""
bootstrap-peers = ""

[statesync] # comment after the header
enable = false
trust_height = 0
`
	if err := os.WriteFile(cfg, []byte(in), 0o600); err != nil {
		t.Fatal(err)
	}
	runShell(t, cosmosScriptHelpers(t)+`
tset `+cfg+` p2p seeds '"a@b:1"'
tset `+cfg+` p2p bootstrap_peers '"a@b:1"'
tset `+cfg+` p2p laddr '"tcp://0.0.0.0:1"'
tset `+cfg+` statesync enable true
tset `+cfg+` statesync trust_height 42
tset `+cfg+` statesync missing_key 1
`)
	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := `proxy_app = "tcp://127.0.0.1:26658"

[rpc]
laddr = "tcp://127.0.0.1:26657"

[p2p]
laddr = "tcp://0.0.0.0:1"
seeds = "a@b:1"
bootstrap-peers = "a@b:1"

[statesync] # comment after the header
enable = true
trust_height = 42
`
	if string(got) != want {
		t.Errorf("tset result:\n%s\nwant:\n%s", got, want)
	}
}

func TestCosmosScriptTmerge(t *testing.T) {
	dir := t.TempDir()
	override := filepath.Join(dir, "override.toml")
	target := filepath.Join(dir, "app.toml")
	if err := os.WriteFile(override, []byte(`# overrides
minimum-gas-prices = "1uatom"

[api]
enable = true
address = "tcp://0.0.0.0:1317"

[json-rpc]
enable = true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(`minimum-gas-prices = ""
pruning = "default"

[telemetry]
enabled = false
global-labels = [
]

[api]
enable = false
swagger = false

[grpc]
enable = true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	runShell(t, cosmosScriptHelpers(t)+"tmerge "+override+" "+target+"\n")
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	want := `minimum-gas-prices = "1uatom"
pruning = "default"

[telemetry]
enabled = false
global-labels = [
]

[api]
enable = true
swagger = false
address = "tcp://0.0.0.0:1317"

[grpc]
enable = true

[json-rpc]
enable = true
`
	if string(got) != want {
		t.Errorf("tmerge result:\n%s\nwant:\n%s", got, want)
	}
}

// TestCosmosScriptGenesisRPC unwraps a CometBFT /genesis response the way
// the moca script does.
func TestCosmosScriptGenesisRPC(t *testing.T) {
	script := mocaNode.script()
	i := strings.Index(script, "sed -e ")
	if i < 0 {
		t.Fatal("moca script does not unwrap the RPC response")
	}
	sed, _, _ := strings.Cut(script[i:], " $C/genesis.dl")
	dir := t.TempDir()
	dl := filepath.Join(dir, "genesis.dl")
	inner := `{"genesis_time":"2025-12-17T14:16:16Z","chain_id":"moca_2288-1","app_state":{"a":{"b":{}}}}`
	if err := os.WriteFile(dl, []byte(`{"jsonrpc":"2.0","id":-1,"result":{"genesis":`+inner+`}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "genesis.json")
	runShell(t, sed+" "+dl+" > "+out)
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != inner {
		t.Errorf("unwrapped genesis = %s, want %s", got, inner)
	}
}
