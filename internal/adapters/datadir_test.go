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

	corev1 "k8s.io/api/core/v1"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

// The viction image entrypoint hardcodes --datadir data (/tomochain/data),
// so the adapter runs tomo itself: genesis init and data on /data, the
// mounted config, and the network the entrypoint would select.
func TestVictionDataDirAndNetwork(t *testing.T) {
	for _, tc := range []struct {
		network chainsv1alpha2.Network
		genesis string
		want    []string
	}{
		{chainsv1alpha2.NetworkMainnet, "/tomochain/mainnet.json", []string{"--networkid 88"}},
		{chainsv1alpha2.NetworkTestnet, "/tomochain/testnet.json", []string{"--networkid 89", "--tomo-testnet"}},
	} {
		t.Run(string(tc.network), func(t *testing.T) {
			line := strings.Join(evmFlagsCommandLine(t, chainsv1alpha2.ChainViction, tc.network), " ")
			for _, want := range append(tc.want,
				"tomo init --datadir /data "+tc.genesis,
				`exec tomo "$@"`,
				"--config /config/config.toml",
			) {
				if !strings.Contains(line, want) {
					t.Errorf("command line lacks %q: %s", want, line)
				}
			}
			if strings.Contains(line, "entrypoint.sh") || strings.Contains(line, "--datadir data") {
				t.Errorf("command line still uses the image entrypoint: %s", line)
			}
		})
	}

	_, content, err := adapters.MustGet(chainsv1alpha2.ChainViction).ConfigTemplate(chainsv1alpha2.ChainInstanceSpec{})
	if err != nil {
		t.Fatalf("ConfigTemplate: %v", err)
	}
	// tomo is geth 1.8: no snap sync, and the CORS field is HTTPCors.
	for _, want := range []string{`DataDir = "/data"`, `SyncMode = "full"`, `HTTPCors = ["*"]`} {
		if !strings.Contains(content, want) {
			t.Errorf("config lacks %s", want)
		}
	}
}

// Without -Drsk.conf.file rskj reads no config and keeps its database under
// /var/lib/rsk/.rsk; the image's RSKJ_SYS_PROPS also set http.hosts.N,
// which rskj rejects as an object where it expects a list.
func TestRootstockConfigAndDataDir(t *testing.T) {
	adapter := adapters.MustGet(chainsv1alpha2.ChainRootstock)
	spec := chainsv1alpha2.ChainInstanceSpec{Chain: chainsv1alpha2.ChainRootstock}
	filename, content, err := adapter.ConfigTemplate(spec)
	if err != nil {
		t.Fatalf("ConfigTemplate: %v", err)
	}
	env := adapter.(adapters.ContainerEnvProvider).ContainerEnv(spec)
	i := slices.IndexFunc(env, func(e corev1.EnvVar) bool { return e.Name == "RSKJ_SYS_PROPS" })
	if i < 0 {
		t.Fatal("RSKJ_SYS_PROPS is not set")
	}
	props := env[i].Value
	if !strings.Contains(props, "-Drsk.conf.file=/config/"+filename) {
		t.Errorf("RSKJ_SYS_PROPS %q does not load /config/%s", props, filename)
	}
	if strings.Contains(props, "hosts.") {
		t.Errorf("RSKJ_SYS_PROPS %q still sets http.hosts by index", props)
	}
	for _, want := range []string{`database.dir = "/data"`, `hosts = ["*"]`} {
		if !strings.Contains(content, want) {
			t.Errorf("config lacks %s", want)
		}
	}
	if strings.Contains(content, "metrics") {
		t.Error("config has a metrics section; rskj has none and warns about it")
	}
	for _, p := range adapter.ContainerPorts(spec) {
		if p.Name == "metrics" {
			t.Errorf("rootstock exposes metrics port %d but rskj serves no metrics", p.ContainerPort)
		}
	}
}

// --taiko picks the genesis by network id; testnet is Taiko Hoodi.
func TestTaikoTestnetNetworkID(t *testing.T) {
	line := strings.Join(evmFlagsCommandLine(t, chainsv1alpha2.ChainTaiko, chainsv1alpha2.NetworkTestnet), " ")
	if !strings.Contains(line, "--taiko --networkid 167013") {
		t.Errorf("testnet command line does not select Taiko Hoodi: %s", line)
	}
}
