package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// --------------------------------------------------------------------------
// Shared EVM health check logic
// --------------------------------------------------------------------------

// hexToInt64 converts a 0x-prefixed hex string to int64.
func hexToInt64(hex string) int64 {
	hex = strings.TrimPrefix(hex, "0x")
	if hex == "" {
		return 0
	}
	var n int64
	_, _ = fmt.Sscanf(hex, "%x", &n)
	return n
}

// evmHealthCheck performs a standard Ethereum-compatible health check via
// eth_syncing, eth_blockNumber, and net_peerCount. Used by Ethereum, BSC,
// Polygon, and Avalanche (C-Chain) adapters.
func evmHealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	// eth_syncing
	syncResult, err := callRPC(ctx, rpcURL, "eth_syncing", nil)
	if err != nil {
		// RPC timeout during heavy stage work (SenderRecovery, Execution, etc.) is
		// normal for Reth/Erigon and must not trigger a Degraded restart.
		if isContextTimeout(err) {
			return SyncStatus{IsSyncing: true, StallExempt: true}, nil
		}
		return SyncStatus{}, fmt.Errorf("eth_syncing: %w", err)
	}

	// eth_blockNumber
	blockResult, err := callRPC(ctx, rpcURL, "eth_blockNumber", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("eth_blockNumber: %w", err)
	}

	var blockHex string
	if err := json.Unmarshal(blockResult, &blockHex); err != nil {
		return SyncStatus{}, fmt.Errorf("parse eth_blockNumber: %w", err)
	}
	currentBlock := hexToInt64(blockHex)

	peers := evmPeerCount(ctx, rpcURL)

	// eth_syncing returns false when synced, or an object when syncing
	var syncingFalse bool
	if err := json.Unmarshal(syncResult, &syncingFalse); err == nil && !syncingFalse {
		return SyncStatus{
			IsSyncing:    false,
			CurrentBlock: currentBlock,
			HighestBlock: currentBlock,
			Progress:     100.0,
			Peers:        peers,
		}, nil
	}

	// Parse syncing object (Erigon includes a stages array)
	var syncObj struct {
		CurrentBlock string `json:"currentBlock"`
		HighestBlock string `json:"highestBlock"`
		Stages       []struct {
			StageName   string `json:"name"`
			BlockNumber string `json:"block"`
		} `json:"stages"`
	}
	if err := json.Unmarshal(syncResult, &syncObj); err != nil {
		return SyncStatus{}, fmt.Errorf("parse eth_syncing object: %w", err)
	}

	// Polygon-specific stages that are always 0 and must be excluded.
	polygonStages := map[string]bool{"BorHeimdall": true, "Translation": true}

	var current, highest int64
	if len(syncObj.Stages) > 0 {
		// Erigon: derive current from the minimum non-zero stage block
		// (the bottleneck stage), and highest from the maximum (chain tip).
		current = -1 // sentinel
		for _, s := range syncObj.Stages {
			if polygonStages[s.StageName] {
				continue
			}
			n := hexToInt64(s.BlockNumber)
			if n > highest {
				highest = n
			}
			if n > 0 && (current < 0 || n < current) {
				current = n
			}
		}
		if current < 0 {
			current = 0
		}
	} else {
		// Non-Erigon clients (geth, reth, nethermind): use standard fields.
		current = currentBlock
		highest = hexToInt64(syncObj.HighestBlock)
	}

	progress := progressFromBlocks(current, highest)

	// Reth and other pipelined clients process historical stages without advancing
	// eth_blockNumber for hours. Exempt from height-freeze check; liveness TCP probe
	// handles truly stuck nodes.
	return SyncStatus{
		IsSyncing:    true,
		CurrentBlock: current,
		HighestBlock: highest,
		Progress:     progress,
		Peers:        peers,
		StallExempt:  true,
	}, nil
}

// evmPeerCount calls net_peerCount and returns the result. Best-effort: returns 0 on error.
func evmPeerCount(ctx context.Context, rpcURL string) int32 {
	peerResult, err := callRPC(ctx, rpcURL, "net_peerCount", nil)
	if err != nil {
		return 0
	}
	var hexStr string
	if json.Unmarshal(peerResult, &hexStr) == nil {
		return int32(hexToInt64(hexStr))
	}
	return 0
}

// evmPorts returns the standard EVM container port set (RPC, WS, P2P TCP/UDP).
func evmPorts(p2pPort int32) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 8546, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: p2pPort, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: p2pPort, Protocol: corev1.ProtocolUDP},
	}
}
