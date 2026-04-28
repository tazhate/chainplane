# Health Monitoring

The operator includes a comprehensive health monitoring system with two controllers, five trigger conditions, automatic pod replacement with traffic management, and Prometheus metrics export.

## Architecture

Two independent controllers run concurrently:

```
                      BlockchainNode CR
                       /            \
                      /              \
       BlockchainNodeReconciler    NodeHealthReconciler
       (main controller)           (health controller)
              |                          |
              v                          v
       Desired State Management    Health Trigger Checks
       - StatefulSet               - Prometheus queries
       - Service                   - 5 trigger conditions
       - ConfigMap                 - Auto-replacement
       - PVC                       - Traffic management
       - Phase transitions
       - Sync status updates
       - Auto-restart (degraded timeout)
```

**BlockchainNodeReconciler** (30s reconciliation):
- Manages the desired state (StatefulSet, Service, ConfigMap, PVC)
- Performs per-chain RPC health checks via adapters
- Updates status fields (phase, block height, sync progress, peers, ETA)
- Handles auto-restart of Degraded nodes after timeout

**NodeHealthReconciler** (60s reconciliation):
- Evaluates five Prometheus-based health triggers per pod
- Initiates blue/green pod replacement when triggers fire
- Monitors replacement pod health and manages traffic switching
- Rolls back failed replacements

The two controllers do not conflict. The main controller manages Kubernetes resources and sync status. The health controller operates at a higher level, making replacement decisions based on Prometheus metrics.

## Phase Lifecycle

```
Pending  -->  Syncing  -->  Healthy
                 ^             |
                 |             v
                 +------- Degraded -----> Failed (manual)
                              |
                     (auto-restart after timeout)
```

### Phase Descriptions

| Phase | Description | Entry Condition |
|-------|-------------|-----------------|
| **Pending** | StatefulSet created, pod not yet running. | Initial state after CR creation. Pod may be scheduling, pulling images, or running init containers. |
| **Syncing** | Node is catching up to chain tip. | Pod is running and responding to health checks, but `syncProgress < 100%`. |
| **Healthy** | Node is fully synced and responsive. | `syncProgress = 100%` and health checks pass. Block height is advancing. |
| **Degraded** | Node fell behind or is experiencing issues. | Block lag exceeds `blockLagThreshold`, health check failures, or sync stall detected. |
| **Failed** | Requires manual intervention. | Not set automatically by the controller. Reserved for manual operator annotation. |

### Phase Transitions

**Pending -> Syncing:** Pod starts running and the adapter's `HealthCheck()` returns a response (even if still syncing).

**Syncing -> Healthy:** `SyncStatus.Progress` reaches 100% (or `SyncStatus.IsSyncing` is false and current block is close to network tip).

**Healthy -> Degraded:** Block lag exceeds the configured `blockLagThreshold`, or consecutive health check failures occur.

**Degraded -> Syncing:** Node recovers and starts catching up again.

**Degraded -> auto-restart:** After `degradedTimeoutMinutes` (default 15), the controller deletes the pod. The StatefulSet recreates it, and the cycle starts from Pending.

## Health Check Mechanism

Every 30 seconds, the main controller calls the adapter's `HealthCheck(ctx, rpcURL)` method for each BlockchainNode. The adapter makes chain-specific RPC calls and returns a `SyncStatus`:

```go
type SyncStatus struct {
    IsSyncing    bool
    CurrentBlock int64
    HighestBlock int64
    Peers        int32
    Progress     float64   // 0.0 - 100.0
    StallExempt  bool      // suppress stall detection for this cycle
}
```

**Per-chain health check methods:**

| Chain | RPC Method | Notes |
|-------|-----------|-------|
| Ethereum | `eth_syncing` + `eth_blockNumber` + `net_peerCount` | Erigon: stage-aware parsing |
| Bitcoin | `getblockchaininfo` + `getconnectioncount` | Uses `verificationprogress` |
| Solana | `getSlot` (finalized) + `getEpochInfo` | Slot-based progress |
| TRON | HTTP GET `/wallet/getnodeinfo` | Parses `peerList` sync state |
| TON | HTTP GET to internal seqno server (port 8081) | `local_seqno/network_seqno` |
| Cosmos | HTTP GET `/status` on Tendermint RPC | `catching_up` + `latest_block_height` |
| Avalanche | `/ext/health` + C-Chain EVM RPC | Two-phase bootstrap detection |
| BSC / Polygon | Delegates to Ethereum adapter | Same `eth_syncing` flow |
| Cardano | Prometheus metrics scrape (port 12798) | `cardano_node_metrics_blockNum_int` |
| Stellar | HTTP GET `/info` | State machine: Booting -> Synced! |
| Sui | Prometheus metrics scrape (port 9184) | `highest_synced_checkpoint` |
| XRP | JSON-RPC `server_info` | `server_state` + `validated_ledger.seq` |
| NEAR | HTTP GET `/status` | `sync_info.latest_block_height` |
| Aptos | HTTP GET `/v1` | `ledger_version` + Prometheus peers |

### Stall Detection

The controller tracks block height via annotations (`nodes.chainplane.io/last-sync-block`, `nodes.chainplane.io/sync-stall-since`). If block height stops advancing during sync, the adapter's `StallExempt` flag determines whether this is expected:

**StallExempt chains/conditions:**
- Ethereum: all syncing states (Reth/Erigon pipeline stages freeze block height for hours)
- Bitcoin/Litecoin/Dash: fully synced nodes (block time is 2.5-10 min)
- Stellar: "Joining SCP" phase and ledger=1
- TON: during dump download
- NEAR: during state sync (XOR trick keeps block changing)
- Cosmos: during state sync (pseudo-block generation)
- Aptos: during state snapshot sync when ledger_version=0

### Sync ETA Calculation

The controller snapshots progress percentage and block height at regular intervals (every 5 minutes) via annotations. Using two consecutive snapshots, it calculates:

- Progress rate: `(current_progress - snapshot_progress) / time_elapsed`
- Remaining progress: `100 - current_progress`
- ETA: `remaining / rate`

ETA is displayed in the `syncETA` status field and the `ETA` column in `kubectl get blockchainnodes`.

## Node Health Triggers

The `NodeHealthReconciler` evaluates five independent triggers every 60 seconds using Prometheus queries. All triggers must be backed by a Prometheus instance scraping the cluster metrics.

### 1. Sync Lag (`sync_lag`)

Checks whether the blockchain sync lag has exceeded a threshold for a sustained period.

| Parameter | Ethereum | Solana |
|-----------|----------|--------|
| **PromQL** | `eth_sync_lag{node="<pod>"}` | `sol_slot_lag{node="<pod>"}` |
| **Threshold** | 30 blocks | 150 slots |
| **Duration** | 10 minutes sustained | 10 minutes sustained |

Fires only if the metric stays above the threshold for the full duration window, preventing false triggers from momentary spikes.

### 2. Error Rate (`error_rate`)

Checks whether the RPC error ratio exceeds the configured threshold.

| Parameter | Value |
|-----------|-------|
| **PromQL** | `sum(rate(eth_rpc_errors_total{pod=~"<pod>"}[5m])) / sum(rate(eth_rpc_requests_total{pod=~"<pod>"}[5m]))` |
| **Threshold** | 5% (0.05) |
| **Window** | 5 minutes |

Fires immediately when the current error rate exceeds the threshold (no sustained duration check).

### 3. Latency (`latency`)

Checks whether p99 RPC response time exceeds the threshold for a sustained period.

| Parameter | Ethereum | Solana |
|-----------|----------|--------|
| **PromQL** | `histogram_quantile(0.99, sum(rate(rpc_request_duration_seconds_bucket{pod=~"<pod>"}[5m])) by (le))` | Same |
| **Threshold** | 2.0 seconds | 0.5 seconds |
| **Duration** | 5 minutes sustained | 5 minutes sustained |

### 4. Crash Loop (`crash_loop`)

Checks whether the pod has had excessive restarts within a time window.

| Parameter | Value |
|-----------|-------|
| **PromQL** | `kube_pod_container_status_restarts_total{pod=~"<pod>"}` |
| **Threshold** | 3 restarts |
| **Window** | 10 minutes |

Calculates restarts within the window by comparing current restart count against the count at `now - window`.

### 5. Disk Usage (`disk_usage`)

Checks whether PVC utilization exceeds the threshold.

| Parameter | Value |
|-----------|-------|
| **PromQL** | `sum(container_fs_usage_bytes{pod=~"<pod>"}) / sum(container_fs_limit_bytes{pod=~"<pod>"})` |
| **Threshold** | 90% (0.9) |

Fires immediately when exceeded.

### Threshold Configuration

All thresholds can be customized when creating the `Checker`:

```go
health.Thresholds{
    SyncLagETH:        30,    // blocks
    SyncLagSOL:        150,   // slots
    SyncLagDuration:   10,    // minutes
    ErrorRate:         0.05,  // ratio (5%)
    ErrorRateWindow:   5,     // minutes
    LatencyETH:        2.0,   // seconds
    LatencySOL:        0.5,   // seconds
    LatencyDuration:   5,     // minutes
    CrashLoopRestarts: 3,     // count
    CrashLoopWindow:   10,    // minutes
    DiskUsage:         0.9,   // ratio (90%)
}
```

Zero-value fields fall back to the defaults listed above.

### Safe Fallback

If Prometheus is unreachable, all triggers report as **not triggered**. This prevents the health system from making replacement decisions without metrics data.

## Auto-Replacement Workflow

When any health trigger fires, the `NodeHealthReconciler` initiates a blue/green pod replacement. The workflow progresses through six phases, tracked via annotations on the BlockchainNode resource.

### Replacement Phases

```
Pending --> Draining --> Creating --> Verifying --> Completing
                                        |
                                        v (on timeout/failure)
                                    RollingBack
```

### Phase Details

**1. Draining**
- Old pod is marked `nodes.chainplane.io/ready=false` (removes it from Service endpoints)
- `TrafficManager.Drain()` is called to wait for active connections to close
- Default drain timeout: 30 seconds

**2. Creating**
- Replacement pod is created by cloning the original pod's spec
- Replacement pod gets labels: `nodes.chainplane.io/instance=replacement`, `nodes.chainplane.io/replacement-for=<old-pod>`, `nodes.chainplane.io/ready=false`
- Pod is created with `NodeName=""` so the scheduler picks a node

**3. Verifying**
- On each reconciliation (every 30s), the controller checks:
  - Replacement pod is in `Running` phase
  - All containers are ready
  - All 5 health triggers pass for the replacement pod
- If sync timeout (default 10 min) is exceeded, triggers rollback

**4. Completing**
- `TrafficManager.SwitchTraffic()` atomically moves traffic:
  1. Set old pod `ready=false`
  2. Wait 5s for label propagation
  3. Set new pod `ready=true`
- `TrafficManager.ValidateTraffic()` verifies the new pod appears in Service endpoints
- Old pod is deleted
- (Optional) Old pod's PVCs are cleaned up if `CleanupPVC=true`
- Replacement annotations are cleared

**5. RollingBack** (on failure)
- Replacement pod is deleted
- Old pod is restored to `ready=true`
- Replacement annotations are cleared

### Replacement Annotations

| Annotation | Description |
|------------|-------------|
| `nodes.chainplane.io/replacement-phase` | Current phase: `Draining`, `Creating`, `Verifying`, `Completing`, `RollingBack` |
| `nodes.chainplane.io/replacement-pod` | Name of the replacement pod |
| `nodes.chainplane.io/replacement-old-pod` | Name of the original pod being replaced |
| `nodes.chainplane.io/replacement-started-at` | RFC3339 timestamp of when replacement started |

### Replacement Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `DrainTimeout` | 30s | How long to wait for traffic to drain from old pod |
| `SyncTimeout` | 10 min | How long to wait for replacement pod to become healthy |
| `PollInterval` | 10s | Interval between status checks during monitoring |
| `GracePeriodSeconds` | 30 | Pod deletion grace period |
| `CleanupPVC` | false | Whether to delete old pod's PVCs after replacement |

## Traffic Management

Traffic management uses a label-based approach. Services must include `nodes.chainplane.io/ready=true` in their selector. Toggling this label on pods controls whether the Kubernetes endpoint controller includes them in the Service's endpoint list.

### Drain Operation

1. Set `nodes.chainplane.io/ready=false` on the pod
2. Endpoint controller removes the pod from Service endpoints
3. Wait for drain timeout to allow active connections to complete
4. Log pod details for observability

### Traffic Switch

1. Set old pod `ready=false` (removes from Service)
2. Wait 5 seconds for label propagation delay
3. Set new pod `ready=true` (adds to Service)

### Validation

After switching, the system verifies:
1. New pod has `ready=true` label
2. New pod has an assigned IP
3. New pod IP appears in all matching Service endpoint subsets

## Prometheus Metrics Reference

The operator exports these Prometheus metrics via the controller-runtime metrics endpoint (default `:8080/metrics`).

### Sync Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `blockchain_node_block_height` | Gauge | `chain`, `network`, `node`, `node_type` | Latest confirmed block height |
| `blockchain_node_sync_progress` | Gauge | `chain`, `network`, `node` | Sync progress percentage (0-100) |
| `blockchain_node_peers_count` | Gauge | `chain`, `network`, `node` | Number of connected peers |

### Phase Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `blockchain_node_phase` | Gauge | `chain`, `network`, `node`, `phase` | Current phase (1 = active, 0 = inactive). One gauge per phase per node. |

### Recovery Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `blockchain_node_restarts_total` | Counter | `chain`, `network`, `node`, `reason` | Total auto-restarts triggered by the operator |
| `blockchain_node_degraded_duration_seconds` | Histogram | `chain`, `network` | Duration of degraded episodes (buckets: 1m, 5m, 10m, 15m, 30m, 1h, 2h) |

## Alerting Recommendations

### Critical Alerts

```yaml
# Node has been Degraded for more than 30 minutes
- alert: BlockchainNodeDegradedLong
  expr: blockchain_node_phase{phase="Degraded"} == 1
  for: 30m
  labels:
    severity: critical
  annotations:
    summary: "{{ $labels.node }} has been Degraded for 30+ minutes"

# Node has zero peers
- alert: BlockchainNodeNoPeers
  expr: blockchain_node_peers_count == 0
  for: 10m
  labels:
    severity: critical
  annotations:
    summary: "{{ $labels.node }} has no connected peers"

# Multiple auto-restarts in short period
- alert: BlockchainNodeExcessiveRestarts
  expr: increase(blockchain_node_restarts_total[1h]) > 3
  labels:
    severity: critical
  annotations:
    summary: "{{ $labels.node }} has been auto-restarted {{ $value }} times in the last hour"
```

### Warning Alerts

```yaml
# Node is still syncing after 24 hours
- alert: BlockchainNodeSyncSlow
  expr: blockchain_node_phase{phase="Syncing"} == 1
  for: 24h
  labels:
    severity: warning
  annotations:
    summary: "{{ $labels.node }} has been syncing for 24+ hours"

# Sync progress not advancing
- alert: BlockchainNodeSyncStalled
  expr: deriv(blockchain_node_sync_progress[30m]) == 0 and blockchain_node_sync_progress < 100
  for: 30m
  labels:
    severity: warning
  annotations:
    summary: "{{ $labels.node }} sync progress has not advanced in 30 minutes"

# Low peer count
- alert: BlockchainNodeLowPeers
  expr: blockchain_node_peers_count < 3 and blockchain_node_peers_count > 0
  for: 15m
  labels:
    severity: warning
  annotations:
    summary: "{{ $labels.node }} has only {{ $value }} peers"

# Disk usage approaching limit
- alert: BlockchainNodeDiskHigh
  expr: |
    sum(container_fs_usage_bytes{pod=~".*-0"}) by (pod)
    / sum(container_fs_limit_bytes{pod=~".*-0"}) by (pod)
    > 0.85
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "{{ $labels.pod }} disk usage above 85%"
```

### Informational

```yaml
# Node entered Degraded phase
- alert: BlockchainNodeDegraded
  expr: blockchain_node_phase{phase="Degraded"} == 1
  for: 1m
  labels:
    severity: info
  annotations:
    summary: "{{ $labels.node }} entered Degraded phase"

# Degraded episodes longer than 15 minutes
- alert: BlockchainNodeDegradedEpisodeLong
  expr: histogram_quantile(0.95, rate(blockchain_node_degraded_duration_seconds_bucket[1h])) > 900
  labels:
    severity: info
  annotations:
    summary: "p95 degraded episode duration exceeds 15 minutes for {{ $labels.chain }}/{{ $labels.network }}"
```
