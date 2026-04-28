package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// TODO: moca chain is Cosmos/EVMOS-based; verify correct image before deploying.
// moca-network/moca:v0.1.0 is a placeholder until the official image is confirmed.
const defaultMocaImage = "moca-network/moca:v0.1.0"

// mocaBlockTime is the average Moca block interval used to estimate network tip.
const mocaBlockTime = 2.0 // seconds

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type mocaAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainMoca, &mocaAdapter{
		baseAdapter: baseAdapter{livenessPort: 26657},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *mocaAdapter) DefaultImage(_ string) string {
	return defaultMocaImage
}

func (a *mocaAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "app.toml", mocaConfig, nil
}

// ContainerArgs passes --home /data so the node reads from the PVC mount.
func (a *mocaAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"start", "--home", "/data"}
}

func (a *mocaAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		{Name: "api", ContainerPort: 1317, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "evm-rpc", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

// HealthCheck queries CometBFT RPC /status on port 26657.
func (a *mocaAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/status", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("moca health request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("moca health: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var status cosmosStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return SyncStatus{}, fmt.Errorf("moca status parse: %w", err)
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
			if lag > mocaBlockTime {
				highestBlock = height + int64(lag/mocaBlockTime)
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

func (a *mocaAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *mocaAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "moca-network/moca",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const mocaConfig = `# Moca Network node config (Cosmos/EVMOS-based)
[api]
enable = true
address = "tcp://0.0.0.0:1317"
enabled-unsafe-cors = true

[grpc]
enable = false

[telemetry]
enabled = true
prometheus-retention-time = 60

[json-rpc]
enable = true
address = "0.0.0.0:8545"
`
