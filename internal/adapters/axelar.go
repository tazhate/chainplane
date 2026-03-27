package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const (
	defaultAxelarImage = "axelarnet/axelar-core:v1.3.4"

	// axelarBlockTime is the average Axelar block interval used to estimate network tip.
	axelarBlockTime = 6.0 // seconds
)

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const axelarConfig = `# Axelar cross-chain gateway protocol node config override
[api]
enable = true
address = "tcp://0.0.0.0:1317"
enabled-unsafe-cors = true

[grpc]
enable = true
address = "0.0.0.0:9090"

[telemetry]
enabled = false
`

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type axelarAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainAxelar, &axelarAdapter{
		baseAdapter: baseAdapter{livenessPort: 26657},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *axelarAdapter) DefaultImage(_ string) string {
	return defaultAxelarImage
}

func (a *axelarAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "app.toml", axelarConfig, nil
}

// ContainerArgs passes --home /data so the node reads from the PVC mount.
func (a *axelarAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"start", "--home", "/data"}
}

func (a *axelarAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 26656, Protocol: corev1.ProtocolUDP},
		{Name: "grpc", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
	}
}

// HealthCheck queries CometBFT RPC /status on port 26657.
func (a *axelarAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/status", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("axelar health request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("axelar health: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var status cosmosStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return SyncStatus{}, fmt.Errorf("axelar status parse: %w", err)
	}

	height := parseDecimalInt64(status.Result.SyncInfo.LatestBlockHeight)
	syncing := status.Result.SyncInfo.CatchingUp

	if height == 0 && syncing {
		return syncingPseudo(), nil
	}

	highestBlock := height
	if syncing {
		if blockTime, err := time.Parse(time.RFC3339Nano, status.Result.SyncInfo.LatestBlockTime); err == nil {
			lag := time.Since(blockTime).Seconds()
			if lag > axelarBlockTime {
				highestBlock = height + int64(lag/axelarBlockTime)
			}
		}
		if tip := cosmosNetworkTip(ctx, rpcURL); tip > highestBlock {
			highestBlock = tip
		}
	}

	var progress float64
	if !syncing {
		progress = 100.0
	} else {
		progress = progressFromBlocks(height, highestBlock)
	}

	var peers int32
	if req2, err2 := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/net_info", nil); err2 == nil {
		if resp2, err2 := (&http.Client{Timeout: 5 * time.Second}).Do(req2); err2 == nil {
			defer resp2.Body.Close()
			body2, _ := io.ReadAll(io.LimitReader(resp2.Body, maxResponseBytes))
			var netInfo struct {
				Result struct {
					NPeers string `json:"n_peers"`
				} `json:"result"`
			}
			if json.Unmarshal(body2, &netInfo) == nil {
				peers = int32(parseDecimalInt64(netInfo.Result.NPeers))
			}
		}
	}

	return SyncStatus{
		IsSyncing:    syncing,
		CurrentBlock: height,
		HighestBlock: highestBlock,
		Progress:     progress,
		Peers:        peers,
	}, nil
}
