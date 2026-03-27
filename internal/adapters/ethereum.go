package adapters

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const (
	defaultGethImage       = "ethereum/client-go:v1.17.1"
	defaultRethImage       = "ghcr.io/paradigmxyz/reth:v1.11.3"
	defaultErigonImage     = "erigontech/erigon:v2.61.3"
	defaultNethermindImage = "nethermind/nethermind:1.36.1"
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type ethereumAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	eth := &ethereumAdapter{baseAdapter: baseAdapter{livenessPort: 8545}}
	Register(nodesv1alpha1.ChainEthereum, eth)
	Register(nodesv1alpha1.ChainEthereumArchive, eth)
}

// --------------------------------------------------------------------------
// Config templates (parsed once at init time)
// --------------------------------------------------------------------------

var ethConfigTemplates = map[string]*template.Template{
	"geth": template.Must(template.New("geth").Parse(`# Geth config for {{ .Chain }} {{ .Network }}
[Eth]
NetworkId = 1

[Node]
DataDir = "/data"

[Node.P2P]
MaxPeers = 50

[Node.HTTPTimeouts]
ReadTimeout = "30s"
`)),
	"reth": template.Must(template.New("reth").Parse(`# Reth config for {{ .Chain }} {{ .Network }}
[network]
chain = "{{ .Network }}"

[rpc]
http = true
http-addr = "0.0.0.0"
http-port = 8545
ws = true
ws-addr = "0.0.0.0"
ws-port = 8546
`)),
	"erigon": template.Must(template.New("erigon").Parse(`# Erigon config for {{ .Chain }} {{ .Network }}
[main]
chain = "{{ .Network }}"
datadir = "/data"
http = true
http.addr = "0.0.0.0"
http.port = 8545
http.vhosts = "*"
ws = true
`)),
	"nethermind": template.Must(template.New("nethermind").Parse(`{
  "Init": {
    "Network": "{{ .Network }}",
    "WebSocketsEnabled": true,
    "BaseDbPath": "/data"
  },
  "JsonRpc": {
    "Enabled": true,
    "Host": "0.0.0.0",
    "Port": 8545,
    "WebSocketsPort": 8546
  },
  "Sync": {
    "FastSync": {{ .FastSync }}
  }
}`)),
}

var ethConfigFilenames = map[string]string{
	"geth":       "config.toml",
	"reth":       "reth.toml",
	"erigon":     "erigon.yaml",
	"nethermind": "nethermind.json",
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *ethereumAdapter) DefaultImage(client string) string {
	switch strings.ToLower(client) {
	case "geth":
		// v1.15+ changed DB format (PBSS). Existing nodes must resync when upgrading from v1.14.x.
		return defaultGethImage
	case "reth":
		return defaultRethImage
	case "erigon":
		return defaultErigonImage
	default:
		return defaultNethermindImage
	}
}

func (a *ethereumAdapter) ConfigTemplate(spec nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	network := "mainnet"
	if spec.Network == nodesv1alpha1.NetworkTestnet {
		network = "sepolia"
	}

	client := strings.ToLower(spec.Client)
	tpl, ok := ethConfigTemplates[client]
	if !ok {
		tpl = ethConfigTemplates["nethermind"]
		client = "nethermind"
	}

	var buf bytes.Buffer
	err := tpl.Execute(&buf, struct {
		Chain    nodesv1alpha1.Chain
		Network  string
		FastSync bool
	}{
		Chain:    spec.Chain,
		Network:  network,
		FastSync: spec.NodeType != nodesv1alpha1.NodeTypeArchive,
	})
	if err != nil {
		return "", "", fmt.Errorf("ethereum config template (%s): %w", client, err)
	}

	return ethConfigFilenames[client], buf.String(), nil
}

func (a *ethereumAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *ethereumAdapter) LivenessProbe(spec nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	port := int32(8545)
	if spec.RPC.Port > 0 {
		port = spec.RPC.Port
	}
	return tcpProbe(port, 300, 30, 10, 3) // Reth/Erigon startup time before RPC ready
}

func (a *ethereumAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return evmPorts(30303)
}
