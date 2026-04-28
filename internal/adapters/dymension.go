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

const (
	defaultDymensionImage = "dymensionxyz/dymd:v3.1.0"

	// dymensionBlockTime is the average Dymension block interval used to estimate network tip.
	dymensionBlockTime = 6.0 // seconds
)

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const dymensionConfig = `# Dymension modular rollup hub node config override
[api]
enable = true
address = "tcp://0.0.0.0:1317"
enabled-unsafe-cors = true

[grpc]
enable = true
address = "0.0.0.0:9090"

[telemetry]
enabled = true
prometheus-retention-time = 60
`

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type dymensionAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainDymension, &dymensionAdapter{
		baseAdapter: baseAdapter{livenessPort: 26657},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *dymensionAdapter) DefaultImage(_ string) string {
	return defaultDymensionImage
}

func (a *dymensionAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "app.toml", dymensionConfig, nil
}

// ContainerArgs passes --home /data so the node reads from the PVC mount.
func (a *dymensionAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"start", "--home", "/data"}
}

func (a *dymensionAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 26657, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 26656, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 26656, Protocol: corev1.ProtocolUDP},
		{Name: "grpc", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: 26660, Protocol: corev1.ProtocolTCP},
	}
}

// HealthCheck queries CometBFT RPC /status on port 26657.
func (a *dymensionAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/status", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("dymension health request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("dymension health: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var status cosmosStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return SyncStatus{}, fmt.Errorf("dymension status parse: %w", err)
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
			if lag > dymensionBlockTime {
				highestBlock = height + int64(lag/dymensionBlockTime)
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

func (a *dymensionAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("300Gi"),
	}
}

func (a *dymensionAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "dymensionxyz/dymd",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}
