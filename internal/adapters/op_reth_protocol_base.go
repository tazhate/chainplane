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
	"context"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const (
	opRethHTTPPort    = 8545
	opRethWSPort      = 8546
	opRethP2PPort     = 30303
	opRethMetricsPort = 9001
	opRethAuthPort    = 8551

	opNodeRPCPort     = 9545
	opNodeP2PPort     = 9222
	opNodeMetricsPort = 7300

	// op-reth and op-node keep their state in subdirectories of the data
	// volume. op-reth writes the engine API secret to its datadir on first
	// start; op-node waits for that file and reads it.
	opRethDataDir = "/data/reth"
	opNodeDataDir = "/data/op-node"
	opRethJWTPath = opRethDataDir + "/jwt.hex"

	// opNodeClient keys the op-node sidecar image in versions_gen.go, so
	// versioncheck tracks it next to op-reth. It is not a selectable client.
	opNodeClient = "op-node"

	// L1 endpoints op-node uses unless spec.extraEnv sets L1_RPC_URL or
	// L1_BEACON_URL.
	defaultOpStackL1URL       = "http://ethereum:8545"
	defaultOpStackL1BeaconURL = "http://ethereum-beacon:5052"
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

// opRethProtocolAdapter is the shared base for OP Stack chains in the
// superchain registry bundled with op-reth and op-node. The pod runs op-reth
// as the main container and op-node as a sidecar that drives it over the
// engine API on 127.0.0.1 and syncs it from L2 peers (execution-layer sync).
type opRethProtocolAdapter struct {
	protocolAdapter
	chain chainsv1alpha2.Chain
	// rethChain is the op-reth --chain name, nodeNetwork the op-node --network.
	rethChain   string
	nodeNetwork string
	// sequencerURL receives eth_sendRawTransaction, as the node does not
	// gossip transactions.
	sequencerURL string
}

func newOpRethProtocolAdapter(chain chainsv1alpha2.Chain, rethChain, nodeNetwork, sequencerURL string) opRethProtocolAdapter {
	return opRethProtocolAdapter{
		protocolAdapter: protocolAdapter{livenessPort: opRethHTTPPort},
		chain:           chain,
		rethChain:       rethChain,
		nodeNetwork:     nodeNetwork,
		sequencerURL:    sequencerURL,
	}
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

// DefaultImage resolves the main container image. The opNodeClient entry is
// the sidecar image, not a client: spec.client "op-node" falls back to
// op-reth instead of putting op-node in the main container.
func (b opRethProtocolAdapter) DefaultImage(client string) string {
	if strings.EqualFold(client, opNodeClient) {
		client = ""
	}
	return DefaultImageFor(b.chain, client)
}

// ConfigTemplate returns no file: op-reth and op-node are configured by flags.
func (b opRethProtocolAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "", "", nil
}

func (b opRethProtocolAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (b opRethProtocolAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return append(evmPorts(opRethP2PPort), corev1.ContainerPort{
		Name: "metrics", ContainerPort: opRethMetricsPort, Protocol: corev1.ProtocolTCP,
	})
}

// ContainerArgs runs op-reth on its built-in chain spec without transaction
// gossip; transactions go to the sequencer.
func (b opRethProtocolAdapter) ContainerArgs(spec chainsv1alpha2.ChainInstanceSpec) []string {
	return append(rethNodeArgs(b.rethChain, b.sequencerURL, spec), "--rollup.disable-tx-pool-gossip")
}

// rethNodeArgs returns the `node` flags shared by op-reth and Base's
// base-reth-node. The engine API stays on loopback for the consensus
// sidecar, which reads the secret reth writes to its datadir. Archive nodes
// keep all state, the rest run with --full.
func rethNodeArgs(chain, sequencerURL string, spec chainsv1alpha2.ChainInstanceSpec) []string {
	args := []string{
		"node",
		"--chain", chain,
		"--datadir", opRethDataDir,
		"--http", "--http.addr", "0.0.0.0", "--http.port", strconv.Itoa(opRethHTTPPort),
		"--http.api", "eth,net,web3,txpool,debug", "--http.corsdomain", "*",
		"--ws", "--ws.addr", "0.0.0.0", "--ws.port", strconv.Itoa(opRethWSPort),
		"--ws.api", "eth,net,web3", "--ws.origins", "*",
		"--authrpc.addr", "127.0.0.1", "--authrpc.port", strconv.Itoa(opRethAuthPort),
		"--port", strconv.Itoa(opRethP2PPort), "--discovery.port", strconv.Itoa(opRethP2PPort),
		"--metrics", "0.0.0.0:" + strconv.Itoa(opRethMetricsPort),
		"--rollup.sequencer", sequencerURL,
	}
	if spec.NodeType != chainsv1alpha2.NodeTypeArchive {
		args = append(args, "--full")
	}
	return args
}

// waitForEngineSecret returns a sidecar command that starts binary once reth
// has written the engine API secret, after creating dataDir for its state.
func waitForEngineSecret(binary, dataDir string) []string {
	return []string{"sh", "-c", `until [ -s ` + opRethJWTPath + ` ]; do sleep 1; done
mkdir -p ` + dataDir + `
exec ` + binary + ` "$@"`, "--"}
}

// Sidecars adds op-node. It starts once op-reth has written the engine API
// secret, then follows the chain with execution-layer sync. L1 endpoints
// come from L1_RPC_URL and L1_BEACON_URL in spec.extraEnv (which otherwise
// only reach the main container), and any OP_NODE_* entry there is passed
// through, so op-node flags can be tuned without a new adapter release.
func (b opRethProtocolAdapter) Sidecars(spec chainsv1alpha2.ChainInstanceSpec) []corev1.Container {
	return []corev1.Container{{
		Name:    "op-node",
		Image:   DefaultImageFor(b.chain, opNodeClient),
		Command: waitForEngineSecret("op-node", opNodeDataDir),
		Args: []string{
			"--network", b.nodeNetwork,
			"--l2", "http://127.0.0.1:" + strconv.Itoa(opRethAuthPort),
			"--l2.jwt-secret", opRethJWTPath,
			"--l2.enginekind", "reth",
			"--syncmode", "execution-layer",
			"--rpc.addr", "0.0.0.0", "--rpc.port", strconv.Itoa(opNodeRPCPort),
			"--metrics.enabled", "--metrics.addr", "0.0.0.0", "--metrics.port", strconv.Itoa(opNodeMetricsPort),
			"--p2p.listen.tcp", strconv.Itoa(opNodeP2PPort), "--p2p.listen.udp", strconv.Itoa(opNodeP2PPort),
			"--p2p.priv.path", opNodeDataDir + "/p2p_priv.txt",
			"--p2p.peerstore.path", opNodeDataDir + "/peerstore",
			"--p2p.discovery.path", opNodeDataDir + "/discovery",
			"--safedb.path", opNodeDataDir + "/safedb",
		},
		Env: l1Env(spec.ExtraEnv, "OP_NODE_L1_ETH_RPC", "OP_NODE_L1_BEACON", "OP_NODE_"),
		Ports: []corev1.ContainerPort{
			{Name: "opnode-rpc", ContainerPort: opNodeRPCPort, Protocol: corev1.ProtocolTCP},
			{Name: "opnode-p2p-tcp", ContainerPort: opNodeP2PPort, Protocol: corev1.ProtocolTCP},
			{Name: "opnode-p2p-udp", ContainerPort: opNodeP2PPort, Protocol: corev1.ProtocolUDP},
			{Name: "opnode-metrics", ContainerPort: opNodeMetricsPort, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
	}}
}

// l1Env sets the consensus client's L1 endpoint variables rpcName and
// beaconName from L1_RPC_URL and L1_BEACON_URL in extraEnv (or from rpcName
// and beaconName themselves), falling back to the in-cluster defaults, and
// appends other extraEnv entries starting with prefix unchanged. Value and
// ValueFrom are both kept, so the URLs may come from a Secret.
func l1Env(extra []corev1.EnvVar, rpcName, beaconName, prefix string) []corev1.EnvVar {
	l1 := corev1.EnvVar{Name: rpcName, Value: defaultOpStackL1URL}
	beacon := corev1.EnvVar{Name: beaconName, Value: defaultOpStackL1BeaconURL}
	var passthrough []corev1.EnvVar
	for _, e := range extra {
		switch e.Name {
		case "L1_RPC_URL", rpcName:
			l1.Value, l1.ValueFrom = e.Value, e.ValueFrom
		case "L1_BEACON_URL", beaconName:
			beacon.Value, beacon.ValueFrom = e.Value, e.ValueFrom
		default:
			if strings.HasPrefix(e.Name, prefix) {
				passthrough = append(passthrough, e)
			}
		}
	}
	return append([]corev1.EnvVar{l1, beacon}, passthrough...)
}

// VersionPolicy tracks op-reth. ClientVersionPolicies tracks the op-node
// sidecar under the opNodeClient key.
func (b opRethProtocolAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "us-docker.pkg.dev",
		Repository: "oplabs-tools-artifacts/images/op-reth",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

func (b opRethProtocolAdapter) ClientVersionPolicies() map[string]ChainVersionPolicy {
	return map[string]ChainVersionPolicy{
		opNodeClient: {
			Registry:   "us-docker.pkg.dev",
			Repository: "oplabs-tools-artifacts/images/op-node",
			TagPattern: `^v\d+\.\d+\.\d+$`,
		},
	}
}
