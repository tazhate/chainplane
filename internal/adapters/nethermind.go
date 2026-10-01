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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// --------------------------------------------------------------------------
// Shared Nethermind setup for networks with a built-in Nethermind config
// --------------------------------------------------------------------------

// nethermindConfigFile is the reference copy of the overrides rendered into
// the config ConfigMap.
const nethermindConfigFile = "nethermind.json"

// nethermindSetting is one Nethermind option, e.g. JsonRpc.Port.
type nethermindSetting struct {
	Section, Key string
	Value        any // bool, int32, string or []string
}

// nethermindNode runs Nethermind on one of its built-in network configs
// (--config gnosis, --config fuse, ...). Those configs carry the chainspec,
// the sync pivot and the bootnodes, which upstream bumps every release.
// Nethermind loads exactly one config file, so the operator's settings are
// passed on the command line on top of it; the rendered nethermind.json lists
// the same settings for reference and is not read by the node.
type nethermindNode struct {
	// Config is the built-in config name.
	Config      string
	P2PPort     int32
	MetricsPort int32
	// Extra settings follow the shared ones, e.g. the Engine API endpoint.
	Extra []nethermindSetting
}

func (n nethermindNode) settings() []nethermindSetting {
	return append([]nethermindSetting{
		{"Init", "WebSocketsEnabled", true},
		{"JsonRpc", "Enabled", true},
		{"JsonRpc", "Host", "0.0.0.0"},
		{"JsonRpc", "Port", int32(8545)},
		{"JsonRpc", "WebSocketsPort", int32(8546)},
		{"JsonRpc", "EnabledModules", []string{"Eth", "Net", "Web3", "Subscribe", "Health"}},
		{"Network", "P2PPort", n.P2PPort},
		{"Network", "DiscoveryPort", n.P2PPort},
		{"HealthChecks", "Enabled", true},
		{"Metrics", "Enabled", true},
		{"Metrics", "ExposePort", n.MetricsPort},
	}, n.Extra...)
}

// args returns the Nethermind command line: the built-in config, the data
// directory on the data volume and the settings as --Section.Key value pairs.
func (n nethermindNode) args() []string {
	settings := n.settings()
	args := make([]string, 0, 4+2*len(settings))
	args = append(args, "--config", n.Config, "--datadir", "/data")
	for _, s := range settings {
		var v string
		switch val := s.Value.(type) {
		case bool:
			v = strconv.FormatBool(val)
		case int32:
			v = strconv.Itoa(int(val))
		case []string:
			v = "[" + strings.Join(val, ",") + "]"
		default:
			v = fmt.Sprint(val)
		}
		args = append(args, "--"+s.Section+"."+s.Key, v)
	}
	return args
}

// configFile renders the settings as a Nethermind JSON config.
func (n nethermindNode) configFile() (string, error) {
	cfg := map[string]map[string]any{}
	for _, s := range n.settings() {
		if cfg[s.Section] == nil {
			cfg[s.Section] = map[string]any{}
		}
		cfg[s.Section][s.Key] = s.Value
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("nethermind config: %w", err)
	}
	return string(out) + "\n", nil
}
