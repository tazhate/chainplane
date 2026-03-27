# Configuration Reference

Complete reference for the `BlockchainNode` custom resource spec, status fields, and chain-specific configuration.

## CRD Spec Reference

### Core Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `spec.chain` | `string` | Yes | -- | Blockchain to run. One of: `ethereum`, `ethereum-archive`, `bitcoin`, `solana`, `bsc`, `tron`, `polygon`, `avalanche`, `litecoin`, `xrp`, `stellar`, `dash`, `ton`, `cosmos`, `near`, `sui`, `aptos`, `cardano`. |
| `spec.network` | `string` | No | `mainnet` | Network environment. One of: `mainnet`, `testnet`, `devnet`. |
| `spec.nodeType` | `string` | No | `rpc` | Node type. One of: `rpc`, `archive`, `validator`, `light`. |
| `spec.client` | `string` | No | adapter default | Client software for multi-client chains. Ethereum supports: `nethermind` (default), `geth`, `reth`, `erigon`. |
| `spec.replicas` | `int32` | No | `1` | Number of instances (0-10). Set to `0` to pause the node while preserving storage. |
| `spec.nodeGroup` | `string` | No | `medium` | Hardware tier for scheduling. One of: `light`, `medium`, `heavy`, `archive`, `storage`, `blockchain`. Maps to node labels for scheduling. |

### Image

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `spec.image.repository` | `string` | Yes (if image set) | -- | Container image repository. Example: `nethermind/nethermind`. |
| `spec.image.tag` | `string` | Yes (if image set) | -- | Image tag. Example: `1.36.1`. |
| `spec.image.pullPolicy` | `string` | No | `IfNotPresent` | Kubernetes image pull policy. One of: `Always`, `IfNotPresent`, `Never`. |

If `spec.image` is not set, the adapter's default image is used. See the [adapters reference](adapters.md) for default images per chain.

**Example: Override image**
```yaml
spec:
  image:
    repository: ethereum/client-go
    tag: v1.17.1
    pullPolicy: IfNotPresent
```

### Storage

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `spec.storage.size` | `Quantity` | Yes | -- | PVC size. Examples: `600Gi`, `2Ti`. Cannot be reduced after creation. |
| `spec.storage.storageClass` | `string` | No | cluster default | Kubernetes StorageClass name. Use an SSD-backed class for blockchain workloads. |

**Example:**
```yaml
spec:
  storage:
    size: 2Ti
    storageClass: fast-ssd
```

### Resources

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `spec.resources.requests.cpu` | `Quantity` | No | -- | CPU request. |
| `spec.resources.requests.memory` | `Quantity` | No | -- | Memory request. |
| `spec.resources.limits.cpu` | `Quantity` | No | -- | CPU limit. |
| `spec.resources.limits.memory` | `Quantity` | No | -- | Memory limit. |

**Example:**
```yaml
spec:
  resources:
    requests:
      cpu: "4"
      memory: 16Gi
    limits:
      cpu: "8"
      memory: 32Gi
```

### RPC

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `spec.rpc.enabled` | `bool` | No | `true` | Expose RPC endpoints via a Service. |
| `spec.rpc.port` | `int32` | No | `8545` | HTTP RPC port. Chain-specific defaults apply (BTC: 8332, SOL: 8899, etc.). |
| `spec.rpc.wsPort` | `int32` | No | -- | WebSocket RPC port. Set only for chains that support it (ETH: 8546, SOL: 8900, etc.). |

**Example:**
```yaml
spec:
  rpc:
    enabled: true
    port: 8545
    wsPort: 8546
```

### Health

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `spec.health.blockLagThreshold` | `int64` | No | chain default | Maximum block/slot lag before marking the node Degraded. Defaults: ETH=30, BTC=2, TRON=200. |
| `spec.health.degradedTimeoutMinutes` | `int32` | No | `15` | Minutes a node may remain Degraded before auto-restart. Set to `0` to disable auto-restart. |

**Example:**
```yaml
spec:
  health:
    blockLagThreshold: 50
    degradedTimeoutMinutes: 30
```

### Snapshot

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `spec.snapshot.disabled` | `bool` | No | `false` | Skip snapshot bootstrap; sync from genesis. |
| `spec.snapshot.type` | `string` | No | `full` | Snapshot variant: `full` or `lite`. Lite snapshots are smaller but may lack full history. For TRON: `lite` uses LiteFullNode (~60 GiB) instead of FullNode (~2.9 TiB). |
| `spec.snapshot.bucket` | `string` | No | `snapshots-{chain}` | Override the MinIO bucket name. |
| `spec.snapshot.key` | `string` | No | auto | Override the specific object key in the bucket. |

Snapshot bootstrap requires the `MINIO_ENDPOINT` environment variable to be set on the operator deployment. The operator adds an init container to the StatefulSet that downloads and extracts the snapshot before the node starts.

**Example: Disable snapshot**
```yaml
spec:
  snapshot:
    disabled: true
```

**Example: TRON lite snapshot**
```yaml
spec:
  chain: tron
  snapshot:
    type: lite
```

### Extra Configuration

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `spec.extraArgs` | `[]string` | No | -- | Additional CLI arguments appended to the node process command line. |
| `spec.extraEnv` | `[]EnvVar` | No | -- | Additional environment variables for the node container. Uses standard Kubernetes EnvVar format. |
| `spec.sidecars` | `[]Container` | No | -- | Additional containers running alongside the node. Sidecars share the `data` volume. Useful for consensus-layer clients (e.g., Lighthouse for Ethereum). |
| `spec.extraVolumes` | `[]Volume` | No | -- | Additional pod volumes appended to the default `data` and `config` volumes. |
| `spec.extraVolumeMounts` | `[]VolumeMount` | No | -- | Additional volume mounts for the main node container. |

**Example: Ethereum with Lighthouse sidecar and JWT secret**
```yaml
spec:
  chain: ethereum
  client: geth
  extraArgs:
    - --authrpc.jwtsecret=/secrets/jwt.hex
  extraVolumes:
    - name: jwt-secret
      secret:
        secretName: ethereum-jwt
  extraVolumeMounts:
    - name: jwt-secret
      mountPath: /secrets
      readOnly: true
  sidecars:
    - name: lighthouse
      image: sigp/lighthouse:latest
      args:
        - beacon_node
        - --execution-endpoint=http://localhost:8551
        - --jwt-secrets=/secrets/jwt.hex
      volumeMounts:
        - name: jwt-secret
          mountPath: /secrets
          readOnly: true
```

## Status Fields

These are read-only fields set by the controller.

| Field | Type | Description |
|-------|------|-------------|
| `status.phase` | `string` | Current lifecycle phase: `Pending`, `Syncing`, `Healthy`, `Degraded`, `Failed`. |
| `status.blockHeight` | `int64` | Latest confirmed block/slot number. |
| `status.syncProgress` | `string` | Human-readable sync percentage (e.g., `98.5%`). |
| `status.syncETA` | `string` | Estimated time remaining (e.g., `2h15m`). Empty when synced or when rate cannot be calculated. |
| `status.peersCount` | `int32` | Connected peer count. |
| `status.conditions` | `[]Condition` | Standard Kubernetes conditions: `Ready`, `Syncing`, `Degraded`. |
| `status.observedGeneration` | `int64` | Last spec generation processed by the controller. |

## Chain-Specific Configuration Notes

### Ethereum

- **Multi-client:** Set `spec.client` to `nethermind` (default), `geth`, `reth`, or `erigon`. Each client uses a different config format and default image.
- **Archive mode:** Use `chain: ethereum-archive` instead of `chain: ethereum` for archive nodes.
- **Testnet:** Uses Sepolia. Set `network: testnet`.
- Geth v1.15+ changed DB format (PBSS) -- resync required when upgrading from v1.14.x.
- Reth and Erigon have long startup times; liveness probe is configured with `initialDelaySeconds=300`.

### Bitcoin / Litecoin / Dash

- RPC authentication is required. Default credentials are injected via environment variables (`BTC_RPC_USER`/`BTC_RPC_PASSWORD` for Bitcoin, similar for others). Default: `rpc`/`rpc`.
- `txindex=1` is enabled by default.
- Block intervals are long (BTC: ~10 min, LTC: ~2.5 min, Dash: ~2.5 min), so synced nodes are StallExempt.

### TRON

- Extremely long startup on mainnet (~6h for `loadTransForLiteNode` with 81M+ transactions).
- Use `snapshot.type: lite` for LiteFullNode snapshots (~60 GiB vs ~2.9 TiB).
- Custom JVM command bypasses the shell script to avoid SIGSEGV bug in JDK 8 on Linux 6.x kernels.
- `vm.maxTimeRatio=100` is set for heavy historical blocks.

### TON

- Requires UDP NodePort for ADNL P2P protocol. The adapter auto-allocates per-instance ports.
- Startup probe allows up to 24h for ~500 GiB dump download + extraction.
- `PUBLIC_IP` is injected from `status.hostIP` via Kubernetes Downward API.

### Solana

- Requires high memory (64+ GiB recommended).
- Startup probe allows 1h for snapshot download.
- Gossip port (8001 UDP) is used for cluster participation.

### Cosmos

- CometBFT state sync with auto-fetched trust height/hash from Polkachu RPC.
- Genesis downloaded from MinIO on first run.
- During state sync, pseudo-block (time-varying) is reported to prevent stall detection.

### Cardano

- IPv6 is disabled in P2P topology (causes `TraceOutboundGovernorCriticalFailure` on IPv4-only hosts).
- Prometheus metrics patched to bind to `0.0.0.0` (default is `127.0.0.1`).
- Config files auto-downloaded from IntersectMBO/cardano-node GitHub tag.

## Resource Recommendations per Chain

| Chain | CPU Request | Memory Request | CPU Limit | Memory Limit | Storage |
|-------|-------------|----------------|-----------|--------------|---------|
| Bitcoin | 2 | 4Gi | 4 | 8Gi | 600Gi |
| Ethereum | 4 | 16Gi | 8 | 32Gi | 2Ti |
| Ethereum (archive) | 8 | 32Gi | 16 | 64Gi | 3Ti+ |
| Solana | 8 | 64Gi | 16 | 128Gi | 2Ti |
| TRON | 4 | 16Gi | 8 | 32Gi | 2Ti |
| TON | 4 | 16Gi | 8 | 32Gi | 1Ti |
| Cosmos | 2 | 8Gi | 4 | 16Gi | 500Gi |
| Avalanche | 4 | 16Gi | 8 | 32Gi | 1Ti |
| BSC | 4 | 16Gi | 8 | 32Gi | 2Ti |
| Polygon | 4 | 16Gi | 8 | 32Gi | 3Ti |
| Cardano | 2 | 8Gi | 4 | 16Gi | 200Gi |
| Litecoin | 2 | 4Gi | 4 | 8Gi | 150Gi |
| XRP | 2 | 4Gi | 4 | 8Gi | 50Gi |
| Stellar | 2 | 4Gi | 4 | 8Gi | 50Gi |
| Dash | 2 | 4Gi | 4 | 8Gi | 50Gi |
| NEAR | 4 | 8Gi | 8 | 16Gi | 500Gi |
| Sui | 4 | 16Gi | 8 | 32Gi | 1Ti |
| Aptos | 4 | 16Gi | 8 | 32Gi | 1Ti |

These are starting recommendations. Monitor actual usage and adjust.

## Storage Class Considerations

- **IOPS matter:** Blockchain nodes are I/O-heavy. Use SSD or NVMe-backed StorageClasses. HDD storage will severely degrade sync performance.
- **Expansion:** Ensure your StorageClass supports volume expansion (`allowVolumeExpansion: true`) for when chain data grows beyond initial estimates.
- **Retain policy:** Consider `reclaimPolicy: Retain` to prevent accidental data loss. PVCs are not deleted when the BlockchainNode resource is removed.
- **ReadWriteOnce:** All PVCs are created with `ReadWriteOnce` access mode (single-node attachment).

Example StorageClass (AWS EBS gp3):

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: fast-ssd
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
  iops: "6000"
  throughput: "250"
allowVolumeExpansion: true
reclaimPolicy: Retain
volumeBindingMode: WaitForFirstConsumer
```

## Health Check Tuning

### Block Lag Threshold

The `blockLagThreshold` determines how many blocks behind the chain tip a node can be before transitioning to Degraded. Tune based on the chain's block time:

| Chain | Block Time | Recommended Threshold | Rationale |
|-------|------------|----------------------|-----------|
| Ethereum | ~12s | 30 blocks (~6 min) | Allows temporary network latency |
| Bitcoin | ~10 min | 2 blocks (~20 min) | Long block times, low threshold |
| Solana | ~400ms | 150 slots (~1 min) | Fast block production |
| TRON | ~3s | 200 blocks (~10 min) | Moderate block time |
| Cardano | ~20s | 50 blocks (~17 min) | Moderate block time |

### Degraded Timeout

The `degradedTimeoutMinutes` controls how long a node stays in Degraded before the controller deletes the pod to trigger a restart:

- **Default: 15 minutes** -- suitable for most chains.
- **Set higher (30-60)** for chains with known long recovery times after temporary issues.
- **Set to 0** to disable auto-restart entirely (manual intervention required).

### Stall Detection

The controller tracks block height changes via annotations. If block height stops advancing during sync, the controller checks the adapter's StallExempt flag. Chains with known slow internal phases (Ethereum pipeline stages, Stellar bucket apply, TON dump download) set StallExempt to prevent false restarts.

You do not need to configure stall detection -- it is handled automatically per chain.
