package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const (

	// tronSyncedThreshold is the maximum number of remaining blocks (unFetchSynNum)
	// across all peers below which the node is considered fully synced.
	tronSyncedThreshold = int64(100)
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type tronAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainTRON, &tronAdapter{
		baseAdapter: baseAdapter{livenessPort: 8090},
	})
}

// --------------------------------------------------------------------------
// Config (static raw strings -- TRON HOCON configs are too complex for templates)
// --------------------------------------------------------------------------

const tronTestnetConfig = `
node.discovery.enable = true
storage.db.directory = "database"
storage.db.engine = "LEVELDB"
node.p2p.version = 201910292

net {
  type = nile
}

node {
  listen.port = 18888
  http.fullNodePort = 8090
  http.solidityPort = 8091
  rpc.port = 50051
}

seed.node = {
  ip.list = [
    "44.236.192.97:18888",
    "44.236.125.107:18888",
    "44.232.119.174:18888",
    "52.39.105.180:18888",
    "54.70.52.47:18888"
  ]
}

vm {
  maxTimeRatio = 10
  minTimeRatio = 0.0
}

block = {
  needSyncCheck = false
  maintenanceTimeInterval = 600000
  proposalExpireTime = 600000
}

genesis.block = {
  assets = [
    {
      accountName = "Zion"
      accountType = "AssetIssue"
      address = "TMWXhuxiT1KczhBxCseCDDsrhmpYGUcoA9"
      balance = "99000000000000000"
    },
    {
      accountName = "Sun"
      accountType = "AssetIssue"
      address = "TN21Wx2yoNYiZ7znuQonmZMJnH5Vdfxu78"
      balance = "99000000000000000"
    },
    {
      accountName = "Blackhole"
      accountType = "AssetIssue"
      address = "TDPJULRzVtzVjnBmZvfaTcTNQ2tsVi6XxQ"
      balance = "-9223372036854775808"
    }
  ]

  witnesses = [
    { address: TD23EqH3ixYMYh8CMXKdHyQWjePi3KQvxV,  url = "http://GR1.com",  voteCount = 100000026 },
    { address: TCm4Lz1uP3tQm3jzpwFTG6o5UvSTA2XEHc,  url = "http://GR2.com",  voteCount = 100000025 },
    { address: TTgDUgREiPBeY3iudD5e2eEibE4v4CE8C9,   url = "http://GR3.com",  voteCount = 100000024 },
    { address: TFVDe7kMEmb8EuUxxp42kocQY1fFY727WS,   url = "http://GR4.com",  voteCount = 100000023 },
    { address: TY4NSjctzTchHkhaCskVc5zQtnX9s1uxAX,   url = "http://GR5.com",  voteCount = 100000022 },
    { address: TWSMPrm6aizvsJmPnjMB7x3UExJfRhyQhd,   url = "http://GR6.com",  voteCount = 100000021 },
    { address: TKwLkSaCvqqpAB44qaHGTohCTCFoYw7ecy,   url = "http://GR7.com",  voteCount = 100000020 },
    { address: TDsYmm1St9r4UZebDGWBcTMtfYTw9YX5h4,   url = "http://GR8.com",  voteCount = 100000019 },
    { address: TFEQbWAPxhbUr1P14y9UJBUZo3LgtdqTS7,   url = "http://GR9.com",  voteCount = 100000018 },
    { address: TCynAi8tb7UWP7uhLv6fe971KLm2KT8tcs,   url = "http://GR10.com", voteCount = 100000017 },
    { address: TC2YsLp4rzrt3AbeN3EryoSywrBjEUVCq3,   url = "http://GR11.com", voteCount = 100000016 },
    { address: THxMKH1uaL5FpURujkQR7u2sNZ2n5PSsiH,   url = "http://GR12.com", voteCount = 100000015 },
    { address: TWbzgoHimDcXWy19ts1An8bxA4JKjcYHeG,   url = "http://GR13.com", voteCount = 100000014 },
    { address: TW2LmXnVCEaxuVtQN8gZR1ixT5PNm4QLft,   url = "http://GR14.com", voteCount = 100000013 },
    { address: TVuqk4rYYVHVA6j6sSEnaLexhhoQhN8nyZ,   url = "http://GR15.com", voteCount = 100000012 },
    { address: TVMZu5ptZPhhkZ3Kaagkq35FmyuKNvUKJV,   url = "http://GR16.com", voteCount = 100000011 },
    { address: TFDHT8PqUrL2Bd8DeysSiHHBAEMidZgkhx,   url = "http://GR17.com", voteCount = 100000010 },
    { address: TVqz5Bj3M1uEenaSsw2NnXvTWChPj6K3hb,   url = "http://GR18.com", voteCount = 100000009 },
    { address: TSt8YNpARJkhdMdEV4C7ajH1tFHpZWzF1T,   url = "http://GR19.com", voteCount = 100000008 },
    { address: TTxWDjEb3Be1Ax8BCvK48cnaorZofLq2C9,   url = "http://GR20.com", voteCount = 100000007 },
    { address: TU5T838YtyZtEQKpnXEdRz3d8hJn6WHhjw,   url = "http://GR21.com", voteCount = 100000006 },
    { address: TRuSs1MpL3o2hzhU8r6HLC7WtDyVE9hsF6,   url = "http://GR22.com", voteCount = 100000005 },
    { address: TYMCoCZyAjWkWdUfEHg1oZQYbLKev282ou,   url = "http://GR23.com", voteCount = 100000004 },
    { address: TQvAyGATpLZymHbpeaRozJCKqSeRWVNhCJ,   url = "http://GR24.com", voteCount = 100000003 },
    { address: TYDd9nskbhJmLLNoe4yV2Z1SYtGjNa8wyg,   url = "http://GR25.com", voteCount = 100000002 },
    { address: TS5991Geh2qeHtw46rskpJyn6hFNbuZGGc,   url = "http://GR26.com", voteCount = 100000001 },
    { address: TKnn5MBnmXXeKdu9dxKVfKk4n1YdCeSRGr,   url = "http://GR27.com", voteCount = 100000000 }
  ]

  timestamp = "0"
  parentHash = "0xe58f33f9baf9305dc6f82b9f1934ea8f0ade2defb951258d50167028c780351f"
}
`

const tronMainnetConfig = `
node.discovery.enable = true
storage.db.directory = "database"
storage.db.engine = "LEVELDB"
storage.txCache.initOptimization = true
node.p2p.version = 11111

net {
  type = mainnet
}

node {
  listen.port = 18888
  http.fullNodePort = 8090
  http.solidityPort = 8091
  rpc.port = 50051
}

seed.node = {
  ip.list = [
    "3.225.171.164:18888",
    "52.53.189.99:18888",
    "18.196.99.16:18888",
    "34.253.187.192:18888",
    "18.133.82.227:18888",
    "35.180.51.163:18888",
    "54.252.224.209:18888",
    "18.228.15.36:18888",
    "52.15.93.92:18888",
    "34.220.77.106:18888"
  ]
}

vm {
  # Increase VM execution timeout multiplier to handle CPU-heavy smart contract
  # transactions during historical sync. Default is 5.0; some blocks (e.g. 81012608)
  # contain transactions that OOT at 5x but succeed at 50x on JDK 11 / G1GC.
  # Block 81017703 OOTs at 50x — raised to 100x.
  maxTimeRatio = 100
  minTimeRatio = 0.0
}

genesis.block = {
  assets = [
    {
      accountName = "Zion"
      accountType = "AssetIssue"
      address = "TLLM21wteSPs4hKjbxgmH1L6poyMjeTbHm"
      balance = "99000000000000000"
    },
    {
      accountName = "Sun"
      accountType = "AssetIssue"
      address = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"
      balance = "0"
    },
    {
      accountName = "Blackhole"
      accountType = "AssetIssue"
      address = "TLsV52sRDL79HXGGm9yzwKibb6BeruhUzy"
      balance = "-9223372036854775808"
    }
  ]

  witnesses = []

  timestamp = "0"
  parentHash = "0xe58f33f9baf9305dc6f82b9f1934ea8f0ade2defb951258d50167028c780351f"
}
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *tronAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainTRON, client)
}

func (a *tronAdapter) ConfigTemplate(spec nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	if spec.Network == nodesv1alpha1.NetworkTestnet {
		return "config.conf", tronTestnetConfig, nil
	}
	return "config.conf", tronMainnetConfig, nil
}

func (a *tronAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/wallet/getnodeinfo", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("tron nodeinfo request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("tron nodeinfo: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var nodeInfo struct {
		Block string `json:"block"` // "Num:80841151,ID:..."
		Peers []struct {
			UnFetchSynNum      int64 `json:"unFetchSynNum"`
			NeedSyncFrom       bool  `json:"needSyncFromPeer"`
			SyncToFetchSize    int64 `json:"syncToFetchSize"`
			SyncToFetchPeekNum int64 `json:"syncToFetchSizePeekNum"`
			BlockInPorcSize    int64 `json:"blockInPorcSize"`
		} `json:"peerList"`
	}
	if err := json.Unmarshal(body, &nodeInfo); err != nil {
		return SyncStatus{}, fmt.Errorf("tron nodeinfo parse: %w", err)
	}

	// Parse current block number from "Num:NNNN,ID:..." format.
	var currentBlock int64
	if _, err := fmt.Sscanf(nodeInfo.Block, "Num:%d,", &currentBlock); err != nil {
		return SyncStatus{IsSyncing: true, Progress: 0}, nil
	}

	// Derive sync state from peer data.
	var maxRemain, highestFromPeers int64
	var activeDownload bool
	for _, p := range nodeInfo.Peers {
		if !p.NeedSyncFrom {
			continue
		}
		if p.UnFetchSynNum > maxRemain {
			maxRemain = p.UnFetchSynNum
		}
		if peerHighest := p.SyncToFetchPeekNum + p.SyncToFetchSize; peerHighest > highestFromPeers {
			highestFromPeers = peerHighest
		}
		if p.SyncToFetchSize > 0 || p.BlockInPorcSize > 0 {
			activeDownload = true
		}
	}

	highestBlock := max(currentBlock+maxRemain, highestFromPeers)
	lag := highestBlock - currentBlock
	isSyncing := lag > tronSyncedThreshold
	stallExempt := activeDownload && isSyncing

	progress := 100.0
	if highestBlock > 0 && isSyncing {
		progress = float64(currentBlock) / float64(highestBlock) * 100.0
	}

	return SyncStatus{
		IsSyncing:    isSyncing,
		CurrentBlock: currentBlock,
		HighestBlock: highestBlock,
		Progress:     progress,
		Peers:        int32(len(nodeInfo.Peers)),
		StallExempt:  stallExempt,
	}, nil
}

// StartupProbe gives TRON mainnet up to 6h to complete loadTransForLiteNode.
// Testnet (Nile) syncs from genesis: shorter window.
func (a *tronAdapter) StartupProbe(spec nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	failureThreshold := int32(720) // mainnet: 720 x 30s = 6h
	if spec.Network == nodesv1alpha1.NetworkTestnet {
		failureThreshold = 40 // testnet: 40 x 30s = 20min
	}
	return tcpProbe(8090, 60, 30, 10, failureThreshold)
}

// ContainerCommand bypasses the FullNode shell script which reads JDK 8-only
// vmoptions (CMS GC flags). We run on JDK 11 to avoid the SIGSEGV bug in all
// JDK 8 variants on Linux 6.x kernels.
func (a *tronAdapter) ContainerCommand(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"/bin/sh", "-c",
		`CP=$(find /java-tron/lib -name '*.jar' -printf '%p:' | sed 's/:$//') && ` +
			`exec java $JAVA_OPTS -Djava.specification.version=1.8 ` +
			`-XX:+UseG1GC -XX:-UseCompressedOops -XX:ReservedCodeCacheSize=256m ` +
			`-verbose:gc -Xloggc:./gc.log -XX:+UseGCLogFileRotation ` +
			`-XX:NumberOfGCLogFiles=5 -XX:GCLogFileSize=20M ` +
			`-classpath "$CP" org.tron.program.FullNode "$@"`,
		"--"}
}

func (a *tronAdapter) ContainerArgs(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"-c", "/config/config.conf", "-d", "/data"}
}

func (a *tronAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "http", ContainerPort: 8090, Protocol: corev1.ProtocolTCP},
		{Name: "grpc", ContainerPort: 50051, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 18888, Protocol: corev1.ProtocolTCP},
		// Java-Tron exposes Prometheus metrics on port 9527 by default.
		{Name: "metrics", ContainerPort: 9527, Protocol: corev1.ProtocolTCP},
	}
}

func (a *tronAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("8"),
		MemoryRequest: resource.MustParse("16Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *tronAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "tronprotocol/java-tron",
		TagPattern: `^GreatVoyage-v\d+`,
		TagPrefix:  "GreatVoyage-",
	}
}
