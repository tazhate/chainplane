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
package adapters_test

import (
	"slices"
	"strings"
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

// opGethChains run op-geth (or a fork of it) with the mounted config.toml,
// mapped to the network flags that select their genesis. "" means the
// adapter runs geth init from a pinned genesis instead.
var opGethChains = map[chainsv1alpha2.Chain]string{
	chainsv1alpha2.ChainOpBNB: "--opBNBMainnet",
	chainsv1alpha2.ChainBlast: "",
}

// opGethFlags are the flags these adapters may pass; geth exits on unknown ones.
var opGethFlags = []string{
	"--config", "--opBNBMainnet",
	"--metrics", "--metrics.addr", "--metrics.port",
}

// opGethTOMLKeys lists, per TOML table, the geth config fields the templates
// may set. geth decodes config.toml strictly into ethconfig.Config and
// node.Config, so a sub-table such as [Node.HTTPHost] or a misspelled field
// stops it at startup.
var opGethTOMLKeys = map[string][]string{
	"Eth": {"NetworkId", "SyncMode"},
	"Node": {
		"DataDir", "HTTPHost", "HTTPPort", "HTTPVirtualHosts", "HTTPCors", "HTTPModules",
		"WSHost", "WSPort", "WSOrigins", "WSModules",
	},
	"Node.P2P": {"MaxPeers", "ListenAddr"},
}

func TestOpGethArgsLoadConfig(t *testing.T) {
	for chain, network := range opGethChains {
		t.Run(string(chain), func(t *testing.T) {
			adapter := adapters.MustGet(chain)
			spec := chainsv1alpha2.ChainInstanceSpec{Chain: chain}
			args := adapter.(adapters.ContainerArgsProvider).ContainerArgs(spec)
			i := slices.Index(args, "--config")
			if i < 0 || i+1 >= len(args) || args[i+1] != "/config/config.toml" {
				t.Errorf("args %q do not pass --config /config/config.toml", args)
			}
			for _, arg := range args {
				if strings.HasPrefix(arg, "--") && !slices.Contains(opGethFlags, arg) {
					t.Errorf("unexpected op-geth flag %q", arg)
				}
			}
			joined := strings.Join(args, " ")
			if network != "" {
				if !strings.Contains(joined, network) {
					t.Errorf("args %q do not select the network with %s", args, network)
				}
				return
			}
			ccp, ok := adapter.(adapters.ContainerCommandProvider)
			if !ok || !strings.Contains(strings.Join(ccp.ContainerCommand(spec), " "), "geth init --datadir /data") {
				t.Error("no network flag and no geth init: geth would boot on the Ethereum mainnet genesis")
			}
		})
	}
}

func TestOpGethConfigKnownFields(t *testing.T) {
	for chain := range opGethChains {
		t.Run(string(chain), func(t *testing.T) {
			_, content, err := adapters.MustGet(chain).ConfigTemplate(chainsv1alpha2.ChainInstanceSpec{Chain: chain})
			if err != nil {
				t.Fatalf("ConfigTemplate: %v", err)
			}
			table := ""
			for line := range strings.Lines(content) {
				line = strings.TrimSpace(line)
				switch {
				case line == "" || strings.HasPrefix(line, "#"):
				case strings.HasPrefix(line, "["):
					table = strings.Trim(line, "[]")
					if _, ok := opGethTOMLKeys[table]; !ok {
						t.Errorf("unknown table [%s]", table)
					}
				default:
					key, _, _ := strings.Cut(line, "=")
					key = strings.TrimSpace(key)
					if !slices.Contains(opGethTOMLKeys[table], key) {
						t.Errorf("[%s] has unknown field %q", table, key)
					}
				}
			}
			if !strings.Contains(content, `DataDir = "/data"`) {
				t.Error(`config does not set DataDir = "/data"`)
			}
		})
	}
}

func TestBlastGenesisVerified(t *testing.T) {
	ccp, ok := adapters.MustGet(chainsv1alpha2.ChainBlast).(adapters.ContainerCommandProvider)
	if !ok {
		t.Fatal("blast adapter does not implement ContainerCommandProvider")
	}
	script := strings.Join(ccp.ContainerCommand(chainsv1alpha2.ChainInstanceSpec{}), " ")
	for _, want := range []string{
		"blast-io/deployment/197a870f8d9f65d29ad3eef5566f40bb2f5c5353/mainnet/genesis.json",
		"330379e670b42f8a2b9db30b059f6ce7400590b66c70c2d8f7294ba0efeffbae",
		"sha256sum -c -",
		"geth init --datadir /data",
		`exec geth "$@"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("blast command does not contain %q", want)
		}
	}
}
