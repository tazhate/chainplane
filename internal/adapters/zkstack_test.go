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
	"regexp"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

var zkStackChains = []chainsv1alpha2.Chain{
	chainsv1alpha2.ChainZkSync,
	chainsv1alpha2.ChainAbstract,
	chainsv1alpha2.ChainLens,
	chainsv1alpha2.ChainZeroNetwork,
	chainsv1alpha2.ChainCronosZkEVM,
}

// zkStackFlags are the CLI flags zksync_external_node accepts; everything
// else is configured through EN_* env vars.
var zkStackFlags = []string{
	"--enable-consensus",
	"--components",
	"--config-path",
	"--secrets-path",
	"--external-node-config-path",
	"--consensus-path",
}

func envMap(env []corev1.EnvVar) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		m[e.Name] = e.Value
	}
	return m
}

func TestZkStackPostgresSidecar(t *testing.T) {
	for _, chain := range zkStackChains {
		t.Run(string(chain), func(t *testing.T) {
			adapter := adapters.MustGet(chain)
			icp, ok := adapter.(adapters.InitContainerProvider)
			if !ok {
				t.Fatal("adapter does not implement InitContainerProvider")
			}
			containers := icp.InitContainers(chainsv1alpha2.ChainInstanceSpec{Chain: chain})
			i := slices.IndexFunc(containers, func(c corev1.Container) bool { return c.Name == "postgres" })
			if i < 0 {
				t.Fatalf("no postgres init container in %v", containers)
			}
			pg := containers[i]
			if !strings.HasPrefix(pg.Image, "postgres:16.") {
				t.Errorf("image = %q, want a pinned postgres:16.x tag", pg.Image)
			}
			if pg.RestartPolicy == nil || *pg.RestartPolicy != corev1.ContainerRestartPolicyAlways {
				t.Error("postgres must be a native sidecar (restartPolicy Always)")
			}
			if pg.StartupProbe == nil {
				t.Error("postgres sidecar needs a startup probe to gate the node start")
			}
			if !slices.Contains(pg.Args, "listen_addresses=127.0.0.1") {
				t.Errorf("args = %v, want listen_addresses=127.0.0.1 (trust auth must stay pod-local)", pg.Args)
			}
			env := envMap(pg.Env)
			if env["POSTGRES_HOST_AUTH_METHOD"] != "trust" {
				t.Errorf("POSTGRES_HOST_AUTH_METHOD = %q", env["POSTGRES_HOST_AUTH_METHOD"])
			}
			if len(pg.VolumeMounts) != 1 || pg.VolumeMounts[0].Name != "data" || pg.VolumeMounts[0].SubPath != "postgres" {
				t.Errorf("volume mounts = %+v, want the data volume with subPath postgres", pg.VolumeMounts)
			}
			if !strings.HasPrefix(env["PGDATA"], pg.VolumeMounts[0].MountPath+"/") {
				t.Errorf("PGDATA %q is not below the mount %q", env["PGDATA"], pg.VolumeMounts[0].MountPath)
			}
		})
	}
}

func TestZkStackContainerEnv(t *testing.T) {
	wantL2 := map[chainsv1alpha2.Chain]string{
		chainsv1alpha2.ChainZkSync:      "324",
		chainsv1alpha2.ChainAbstract:    "2741",
		chainsv1alpha2.ChainLens:        "232",
		chainsv1alpha2.ChainZeroNetwork: "543210",
		chainsv1alpha2.ChainCronosZkEVM: "388",
	}
	for _, chain := range zkStackChains {
		t.Run(string(chain), func(t *testing.T) {
			ep, ok := adapters.MustGet(chain).(adapters.ContainerEnvProvider)
			if !ok {
				t.Fatal("adapter does not implement ContainerEnvProvider")
			}
			env := envMap(ep.ContainerEnv(chainsv1alpha2.ChainInstanceSpec{Chain: chain}))
			if !strings.HasPrefix(env["DATABASE_URL"], "postgres://postgres@127.0.0.1:5432/") {
				t.Errorf("DATABASE_URL = %q, want the pod-local postgres", env["DATABASE_URL"])
			}
			for _, name := range []string{
				"DATABASE_POOL_SIZE", "EN_HTTP_PORT", "EN_WS_PORT", "EN_HEALTHCHECK_PORT",
				"EN_PROMETHEUS_PORT", "EN_ETH_CLIENT_URL", "EN_MAIN_NODE_URL",
				"EN_STATE_CACHE_PATH", "EN_MERKLE_TREE_PATH", "EN_SNAPSHOTS_RECOVERY_ENABLED",
			} {
				if env[name] == "" {
					t.Errorf("%s is not set", name)
				}
			}
			if env["EN_L1_CHAIN_ID"] != "1" || env["EN_L2_CHAIN_ID"] != wantL2[chain] {
				t.Errorf("chain ids L1=%q L2=%q, want 1/%s", env["EN_L1_CHAIN_ID"], env["EN_L2_CHAIN_ID"], wantL2[chain])
			}
			if env["EN_SNAPSHOTS_RECOVERY_ENABLED"] == "true" && env["EN_SNAPSHOTS_OBJECT_STORE_BUCKET_BASE_URL"] == "" {
				t.Error("snapshot recovery enabled without a snapshots bucket")
			}
		})
	}
}

func TestZkStackArchiveSkipsSnapshotRecovery(t *testing.T) {
	ep := adapters.MustGet(chainsv1alpha2.ChainZkSync).(adapters.ContainerEnvProvider)
	env := envMap(ep.ContainerEnv(chainsv1alpha2.ChainInstanceSpec{
		Chain:    chainsv1alpha2.ChainZkSync,
		NodeType: chainsv1alpha2.NodeTypeArchive,
	}))
	if env["EN_SNAPSHOTS_RECOVERY_ENABLED"] != "false" {
		t.Errorf("archive node EN_SNAPSHOTS_RECOVERY_ENABLED = %q, want false", env["EN_SNAPSHOTS_RECOVERY_ENABLED"])
	}
}

func TestZkStackContainerArgsKnownFlags(t *testing.T) {
	for _, chain := range zkStackChains {
		t.Run(string(chain), func(t *testing.T) {
			ap, ok := adapters.MustGet(chain).(adapters.ContainerArgsProvider)
			if !ok {
				return // no args: the image entrypoint defaults apply
			}
			for _, arg := range ap.ContainerArgs(chainsv1alpha2.ChainInstanceSpec{Chain: chain}) {
				flag, _, _ := strings.Cut(arg, "=")
				if strings.HasPrefix(flag, "--") && !slices.Contains(zkStackFlags, flag) {
					t.Errorf("zksync_external_node does not accept %q", arg)
				}
			}
		})
	}
}

func TestZkStackLivenessOnHealthPort(t *testing.T) {
	for _, chain := range zkStackChains {
		t.Run(string(chain), func(t *testing.T) {
			adapter := adapters.MustGet(chain)
			spec := chainsv1alpha2.ChainInstanceSpec{Chain: chain}
			// The JSON-RPC port stays closed for hours during snapshot recovery.
			if port := adapter.LivenessProbe(spec).TCPSocket.Port.IntValue(); port != 3081 {
				t.Errorf("liveness port = %d, want healthcheck port 3081", port)
			}
			sp, ok := adapter.(adapters.StartupProbeProvider)
			if !ok || sp.StartupProbe(spec) == nil {
				t.Error("missing startup probe")
			}
		})
	}
}

func TestCronosZkEVMVersionPolicy(t *testing.T) {
	vp := adapters.MustGet(chainsv1alpha2.ChainCronosZkEVM).(adapters.VersionProvider)
	pattern := regexp.MustCompile(vp.VersionPolicy().TagPattern)
	for tag, want := range map[string]bool{
		"v31.3.0":         true,
		"v29.17.0":        true,
		"mainnet-v25.0.0": false, // frozen pre-v29 tag scheme
		"v31.3.0-rc1":     false,
		"latest":          false,
	} {
		if got := pattern.MatchString(tag); got != want {
			t.Errorf("TagPattern match %q = %v, want %v", tag, got, want)
		}
	}
	image := adapters.MustGet(chainsv1alpha2.ChainCronosZkEVM).DefaultImage("")
	_, tag, _ := strings.Cut(image, ":")
	if !pattern.MatchString(tag) {
		t.Errorf("default image %q does not match its own TagPattern", image)
	}
}
