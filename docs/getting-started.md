# Getting Started

This guide walks you through installing the operator, deploying your first blockchain node, and performing common operations.

## Prerequisites

- Kubernetes cluster 1.26+ (Kind, EKS, GKE, AKS, etc.)
- `kubectl` configured to access your cluster
- (Optional) Helm 3 for Helm-based deployment
- (Optional) MinIO instance for snapshot bootstrapping (set `MINIO_ENDPOINT` env var on the operator)

### Snapshot Bootstrap (Optional)

Blockchain nodes can take days or weeks to sync from genesis. The operator supports snapshot bootstrap via MinIO (or any S3-compatible storage): the init container downloads a compressed snapshot before the node starts, reducing sync time to hours.

**Set up MinIO with the bundled Helm subchart:**

```bash
helm install chainplane ./charts/chainplane \
  --namespace chainplane-system \
  --create-namespace \
  --set minio.enabled=true \
  --set minio.rootUser=minioadmin \
  --set minio.rootPassword=changeme \
  --set snapshot.enabled=true \
  --set snapshot.minio.endpoint=http://chainplane-minio:9000 \
  --set snapshot.minio.accessKey=minioadmin \
  --set snapshot.minio.secretKey=changeme
```

**Or use an existing MinIO / S3-compatible endpoint:**

```bash
helm install chainplane ./charts/chainplane \
  --namespace chainplane-system \
  --create-namespace \
  --set snapshot.enabled=true \
  --set snapshot.minio.endpoint=http://minio.minio.svc:9000 \
  --set snapshot.minio.existingSecret=minio-credentials
```

The Secret must contain `MINIO_ACCESS_KEY` and `MINIO_SECRET_KEY` keys.

**Populate snapshot buckets:**

The operator looks for snapshots in buckets named `snapshots-<chain>` (e.g. `snapshots-bsc`, `snapshots-ethereum`). Upload compressed chaindata archives there. Public snapshot providers:

- **Cosmos ecosystem:** [Polkachu](https://polkachu.com/tendermint_snapshots) — `snapshots-cosmos`, `snapshots-osmosis`, etc.
- **BSC / Ethereum:** [ChainData](https://chaindata.info/), [Snapshot Finder](https://snapshot.org/)
- **Solana:** [Triton](https://triton.one/), [Stakewiz](https://stakewiz.com/snapshot)

Once a snapshot is in the bucket, enable it per-node:

```yaml
spec:
  snapshot:
    disabled: false   # default; omit this field to use snapshot
    type: full        # or "lite" for supported chains (e.g. TRON)
```

To skip snapshot bootstrap for a specific node (sync from genesis):

```yaml
spec:
  snapshot:
    disabled: true
```

### Storage

Blockchain nodes require significant persistent storage with high IOPS. Ensure your cluster has a StorageClass that provisions SSDs. For production workloads, use a StorageClass backed by NVMe or SSD volumes (e.g., `gp3` on AWS, `pd-ssd` on GCP, `managed-premium` on Azure).

## Installation

### Install CRDs

```bash
# From source
git clone https://github.com/tazhate/chainplane.git
cd chainplane
make install
```

Or apply the generated CRD manifests directly:

```bash
kubectl apply -f dist/install.yaml
```

### Deploy the Operator

#### Option A: kubectl + kustomize

```bash
make deploy IMG=ghcr.io/tazhate/chainplane:latest
```

This deploys the controller manager with RBAC, ServiceAccount, and CRDs into the `chainplane-system` namespace.

#### Option B: Helm

```bash
helm install chainplane ./charts/chainplane \
  --namespace chainplane-system \
  --create-namespace
```

#### Option C: Standalone YAML

Generate a single install manifest:

```bash
make build-installer IMG=ghcr.io/tazhate/chainplane:latest
kubectl apply -f dist/install.yaml
```

### Verify the Operator is Running

```bash
kubectl get pods -n chainplane-system
```

You should see the `chainplane-controller-manager` pod in `Running` state.

## Deploy Your First Blockchain Node

### Bitcoin Mainnet

Save the following as `bitcoin-node.yaml`:

```yaml
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  labels:
    app.kubernetes.io/name: chainplane
    app.kubernetes.io/managed-by: kustomize
  name: bitcoin-mainnet
spec:
  chain: bitcoin
  network: mainnet
  nodeType: rpc
  nodeGroup: medium
  storage:
    size: 600Gi
    storageClass: fast-ssd
  resources:
    requests:
      cpu: "2"
      memory: 4Gi
    limits:
      cpu: "4"
      memory: 8Gi
  rpc:
    enabled: true
    port: 8332
  health:
    blockLagThreshold: 2
```

Apply it:

```bash
kubectl apply -f bitcoin-node.yaml
```

### Verify It Is Working

Watch the node status:

```bash
kubectl get blockchainnodes -w
```

Output columns: `Chain`, `Network`, `Type`, `Phase`, `Height`, `Peers`, `Sync`, `ETA`, `Age`.

Example output during sync:

```
NAME              CHAIN    NETWORK  TYPE  PHASE    HEIGHT   PEERS  SYNC    ETA     AGE
bitcoin-mainnet   bitcoin  mainnet  rpc   Syncing  650000   8      72.5%   3h20m   15m
```

Once fully synced:

```
NAME              CHAIN    NETWORK  TYPE  PHASE    HEIGHT   PEERS  SYNC    ETA  AGE
bitcoin-mainnet   bitcoin  mainnet  rpc   Healthy  886000   12     100%         2d
```

Check the details:

```bash
kubectl describe blockchainnode bitcoin-mainnet
```

Check the managed resources:

```bash
# StatefulSet
kubectl get statefulset bitcoin-mainnet

# PVC
kubectl get pvc -l app.kubernetes.io/instance=bitcoin-mainnet

# Services
kubectl get svc -l app.kubernetes.io/instance=bitcoin-mainnet

# ConfigMap
kubectl get configmap bitcoin-mainnet
```

### Access the RPC Endpoint

The operator creates a ClusterIP Service exposing the RPC port. To access it from within the cluster:

```bash
# Port-forward for local testing
kubectl port-forward svc/bitcoin-mainnet 8332:8332

# Test the RPC
curl -u rpc:rpc --data-binary \
  '{"jsonrpc":"1.0","method":"getblockchaininfo","params":[]}' \
  http://localhost:8332/
```

## More Examples

### Ethereum with Nethermind

```yaml
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: ethereum-mainnet
spec:
  chain: ethereum
  network: mainnet
  nodeType: rpc
  client: nethermind
  nodeGroup: heavy
  storage:
    size: 2Ti
    storageClass: fast-ssd
  resources:
    requests:
      cpu: "4"
      memory: 16Gi
    limits:
      cpu: "8"
      memory: 32Gi
  rpc:
    enabled: true
    port: 8545
    wsPort: 8546
  health:
    blockLagThreshold: 30
```

### Solana

```yaml
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: solana-mainnet
spec:
  chain: solana
  network: mainnet
  nodeType: rpc
  nodeGroup: heavy
  storage:
    size: 2Ti
    storageClass: fast-ssd
  resources:
    requests:
      cpu: "8"
      memory: 64Gi
    limits:
      cpu: "16"
      memory: 128Gi
  rpc:
    enabled: true
    port: 8899
```

All sample manifests are available in `config/samples/`.

## Common Operations

### Scale to Zero (Pause a Node)

Set `replicas: 0` to stop the node pod while preserving the PVC:

```bash
kubectl patch blockchainnode bitcoin-mainnet --type merge -p '{"spec":{"replicas":0}}'
```

Resume by setting replicas back to 1:

```bash
kubectl patch blockchainnode bitcoin-mainnet --type merge -p '{"spec":{"replicas":1}}'
```

### Upgrade the Node Image

Override the default image with a custom tag:

```bash
kubectl patch blockchainnode bitcoin-mainnet --type merge -p '{
  "spec": {
    "image": {
      "repository": "lncm/bitcoind",
      "tag": "v29.0"
    }
  }
}'
```

The operator updates the StatefulSet, which triggers a rolling restart.

### Change Resources

```bash
kubectl patch blockchainnode bitcoin-mainnet --type merge -p '{
  "spec": {
    "resources": {
      "requests": {"cpu": "4", "memory": "8Gi"},
      "limits": {"cpu": "8", "memory": "16Gi"}
    }
  }
}'
```

### Add Extra CLI Arguments

```bash
kubectl patch blockchainnode bitcoin-mainnet --type merge -p '{
  "spec": {
    "extraArgs": ["-maxmempool=300", "-mempoolexpiry=72"]
  }
}'
```

### Delete a Node

```bash
kubectl delete blockchainnode bitcoin-mainnet
```

This deletes the StatefulSet, Services, and ConfigMap. The PVC is **not** deleted automatically -- you must remove it manually if you want to reclaim the storage:

```bash
kubectl delete pvc data-bitcoin-mainnet-0
```

### Disable Snapshot Bootstrap

To sync from genesis instead of using a MinIO snapshot:

```yaml
spec:
  snapshot:
    disabled: true
```

## Troubleshooting

### Node stuck in Pending phase

**Symptoms:** Phase remains `Pending` for more than a few minutes.

**Check:**
1. Is the StatefulSet created? `kubectl get statefulset <name>`
2. Is the pod scheduling? `kubectl describe pod <name>-0` -- look for events about insufficient resources, missing StorageClass, or unschedulable nodes.
3. Is the PVC bound? `kubectl get pvc data-<name>-0` -- if Pending, the StorageClass may not exist or the volume cannot be provisioned.

### Node stuck in Syncing phase

**Symptoms:** Phase is `Syncing` but block height is not advancing.

**Check:**
1. Check pod logs: `kubectl logs <name>-0`
2. Check peer count in `kubectl get blockchainnodes` -- zero peers means the node cannot connect to the network.
3. For chains with long startup (TRON, TON, Cardano), syncing can take hours or days. Check the `ETA` column.
4. The operator has stall detection with StallExempt logic for known slow phases (Ethereum pipeline stages, Stellar bucket apply, TON dump download). If the node is genuinely stalled, it will eventually transition to Degraded.

### Node in Degraded phase

**Symptoms:** Phase is `Degraded`.

**Check:**
1. `kubectl describe blockchainnode <name>` -- look at the Conditions section for details.
2. The node may have fallen behind the chain tip beyond the `blockLagThreshold`.
3. The operator auto-restarts Degraded nodes after `degradedTimeoutMinutes` (default 15 min). To disable: set `spec.health.degradedTimeoutMinutes: 0`.
4. Check pod logs for RPC errors or crash loops.

### Snapshot init container failing

**Symptoms:** Pod is in `Init:Error` or `Init:CrashLoopBackOff`.

**Check:**
1. Check init container logs: `kubectl logs <name>-0 -c snapshot-restore`
2. Verify `MINIO_ENDPOINT` is set on the operator deployment.
3. Verify the MinIO bucket `snapshots-<chain>` exists and contains the snapshot.
4. To skip snapshots: set `spec.snapshot.disabled: true`.

### Operator not reconciling

**Check:**
1. Operator pod is running: `kubectl get pods -n chainplane-system`
2. Operator logs: `kubectl logs -n chainplane-system deploy/chainplane-controller-manager`
3. CRDs are installed: `kubectl get crd blockchainnodes.nodes.chainplane.io`

### Events

The operator emits Kubernetes events for important lifecycle transitions:

```bash
kubectl get events --field-selector involvedObject.name=bitcoin-mainnet
```
