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
	"maps"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
	"github.com/tazhate/chainplane/internal/controller"
)

// opRethChains run op-reth with an op-node sidecar, mapped to the op-reth
// --chain and op-node --network names from the bundled superchain registry.
var opRethChains = map[chainsv1alpha2.Chain][2]string{
	chainsv1alpha2.ChainOptimism:   {"optimism", "op-mainnet"},
	chainsv1alpha2.ChainUnichain:   {"unichain", "unichain-mainnet"},
	chainsv1alpha2.ChainWorldchain: {"worldchain", "worldchain-mainnet"},
	chainsv1alpha2.ChainInk:        {"ink", "ink-mainnet"},
	chainsv1alpha2.ChainLisk:       {"lisk", "lisk-mainnet"},
	chainsv1alpha2.ChainMode:       {"mode", "mode-mainnet"},
	chainsv1alpha2.ChainSoneium:    {"soneium", "soneium-mainnet"},
	chainsv1alpha2.ChainZora:       {"zora", "zora-mainnet"},
	chainsv1alpha2.ChainBob:        {"bob", "bob-mainnet"},
	chainsv1alpha2.ChainFraxtal:    {"fraxtal", "fraxtal-mainnet"},
	chainsv1alpha2.ChainHashKey:    {"hashkeychain", "hashkeychain-mainnet"},
	chainsv1alpha2.ChainSuperseed:  {"sseed", "sseed-mainnet"},
}

// opRethFlags and opNodeFlags are the flags the adapters may pass, checked
// against `op-reth node --help` v2.5.0 and `op-node --help` v1.19.8; both
// exit on unknown flags.
var (
	opRethFlags = []string{
		"--chain", "--datadir", "--full",
		"--http", "--http.addr", "--http.port", "--http.api", "--http.corsdomain",
		"--ws", "--ws.addr", "--ws.port", "--ws.api", "--ws.origins",
		"--authrpc.addr", "--authrpc.port", "--port", "--discovery.port", "--metrics",
		"--rollup.sequencer", "--rollup.disable-tx-pool-gossip",
	}
	opNodeFlags = []string{
		"--network", "--l2", "--l2.jwt-secret", "--l2.enginekind", "--syncmode",
		"--rpc.addr", "--rpc.port", "--metrics.enabled", "--metrics.addr", "--metrics.port",
		"--p2p.listen.tcp", "--p2p.listen.udp", "--p2p.priv.path", "--p2p.peerstore.path",
		"--p2p.discovery.path", "--safedb.path",
	}
)

// flagValue returns the argument after flag, or "" when flag is absent.
func flagValue(args []string, flag string) string {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return ""
	}
	return args[i+1]
}

func checkFlags(t *testing.T, what string, args, known []string) {
	t.Helper()
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") && !slices.Contains(known, arg) {
			t.Errorf("%s does not accept %q", what, arg)
		}
	}
}

func opNodeSidecar(t *testing.T, chain chainsv1alpha2.Chain, spec chainsv1alpha2.ChainInstanceSpec) corev1.Container {
	t.Helper()
	sp, ok := adapters.MustGet(chain).(adapters.SidecarProvider)
	if !ok {
		t.Fatalf("%s has no sidecars", chain)
	}
	for _, c := range sp.Sidecars(spec) {
		if c.Name == "op-node" {
			return c
		}
	}
	t.Fatalf("%s has no op-node sidecar", chain)
	return corev1.Container{}
}

func TestOpRethChainsPod(t *testing.T) {
	for chain, names := range opRethChains {
		t.Run(string(chain), func(t *testing.T) {
			adapter := adapters.MustGet(chain)
			spec := chainsv1alpha2.ChainInstanceSpec{Chain: chain}

			if img := adapter.DefaultImage(""); !strings.Contains(img, "oplabs-tools-artifacts/images/op-reth:") {
				t.Errorf("main image %q is not op-reth", img)
			}
			args := adapter.(adapters.ContainerArgsProvider).ContainerArgs(spec)
			if len(args) == 0 || args[0] != "node" {
				t.Fatalf("args %q do not start with the node subcommand", args)
			}
			checkFlags(t, "op-reth", args[1:], opRethFlags)
			if got := flagValue(args, "--chain"); got != names[0] {
				t.Errorf("--chain = %q, want %q", got, names[0])
			}
			if got := flagValue(args, "--datadir"); !strings.HasPrefix(got, "/data/") {
				t.Errorf("--datadir = %q, want a directory on the /data volume", got)
			}
			if flagValue(args, "--rollup.sequencer") == "" {
				t.Error("no --rollup.sequencer: transactions sent to the node would go nowhere")
			}

			node := opNodeSidecar(t, chain, spec)
			if !strings.Contains(node.Image, "oplabs-tools-artifacts/images/op-node:") {
				t.Errorf("sidecar image %q is not op-node", node.Image)
			}
			checkFlags(t, "op-node", node.Args, opNodeFlags)
			for flag, want := range map[string]string{
				"--network":        names[1],
				"--l2":             "http://127.0.0.1:8551",
				"--l2.enginekind":  "reth",
				"--syncmode":       "execution-layer",
				"--l2.jwt-secret":  "/data/reth/jwt.hex",
				"--p2p.priv.path":  "/data/op-node/p2p_priv.txt",
				"--safedb.path":    "/data/op-node/safedb",
				"--metrics.port":   "7300",
				"--p2p.listen.tcp": "9222",
			} {
				if got := flagValue(node.Args, flag); got != want {
					t.Errorf("op-node %s = %q, want %q", flag, got, want)
				}
			}
			if !slices.ContainsFunc(node.VolumeMounts, func(m corev1.VolumeMount) bool {
				return m.Name == "data" && m.MountPath == "/data"
			}) {
				t.Error("op-node does not mount the data volume at /data")
			}
		})
	}
}

func TestOpRethFullUnlessArchive(t *testing.T) {
	adapter := adapters.MustGet(chainsv1alpha2.ChainUnichain).(adapters.ContainerArgsProvider)
	for nodeType, want := range map[chainsv1alpha2.NodeType]bool{
		chainsv1alpha2.NodeTypeRPC:     true,
		chainsv1alpha2.NodeTypeArchive: false,
	} {
		args := adapter.ContainerArgs(chainsv1alpha2.ChainInstanceSpec{NodeType: nodeType})
		if got := slices.Contains(args, "--full"); got != want {
			t.Errorf("%s: --full = %v, want %v", nodeType, got, want)
		}
	}
}

func TestOpNodeEnvFromExtraEnv(t *testing.T) {
	secret := &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "l1"}, Key: "url",
	}}
	for name, tc := range map[string]struct {
		extra []corev1.EnvVar
		want  map[string]string
	}{
		"defaults": {
			want: map[string]string{
				"OP_NODE_L1_ETH_RPC": "http://ethereum:8545",
				"OP_NODE_L1_BEACON":  "http://ethereum-beacon:5052",
			},
		},
		"overrides and passthrough": {
			extra: []corev1.EnvVar{
				{Name: "L1_RPC_URL", Value: "https://l1.example"},
				{Name: "OP_NODE_L1_BEACON", Value: "https://beacon.example"},
				{Name: "OP_NODE_P2P_ADVERTISE_IP", Value: "203.0.113.7"},
				{Name: "UNRELATED", Value: "x"},
			},
			want: map[string]string{
				"OP_NODE_L1_ETH_RPC":       "https://l1.example",
				"OP_NODE_L1_BEACON":        "https://beacon.example",
				"OP_NODE_P2P_ADVERTISE_IP": "203.0.113.7",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			node := opNodeSidecar(t, chainsv1alpha2.ChainUnichain, chainsv1alpha2.ChainInstanceSpec{ExtraEnv: tc.extra})
			got := envMap(node.Env)
			if len(got) != len(node.Env) {
				t.Errorf("duplicate env names in %v", node.Env)
			}
			if !maps.Equal(got, tc.want) {
				t.Errorf("env = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("secret", func(t *testing.T) {
		node := opNodeSidecar(t, chainsv1alpha2.ChainUnichain, chainsv1alpha2.ChainInstanceSpec{
			ExtraEnv: []corev1.EnvVar{{Name: "L1_RPC_URL", ValueFrom: secret}},
		})
		i := slices.IndexFunc(node.Env, func(e corev1.EnvVar) bool { return e.Name == "OP_NODE_L1_ETH_RPC" })
		if i < 0 || node.Env[i].ValueFrom != secret || node.Env[i].Value != "" {
			t.Errorf("L1_RPC_URL from a Secret not passed through: %+v", node.Env)
		}
	})
}

// TestNoUpstreamOpGethDefaults guards against an OP Stack chain falling back
// to upstream op-geth, which is deprecated and stops following chains at the
// Karst hardfork.
func TestNoUpstreamOpGethDefaults(t *testing.T) {
	for chain, clients := range adapters.DefaultImages() {
		for client, img := range clients {
			if strings.Contains(img, "oplabs-tools-artifacts/images/op-geth:") {
				t.Errorf("%s/%q defaults to upstream op-geth %s", chain, client, img)
			}
			if strings.Contains(img, "oplabs-tools-artifacts/images/op-reth:") {
				if _, ok := opRethChains[chain]; !ok {
					t.Errorf("%s runs op-reth but is not in opRethChains", chain)
				}
			}
		}
	}
}

// TestOpRethVersionPolicies checks that versioncheck tracks the pinned
// op-reth and op-node repositories, so it never writes a foreign tag.
func TestOpRethVersionPolicies(t *testing.T) {
	for chain := range opRethChains {
		adapter := adapters.MustGet(chain)
		policy := adapter.(adapters.VersionProvider).VersionPolicy()
		repo := policy.Registry + "/" + policy.Repository + ":"
		if img := adapters.DefaultImageFor(chain, ""); !strings.HasPrefix(img, repo) {
			t.Errorf("%s: policy tracks %q but pinned image is %q", chain, repo, img)
		}
		clients := adapter.(adapters.ClientVersionProvider).ClientVersionPolicies()
		if _, ok := clients["op-node"]; !ok {
			t.Errorf("%s: no version policy for the op-node sidecar", chain)
		}
	}
}

func TestBasePod(t *testing.T) {
	adapter := adapters.MustGet(chainsv1alpha2.ChainBase)
	spec := chainsv1alpha2.ChainInstanceSpec{Chain: chainsv1alpha2.ChainBase}
	img := adapter.DefaultImage("")
	if !strings.HasPrefix(img, "ghcr.io/base/node:") {
		t.Fatalf("main image %q is not ghcr.io/base/node", img)
	}
	if cmd := adapter.(adapters.ContainerCommandProvider).ContainerCommand(spec); !slices.Equal(cmd, []string{"/app/base-reth-node"}) {
		t.Errorf("command = %q, want base-reth-node", cmd)
	}
	args := adapter.(adapters.ContainerArgsProvider).ContainerArgs(spec)
	checkFlags(t, "base-reth-node", args[1:], opRethFlags)
	if slices.Contains(args, "--rollup.disable-tx-pool-gossip") {
		t.Error("base-reth-node has no --rollup.disable-tx-pool-gossip")
	}
	if flagValue(args, "--chain") != "base" || flagValue(args, "--datadir") != "/data/reth" {
		t.Errorf("args %q do not run --chain base on /data/reth", args)
	}

	sidecars := adapter.(adapters.SidecarProvider).Sidecars(spec)
	if len(sidecars) != 1 || sidecars[0].Name != "base-consensus" {
		t.Fatalf("sidecars = %v, want base-consensus", sidecars)
	}
	cl := sidecars[0]
	if cl.Image != img {
		t.Errorf("base-consensus image %q differs from main image %q", cl.Image, img)
	}
	for flag, want := range map[string]string{
		"--chain":                "8453",
		"--l2-engine-rpc":        "http://127.0.0.1:8551",
		"--l2-engine-jwt-secret": "/data/reth/jwt.hex",
		"--p2p.priv.path":        "/data/base-consensus/p2p_priv.txt",
	} {
		if got := flagValue(cl.Args, flag); got != want {
			t.Errorf("base-consensus %s = %q, want %q", flag, got, want)
		}
	}
	env := envMap(cl.Env)
	if env["BASE_NODE_L1_ETH_RPC"] == "" || env["BASE_NODE_L1_BEACON"] == "" {
		t.Errorf("base-consensus env %v has no L1 endpoints", env)
	}
}

// TestOpNodeIsNotAClient guards the "op-node" key in versions_gen.go, which
// only exists so versioncheck tracks the sidecar: selecting it as a client
// must not put op-node in the main container.
func TestOpNodeIsNotAClient(t *testing.T) {
	for chain := range opRethChains {
		adapter := adapters.MustGet(chain)
		for _, client := range []string{"op-node", "OP-Node"} {
			if got, want := adapter.DefaultImage(client), adapter.DefaultImage(""); got != want {
				t.Errorf("%s: client %q resolves to %q, want the op-reth image %q", chain, client, got, want)
			}
		}
	}

	node := &chainsv1alpha2.ChainInstance{Spec: chainsv1alpha2.ChainInstanceSpec{
		Chain: chainsv1alpha2.ChainUnichain, Client: "op-node",
	}}
	pod := controller.RenderPodTemplate(node, adapters.MustGet(node.Spec.Chain), "")
	for _, c := range pod.Spec.Containers {
		if c.Name == controller.MainContainerName && !strings.Contains(c.Image, "/op-reth:") {
			t.Errorf("spec.client op-node renders main image %q", c.Image)
		}
	}
}
