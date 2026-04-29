# Supported Chain Adapters

This document describes every blockchain adapter supported by the operator, including default images, ports, health check methods, and chain-specific behavior.

## Quick Reference

| Chain | Clients | Default Image | RPC Port | WS Port | Special |
|-------|---------|---------------|----------|---------|---------|
| Bitcoin | bitcoind | `lncm/bitcoind:v28.0` | 8332 (mainnet) / 18332 (testnet) | - | RPC auth via env vars |
| Ethereum | geth, reth, erigon, nethermind | `nethermind/nethermind:1.36.1` | 8545 | 8546 | Multi-client, archive mode |
| Solana | solana-labs | `solanalabs/solana:v1.18.26` | 8899 | 8900 | Startup probe (1h) |
| TRON | java-tron | `tronprotocol/java-tron:GreatVoyage-v4.8.1` | 8090 (HTTP) | - | Startup probe (6h mainnet), custom JVM command |
| TON | validator-engine | `ghcr.io/ton-blockchain/ton:latest` | 30003 (liteserver) | - | UDP NodePort, startup probe (24h), dump restore |
| Cosmos | gaiad | `ghcr.io/cosmos/gaia:v27.0.0` | 26657 (Tendermint) / 1317 (API) | - | CometBFT state sync, startup probe (1h) |
| Avalanche | avalanchego | `avaplatform/avalanchego:v1.14.1` | 9650 | - | Startup probe (2h), delegates to C-Chain EVM |
| BSC | bsc-geth | `ghcr.io/bnb-chain/bsc:1.6.7` | 8545 | 8546 | Genesis download on first run |
| Cardano | cardano-node | `ghcr.io/intersectmbo/cardano-node:10.6.2` | 12798 (metrics) | - | Startup probe (3h), Prometheus health check |
| Dash | dashd | `dashpay/dashd:23.1.0` | 9998 | - | RPC auth via env vars |
| Litecoin | litecoind | `uphold/litecoin-core:0.21` | 9332 | - | RPC auth via env vars, custom LITECOIN_DATA env |
| NEAR | nearcore | `nearprotocol/nearcore:2.10.7` | 3030 | - | HTTP health probe, startup probe (4h) |
| Polygon | bor | `0xpolygon/bor:2.6.3` | 8545 | 8546 | HostPort P2P, Heimdall sidecar |
| Stellar | stellar-core | `stellar/stellar-core:latest` | 11626 (HTTP) | - | Watcher mode, fast catchup |
| Sui | sui-node | `mysten/sui-node:mainnet` | 9000 | - | Init container (formal snapshot), startup probe (1h) |
| XRP | rippled | `xrpllabsofficial/xrpld:3.1.2` | 5005 (HTTP) | 6006 (WS) | Syncs from current ledger tip (`--net`) |
| Aptos | aptos-node | `aptoslabs/validator:mainnet` | 8080 | - | HTTP health probe, startup probe (4h) |
| Blast | blast-geth | `blastio/blast-geth:v1.2.0` | 8545 | 8546 | OP Stack L2, L1_RPC_URL env |
| Mode | op-geth | `us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101411.2` | 8545 | 8546 | OP Stack L2, L1_RPC_URL env |
| Zora | op-geth | `us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101411.2` | 8545 | 8546 | OP Stack L2, L1_RPC_URL env |
| Taiko | taiko-geth | `taikoxyz/taiko-geth:v1.8.0` | 8545 | 8546 | L2, L1_RPC_URL env |

---

## Per-Chain Details

### Bitcoin

**Supported clients:** Bitcoin Core (bitcoind)
**Default image:** `lncm/bitcoind:v28.0`
**Config file:** `bitcoin.conf`

**Ports:**
- Mainnet: RPC 8332, P2P 8333
- Testnet: RPC 18332, P2P 18333

**Health check:** JSON-RPC `getblockchaininfo` (verificationprogress) + `getconnectioncount`. RPC auth is required -- credentials from `BTC_RPC_USER` / `BTC_RPC_PASSWORD` env vars (default: `rpc`/`rpc`).

**Configuration:**
- `txindex=1` enabled by default
- `rpcallowip=0.0.0.0/0` (cluster CIDR)
- Testnet uses `[test]` config section

**Notes:**
- Fully-synced nodes are StallExempt (block interval ~10 min)
- Liveness probe: TCP on RPC port

**Storage:** 600+ GiB for mainnet with txindex

---

### Ethereum

**Supported clients:** Nethermind (default), Geth, Reth, Erigon
**Default images:**
- Nethermind: `nethermind/nethermind:1.36.1`
- Geth: `ethereum/client-go:v1.17.1`
- Reth: `ghcr.io/paradigmxyz/reth:v1.11.3`
- Erigon: `erigontech/erigon:v2.61.3`

**Registered chains:** `ethereum`, `ethereum-archive`

**Config files:**
- Nethermind: `nethermind.json`
- Geth: `config.toml`
- Reth: `reth.toml`
- Erigon: `erigon.yaml`

**Ports:**
- RPC: 8545 (HTTP)
- WS: 8546
- P2P: 30303 (TCP + UDP)

**Health check:** `eth_syncing` + `eth_blockNumber` + `net_peerCount`. Erigon stage-aware parsing extracts minimum non-zero stage block as current progress. RPC timeouts during heavy pipeline stages (Reth/Erigon) return StallExempt.

**Configuration:**
- Testnet uses Sepolia
- Nethermind uses `FastSync` for non-archive nodes
- Geth v1.15+ changed DB format (PBSS) -- resync required when upgrading from v1.14.x

**Notes:**
- Liveness probe initialDelaySeconds=300 (Reth/Erigon need long startup)
- All syncing states are StallExempt (pipeline stages freeze block height for hours)

**Storage:** 1-2 TiB (full), 3+ TiB (archive)

---

### Solana

**Supported clients:** Solana Labs validator
**Default image:** `solanalabs/solana:v1.18.26`
**Config file:** `validator.yml`

**Ports:**
- RPC: 8899
- WS: 8900
- Gossip: 8001 (UDP)

**Health check:** `getSlot` (finalized commitment) + `getEpochInfo`.

**Configuration:**
- Full RPC API enabled, no-voting mode
- Testnet uses `entrypoint.testnet.solana.com:8001`

**Special features:**
- **Startup probe:** 1h (120 x 30s) for snapshot download
- Liveness probe: TCP on 8899

**Storage:** 2+ TiB recommended

---

### TRON

**Supported clients:** java-tron (FullNode)
**Default image:** `tronprotocol/java-tron:GreatVoyage-v4.8.1`
**Config file:** `config.conf` (HOCON format)

**Ports:**
- HTTP API: 8090
- gRPC: 50051
- P2P: 18888

**Health check:** HTTP GET `/wallet/getnodeinfo`. Parses `peerList` to derive sync state from `unFetchSynNum`, `syncToFetchSize`, and `blockInPorcSize`.

**Configuration:**
- Mainnet: LevelDB with `txCache.initOptimization=true` (persists cache for fast restarts)
- `vm.maxTimeRatio=100` for heavy historical blocks (default 5 is too low)
- Testnet: Nile network

**Special features:**
- **Startup probe:** Mainnet 6h (720 x 30s) for `loadTransForLiteNode` (81M+ transactions). Testnet 20min.
- **Custom container command:** Bypasses FullNode shell script, runs JDK 11 directly with G1GC (avoids SIGSEGV bug in JDK 8 on Linux 6.x kernels)
- Custom container args: `-c /config/config.conf -d /data`

**Storage:** 2+ TiB for mainnet

---

### TON

**Supported clients:** validator-engine
**Default image:** `ghcr.io/ton-blockchain/ton:latest`
**Config file:** `global-config.json` (embedded mainnet global config)

**Ports:**
- Liteserver: 30003 (TCP)
- P2P (ADNL): 30001 (UDP)
- Health seqno: 8081 (TCP, internal)

**Health check:** HTTP GET to internal seqno server (port 8081) which returns `local_seqno/network_seqno`. Background loop queries external liteservers via lite-client and local liteserver via validator-engine-console every 60s.

**Configuration:**
- Full mainnet global config embedded in adapter
- `PUBLIC_IP` injected via Kubernetes Downward API (`status.hostIP`) for ADNL
- Liteserver and control interface auto-repaired in config.json if missing

**Special features:**
- **UDP NodePort:** Required for ADNL P2P protocol. Per-instance port allocation:
  - Instance -01: P2P=30001, liteserver=30003
  - Instance -02: P2P=30011, liteserver=30013
  - Instance -03: P2P=30021, liteserver=30023
- **Startup probe:** 24h (2880 x 30s) -- covers ~500 GiB dump download + extraction
- **Dump restore:** If `DUMP_URL` env var is set and DB is empty, downloads state dump via aria2c with resume support, extracts with plzip
- Custom `wget` shim serves embedded global-config from ConfigMap
- `ContainerEnv`: `PUBLIC_IP` from `status.hostIP`

**Storage:** 1+ TiB for mainnet with dump

---

### Cosmos (Cosmos Hub)

**Supported clients:** gaiad (Cosmos Hub)
**Default image:** `ghcr.io/cosmos/gaia:v27.0.0`
**Config file:** `app.toml`

**Ports:**
- Tendermint RPC: 26657
- REST API: 1317
- P2P: 26656

**Health check:** HTTP GET `/status` on Tendermint RPC (port 26657). Parses `catching_up`, `latest_block_height`, `latest_block_time`. Estimates network tip from block time lag + `/consensus_state`. Peer count from `/net_info`.

**Configuration:**
- CometBFT state sync with auto-fetched trust height/hash from Polkachu RPC
- Seeds: Polkachu + Lavender Five
- Persistent peers: kjnodes snapshot provider
- Genesis downloaded from public URL (configurable via COSMOS_GENESIS_URL)
- Snapshot boot path detected via `application.db` presence

**Special features:**
- **Startup probe:** 1h (120 x 30s) for state sync
- **Custom container command:** Complex init script handling clean-slate state sync vs snapshot boot, incomplete state sync detection, and genesis patching
- During state sync, reports pseudo-block (time-varying) to prevent stall detection

**Storage:** 500+ GiB for mainnet

---

### Avalanche

**Supported clients:** avalanchego
**Default image:** `avaplatform/avalanchego:v1.14.1`
**Config file:** `config.json`

**Ports:**
- RPC: 9650
- Staking: 9651

**Health check:** Two-phase: (1) `/ext/health` to detect bootstrap phase -- if `bootstrapped` check fails, reads Prometheus metrics for `avalanche_snowman_bs_fetched` and `bootstrap_finished`. (2) Once bootstrapped, delegates to C-Chain EVM RPC (`/ext/bc/C/rpc`) using EthereumAdapter. Peer count via `/ext/info` JSON-RPC `info.peers`.

**Configuration:**
- Testnet: Fuji network
- `index-enabled: false`, `api-admin-enabled: false`
- Custom container command passes `--config-file`

**Special features:**
- **Startup probe:** 2h (240 x 30s) for bootstrapping
- During DB compaction, uses time-XOR trick to keep CurrentBlock changing

**Storage:** 1+ TiB for mainnet

---

### BSC (BNB Smart Chain)

**Supported clients:** BSC Geth
**Default image:** `ghcr.io/bnb-chain/bsc:1.6.7`
**Config file:** `config.toml`

**Ports:**
- RPC: 8545
- WS: 8546
- P2P: 30311 (TCP + UDP)

**Health check:** Delegates to EthereumAdapter (`eth_syncing` + `eth_blockNumber`).

**Configuration:**
- `SyncMode = "snap"`, `PriceLimit = 3000000000`
- Mainnet NetworkId=56, Testnet NetworkId=97
- HTTP/WS modules: eth, net, web3, txpool

**Special features:**
- **Custom container command:** Downloads genesis.json from GitHub on first run, then starts geth with explicit HTTP/WS flags
- Liveness probe initialDelaySeconds=300

**Storage:** 2+ TiB for mainnet

---

### Cardano

**Supported clients:** cardano-node
**Default image:** `ghcr.io/intersectmbo/cardano-node:10.6.2`
**Config file:** `config.json`

**Ports:**
- P2P: 3001
- EKG: 12788
- Prometheus metrics: 12798

**Health check:** Prometheus metrics scrape on port 12798. Reads `cardano_node_metrics_blockNum_int`, `cardano_node_metrics_slotNum_int`, `blockReplayProgress_real`, and `peerSelection_hot_peers_int`. Progress estimated from slot number vs known mainnet tip (~182M slots).

**Configuration:**
- EnableP2P=true with IPv4-only peers (IPv6 causes TraceOutboundGovernorCriticalFailure on IPv4-only hosts)
- Prometheus metrics bound to 0.0.0.0 (patched from default 127.0.0.1)
- P2P topology uses Cardano Foundation backbone IPv4 IPs
- Config files downloaded from IntersectMBO/cardano-node GitHub tag

**Special features:**
- **Startup probe:** 3h (360 x 30s) for initial replay
- **Custom container command:** Downloads official IOHK configs to PVC, patches EnableP2P and Prometheus bind address, writes custom P2P topology
- Version-aware config re-download (`.version` marker)

**Storage:** 200+ GiB for mainnet

---

### Dash

**Supported clients:** Dash Core (dashd)
**Default image:** `dashpay/dashd:23.1.0`
**Config file:** `dash.conf`

**Ports:**
- RPC: 9998
- P2P: 9999

**Health check:** JSON-RPC `getblockchaininfo` (verificationprogress) + `getconnectioncount`. Auth via `DASH_RPC_USER` / `DASH_RPC_PASSWORD` env vars.

**Configuration:**
- `txindex=1`, `dbcache=1024`, `maxconnections=125`
- `rpcallowip=0.0.0.0/0`

**Notes:**
- Fully-synced nodes are StallExempt (block time ~2.5 min)
- Liveness probe: TCP on 9998

**Storage:** 50+ GiB for mainnet

---

### Litecoin

**Supported clients:** Litecoin Core
**Default image:** `uphold/litecoin-core:0.21`
**Config file:** `litecoin.conf`

**Ports:**
- RPC: 9332
- P2P: 9333

**Health check:** JSON-RPC `getblockchaininfo` with retry (verificationprogress) + `getconnectioncount`. Auth via `LTC_RPC_USER` / `LTC_RPC_PASSWORD` env vars. StallExempt during IBD when progress < 95%.

**Configuration:**
- `txindex=1`, `dbcache=4096`, `maxconnections=125`
- `rpcworkqueue=128`, `rpcthreads=8`, `par=4`

**Special features:**
- **ContainerEnv:** Sets `LITECOIN_DATA=/data` so the image entrypoint uses the PVC instead of ephemeral overlay

**Storage:** 150+ GiB for mainnet

---

### NEAR

**Supported clients:** nearcore (neard)
**Default image:** `nearprotocol/nearcore:2.10.7`
**Config file:** `config.json`

**Ports:**
- RPC: 3030
- P2P: 24567

**Health check:** HTTP GET `/status`. Parses `sync_info.latest_block_height`, `syncing`, `num_active_peers`. During state sync, XORs block height with time bits to prevent stall detection.

**Configuration:**
- ExternalStorage state sync with `fast-state-parts` GCS bucket (patched from default `state-parts`)
- Boot nodes from active mainnet peers
- Epoch sync enabled

**Special features:**
- **Startup probe:** 4h (480 x 30s) for state download (HTTP GET `/status` probe)
- **Liveness probe:** HTTP GET `/status`
- **Custom container command:** Init downloads mainnet genesis, patches config for fast-state-parts GCS bucket, passes explicit boot nodes

**Storage:** 500+ GiB for mainnet

---

### Polygon (Bor)

**Supported clients:** Bor
**Default image:** `0xpolygon/bor:2.6.3`
**Config file:** `config.toml`

**Ports:**
- RPC: 8545
- WS: 8546
- P2P: 30303 (TCP + UDP, with HostPort)

**Health check:** Delegates to EthereumAdapter (`eth_syncing` + `eth_blockNumber`). Erigon stage parsing excludes Polygon-specific stages (BorHeimdall, Translation).

**Configuration:**
- Bor 2.x `server` subcommand
- `gcmode = "archive"`, `syncmode = "full"`
- Heimdall sidecar at `http://localhost:1317`
- `txpool.pricelimit = 25000000000`

**Special features:**
- P2P ports use `HostPort: 30303` for direct host binding
- Liveness probe initialDelaySeconds=300
- Custom container command: `bor server --config /config/config.toml`

**Storage:** 3+ TiB for mainnet archive

---

### Stellar

**Supported clients:** stellar-core
**Default image:** `stellar/stellar-core:latest`
**Config file:** `stellar-core.cfg`

**Ports:**
- HTTP API: 11626
- Peer: 11625

**Health check:** HTTP GET `/info`. Parses `state` (Booting -> Joining SCP -> Catching up -> Synced!), `ledger.num`, `peers.authenticated_count`. During catchup, regex extracts target ledger and completion percentage from status lines. StallExempt during "Joining SCP" phase and when ledger=1.

**Configuration:**
- Watcher (non-validator) mode: no NODE_SEED
- `CATCHUP_RECENT=8640` (~12h of history) for fast catchup
- `UNSAFE_QUORUM=true`
- SDF Tier 1 validators (3x)
- History archives use direct S3 bucket URLs (Cloudflare CDN unreachable from pod CIDR)
- SQLite database at `/data/stellar.db`

**Special features:**
- **Custom container command:** Initializes DB if missing or too small (< 1MB threshold to retry after failed catchup)
- RPC timeout during bucket apply returns StallExempt
- Liveness probe initialDelaySeconds=120, failureThreshold=10

**Storage:** 50+ GiB for mainnet (recent history only)

---

### Sui

**Supported clients:** sui-node
**Default image:** `mysten/sui-node:mainnet`
**Config file:** `fullnode.yaml`

**Ports:**
- JSON-RPC: 9000
- Metrics: 9184
- P2P: 8084 (UDP)

**Health check:** Prometheus metrics scrape on port 9184. Reads `highest_synced_checkpoint`, `last_executed_checkpoint`, `sui_quinn_network_peers`. Progress estimated from known mainnet tip (~110M checkpoints).

**Configuration:**
- Official Mysten Labs + community SSF seed peers
- State archive from Mysten GCS bucket (`storage.googleapis.com/mysten-mainnet-checkpoints`)
- genesis.blob downloaded from MystenLabs/sui-genesis GitHub

**Special features:**
- **Init container:** `mysten/sui-tools:mainnet` downloads formal snapshot via `sui-tool download-formal-snapshot` (reduces initial sync from months to hours). Resources: 1-4 CPU, 2-8Gi memory.
- **Startup probe:** 1h (120 x 30s)
- **Custom container command:** Downloads genesis.blob on first run

**Storage:** 1+ TiB for mainnet

---

### XRP (Ripple)

**Supported clients:** rippled
**Default image:** `xrpllabsofficial/xrpld:3.1.2`
**Config file:** `rippled.cfg`

**Ports:**
- RPC (HTTP): 5005
- WS: 6006
- Peer: 51235

**Health check:** JSON-RPC `server_info`. Parses `server_state` (synced when `full`/`validating`/`proposing` and ledger age < 60s), `complete_ledgers`, `validated_ledger.seq`, `peers`.

**Configuration:**
- NuDB storage engine
- `online_delete=512` (keeps ~30 min of history)
- `ledger_history=none` (syncs from current tip, not genesis)
- `node_size=small`
- Fixed peers: s1.ripple.com, s2.ripple.com
- Validator lists from vl.ripple.com and unl.xrplf.org

**Special features:**
- **Custom container command:** `rippled --conf=/config/rippled.cfg --net` (`--net` bootstraps directly to current validated ledger)
- Liveness probe initialDelaySeconds=60

**Storage:** 50+ GiB for mainnet (minimal history)

---

### Aptos

**Supported clients:** aptos-node
**Default image:** `aptoslabs/validator:mainnet`
**Config file:** `fullnode.yaml`

**Ports:**
- REST API: 8080
- Metrics: 9101
- P2P: 6180

**Health check:** HTTP GET `/v1` parses `ledger_version`. Peer count from Prometheus metrics (port 9101, `aptos_connections{direction="inbound"}`). During state snapshot sync, returns pseudo-block (time-varying) when ledger_version=0.

**Configuration:**
- State sync: `DownloadLatestStates` bootstrapping mode
- `enable_storage_sharding: true` (RocksDB)
- Public fullnode network with onchain discovery
- genesis.blob and waypoint.txt downloaded from aptos-labs/aptos-networks GitHub

**Special features:**
- **Startup probe:** 4h (480 x 30s) with HTTP GET `/v1/-/healthy`
- **Liveness probe:** HTTP GET `/v1/-/healthy`
- **Custom container command:** Downloads genesis.blob and waypoint.txt to PVC on first run

**Storage:** 1+ TiB for mainnet

---

### Blast

**Supported clients:** blast-geth
**Default image:** `blastio/blast-geth:v1.2.0`
**Config file:** `config.toml`

**Ports:**
- RPC: 8545
- WS: 8546
- P2P: 30303 (TCP + UDP)

**Health check:** Delegates to EVM health check (`eth_syncing` + `eth_blockNumber`).

**Configuration:**
- OP Stack L2 (NetworkId=81457)
- `SyncMode = "snap"`

**Special features:**
- **ContainerEnv:** `L1_RPC_URL` defaults to `http://ethereum:8545`

**Storage:** 500+ GiB for mainnet

---

### Mode

**Supported clients:** op-geth
**Default image:** `us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101411.2`
**Config file:** `config.toml`

**Ports:**
- RPC: 8545
- WS: 8546
- P2P: 30303 (TCP + UDP)

**Health check:** Delegates to EVM health check (`eth_syncing` + `eth_blockNumber`).

**Configuration:**
- OP Stack L2 (NetworkId=34443)
- `SyncMode = "snap"`

**Special features:**
- **ContainerEnv:** `L1_RPC_URL` defaults to `http://ethereum:8545`

**Storage:** 500+ GiB for mainnet

---

### Zora

**Supported clients:** op-geth
**Default image:** `us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101411.2`
**Config file:** `config.toml`

**Ports:**
- RPC: 8545
- WS: 8546
- P2P: 30303 (TCP + UDP)

**Health check:** Delegates to EVM health check (`eth_syncing` + `eth_blockNumber`).

**Configuration:**
- OP Stack L2 (NetworkId=7777777)
- `SyncMode = "snap"`

**Special features:**
- **ContainerEnv:** `L1_RPC_URL` defaults to `http://ethereum:8545`

**Storage:** 500+ GiB for mainnet

---

### Taiko

**Supported clients:** taiko-geth
**Default image:** `taikoxyz/taiko-geth:v1.8.0`
**Config file:** `config.toml`

**Ports:**
- RPC: 8545
- WS: 8546
- P2P: 30303 (TCP + UDP)

**Health check:** Delegates to EVM health check (`eth_syncing` + `eth_blockNumber`).

**Configuration:**
- Taiko L2 (NetworkId=167000)
- `SyncMode = "snap"`

**Special features:**
- **ContainerEnv:** `L1_RPC_URL` defaults to `http://ethereum:8545`

**Storage:** 500+ GiB for mainnet

---

## Common Adapter Interfaces

All adapters implement the core `ChainAdapter` interface:
- `DefaultImage(client)` -- default container image
- `ConfigTemplate(spec)` -- generates config file (filename + content)
- `HealthCheck(ctx, rpcURL)` -- returns `SyncStatus` with sync progress
- `LivenessProbe(spec)` -- Kubernetes liveness probe
- `NodeSelector(nodeGroup)` -- label selector for hardware tier
- `ContainerPorts(spec)` -- exposed container ports

Optional interfaces (implemented by specific adapters):
- `ContainerCommandProvider` -- custom entrypoint (TRON, TON, Cosmos, BSC, Avalanche, Cardano, Stellar, Sui, NEAR, Polygon, XRP, Aptos)
- `ContainerArgsProvider` -- custom args (TRON)
- `ContainerEnvProvider` -- custom env vars (TON, Litecoin, Blast, Mode, Zora, Taiko)
- `StartupProbeProvider` -- long startup tolerance (Solana, TRON, TON, Cosmos, Avalanche, Cardano, NEAR, Sui, Aptos)
- `NodePortProvider` -- UDP NodePort (TON)
- `InitContainerProvider` -- init containers (Sui)
