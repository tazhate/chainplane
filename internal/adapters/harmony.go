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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type harmonyAdapter struct {
	protocolAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(chainsv1alpha2.ChainHarmony, &harmonyAdapter{
		protocolAdapter: protocolAdapter{livenessPort: 9500},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *harmonyAdapter) DefaultImage(client string) string {
	return DefaultImageFor(chainsv1alpha2.ChainHarmony, client)
}

func (a *harmonyAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "harmony.conf", harmonyConfig, nil
}

// ContainerArgs replaces the image Cmd ("harmony -c harmony.conf", relative
// to WorkingDir /harmony) so the node reads the mounted config. The image
// entrypoint "tini --" stays in place.
func (a *harmonyAdapter) ContainerArgs(_ chainsv1alpha2.ChainInstanceSpec) []string {
	return []string{"harmony", "-c", "/config/harmony.conf"}
}

func (a *harmonyAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *harmonyAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 9500, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: 9800, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 9000, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 9000, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 9900, Protocol: corev1.ProtocolTCP},
	}
}

func (a *harmonyAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("2000Gi"),
	}
}

func (a *harmonyAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "harmonyone/harmony",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const harmonyConfig = `# Harmony ONE mainnet RPC (explorer) node, shard 0.
# Full output of "harmony config dump" from v2026.1.3 with the RPC, data
# and metrics settings changed. Keep it complete: harmony zero-fills missing
# sections (an absent [Sync] disables sync, an absent [Network] fails with
# "unknown network type"), and an older Version asks to migrate on stdin.
Version = "2.6.8"

[BLSKeys]
  KMSConfigFile = ""
  KMSConfigSrcType = "shared"
  KMSEnabled = false
  KeyDir = "./.hmy/blskeys"
  KeyFiles = []
  MaxKeys = 11
  PassEnabled = true
  PassFile = ""
  PassSrcType = "auto"
  SavePassphrase = false

[Cache]
  Disabled = true
  Preimages = true
  SnapshotLimit = 0
  SnapshotNoBuild = false
  SnapshotWait = true
  TrieNodeLimit = 256
  TrieTimeLimit = "2m0s"
  TriesInMemory = 128

# Not in the dump (default MinPeers 6). harmony exits when fewer peers connect
# within 60s of start; an explorer node does not vote, so one peer is enough.
[Consensus]
  AggregateSig = true
  MinPeers = 1

[DNSSync]
  Client = true
  Port = 6000
  Server = true
  ServerPort = 6000
  Zone = "t.hmny.io"

[GPO]
  BlockGasLimit = 0
  Blocks = 20
  DefaultPrice = 100000000000
  LowUsageThreshold = 50
  MaxPrice = 1000000000000
  Percentile = 60
  Transactions = 3

[General]
  DataDir = "/data"
  EnablePruneBeaconChain = false
  IsArchival = false
  IsBackup = false
  IsBeaconArchival = false
  IsOffline = false
  NoStaking = true
  NodeType = "explorer"
  RunElasticMode = false
  ShardID = 0
  TraceEnable = false

[HTTP]
  AuthPort = 9501
  Enabled = true
  IP = "0.0.0.0"
  IdleTimeout = "120s"
  Port = 9500
  ReadTimeout = "30s"
  RosettaEnabled = false
  RosettaPort = 9700
  WriteTimeout = "30s"

[Localnet]
  BlocksPerEpoch = 16
  BlocksPerEpochV2 = 16

[Log]
  Console = true
  FileName = "harmony.log"
  Folder = "./latest"
  RotateCount = 0
  RotateMaxAge = 0
  RotateSize = 100
  Verbosity = 3

  [Log.VerbosePrints]
    Config = true

[Network]
  BootNodes = ["/dnsaddr/bootstrap.t.hmny.io"]
  NetworkType = "mainnet"

[P2P]
  ConnManagerHighWatermark = 192
  ConnManagerLowWatermark = 160
  DialTimeout = "1m0s"
  DisablePrivateIPScan = false
  DiscConcurrency = 0
  IP = "0.0.0.0"
  KeyFile = "/data/.hmykey"
  MaxConnsPerIP = 10
  MaxPeers = 0
  Muxer = "yamux, mplexC6"
  NAT = true
  NoRelay = true
  NoTransportSecurity = false
  Port = 9000
  ResourceMgrEnabled = false
  ResourceMgrFileDescriptorsLimit = 0
  ResourceMgrMemoryLimitBytes = 0
  UserAgent = ""
  WaitForEachPeerToConnect = false

[Prometheus]
  Enabled = true
  EnablePush = false
  Gateway = "https://gateway.harmony.one"
  IP = "0.0.0.0"
  Port = 9900

[Pprof]
  Enabled = false
  Folder = "./profiles"
  ListenAddr = "127.0.0.1:6060"
  ProfileDebugValues = [0]
  ProfileIntervals = [600]
  ProfileNames = []

[RPCOpt]
  DebugEnabled = false
  EthRPCsEnabled = true
  EvmCallTimeout = "5s"
  LegacyRPCsEnabled = true
  PreimagesEnabled = false
  RateLimterEnabled = true
  RequestsPerSecond = 1000
  RpcFilterFile = "./.hmy/rpc_filter.txt"
  StakingRPCsEnabled = true

[ShardData]
  CacheSize = 512
  CacheTime = 10
  DiskCount = 8
  EnableShardData = false
  ShardCount = 2

[Sync]
  Client = false
  Concurrency = 6
  DNSStaticNodes = ["/dnsaddr/trusted.s0.t.hmny.io", "/dnsaddr/trusted.s1.t.hmny.io"]
  DiscBatch = 8
  DiscHardLowCap = 6
  DiscHighCap = 128
  DiscSoftLowCap = 8
  Enabled = true
  InitStreams = 8
  MaxAdvertiseWaitTime = 60
  MinPeers = 6
  SyncMode = 0

  [Sync.StagedSyncCfg]
    DebugMode = false
    DoubleCheckBlockHashes = false
    InsertChainBatchSize = 128
    LogProgress = false
    MaxBackgroundBlocks = 512
    MaxBlocksPerSyncCycle = 512
    MaxMemSyncCycleSize = 1024
    UseMemDB = true
    VerifyAllSig = false
    VerifyHeaderBatchSize = 100

[TxPool]
  AccountQueue = 64
  AccountSlots = 16
  GlobalQueue = 5120
  GlobalSlots = 4096
  Lifetime = "30m0s"
  LocalAccountsFile = "./.hmy/locals.txt"
  PriceBump = 1
  PriceLimit = 100e9
  RosettaFixFile = ""

[WS]
  AuthPort = 9801
  Enabled = true
  IP = "0.0.0.0"
  Port = 9800
`
