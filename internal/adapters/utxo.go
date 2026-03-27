package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// utxoAdapter is the shared base for Bitcoin-family chains (Bitcoin, Litecoin, Dash).
// It provides common getblockchaininfo-based health checking and RPC auth handling.
type utxoAdapter struct {
	baseAdapter
	rpcUserEnv     string // env var name for RPC user (e.g. "BTC_RPC_USER")
	rpcPasswordEnv string // env var name for RPC password
	defaultUser    string
	defaultPass    string
	useRetry       bool // whether to use callRPCWithRetry
}

// rpcCredentials reads RPC credentials from environment with defaults.
func (u *utxoAdapter) rpcCredentials() (user, pass string) {
	user = os.Getenv(u.rpcUserEnv)
	if user == "" {
		user = u.defaultUser
	}
	pass = os.Getenv(u.rpcPasswordEnv)
	if pass == "" {
		pass = u.defaultPass
	}
	return user, pass
}

// authenticatedURL injects RPC credentials into the given URL.
func (u *utxoAdapter) authenticatedURL(rpcURL string) (string, error) {
	parsed, err := url.Parse(rpcURL)
	if err != nil {
		return "", fmt.Errorf("parse rpc url: %w", err)
	}
	user, pass := u.rpcCredentials()
	parsed.User = url.UserPassword(user, pass)
	return parsed.String(), nil
}

// blockchainInfo holds the common fields from getblockchaininfo across UTXO chains.
type blockchainInfo struct {
	Blocks               int64   `json:"blocks"`
	Headers              int64   `json:"headers"`
	VerificationProgress float64 `json:"verificationprogress"`
}

// utxoHealthCheck performs a standard UTXO-chain health check via getblockchaininfo.
// stallPolicy determines how StallExempt is set based on sync state:
//   - "synced-exempt": exempt when fully synced (BTC, DASH)
//   - "ibd-exempt":    exempt during initial block download at <95% (LTC)
func utxoHealthCheck(ctx context.Context, rpcURL string, u *utxoAdapter, stallPolicy string) (SyncStatus, error) {
	authURL, err := u.authenticatedURL(rpcURL)
	if err != nil {
		return SyncStatus{}, err
	}

	var result json.RawMessage
	if u.useRetry {
		result, err = callRPCWithRetry(ctx, authURL, "getblockchaininfo", nil, 2)
	} else {
		result, err = callRPC(ctx, authURL, "getblockchaininfo", nil)
	}
	if err != nil {
		if u.useRetry && isTransientRPCError(err) {
			return SyncStatus{IsSyncing: true, StallExempt: true, Progress: 0}, nil
		}
		return SyncStatus{}, fmt.Errorf("getblockchaininfo: %w", err)
	}

	var info blockchainInfo
	if err := json.Unmarshal(result, &info); err != nil {
		return SyncStatus{}, fmt.Errorf("parse getblockchaininfo: %w", err)
	}

	var peers int32
	if peerResult, err2 := callRPC(ctx, authURL, "getconnectioncount", nil); err2 == nil {
		_ = json.Unmarshal(peerResult, &peers)
	}

	isSyncing := info.VerificationProgress < 0.999

	var stallExempt bool
	switch stallPolicy {
	case "synced-exempt":
		// Fully-synced nodes: block intervals are long (BTC ~10min, DASH ~2.5min).
		stallExempt = !isSyncing
	case "ibd-exempt":
		// During IBD, height can freeze for >25 min while validating large batches.
		stallExempt = isSyncing && info.VerificationProgress < 0.95
	}

	return SyncStatus{
		IsSyncing:    isSyncing,
		CurrentBlock: info.Blocks,
		HighestBlock: info.Headers,
		Progress:     info.VerificationProgress * 100.0,
		Peers:        peers,
		StallExempt:  stallExempt,
	}, nil
}

// utxoConfigValues returns the testnet flag and RPC credentials for template rendering.
func utxoConfigValues(spec nodesv1alpha1.BlockchainNodeSpec, u *utxoAdapter) (testnet string, rpcUser string, rpcPassword string) {
	testnet = "0"
	if spec.Network == nodesv1alpha1.NetworkTestnet {
		testnet = "1"
	}
	rpcUser, rpcPassword = u.rpcCredentials()
	return testnet, rpcUser, rpcPassword
}
