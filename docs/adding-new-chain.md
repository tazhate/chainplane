# Adding a New Chain Adapter

Step-by-step guide for adding a new blockchain to the operator.

## Overview

Each blockchain is implemented as a `ChainAdapter` in `internal/adapters/`. The operator uses adapters to generate config files, build containers, and check node health via RPC.

## Step 1: Add Chain Constant

Edit `api/v1alpha1/blockchainnode_types.go`:

```go
const (
    // ... existing chains
    ChainMyChain Chain = "mychain"
)
```

## Step 2: Create Adapter File

Create `internal/adapters/mychain.go`. Choose the right base:

| Base | When to Use | Examples |
|------|-------------|---------|
| `baseAdapter` | Generic chain | Solana, Stellar, Cardano |
| `utxoAdapter` | Bitcoin-like (getblockchaininfo RPC) | Bitcoin, Litecoin, Dash |
| EVM via `evmHealthCheck()` | EVM-compatible (eth_syncing RPC) | Ethereum, BSC, Polygon, Avalanche |

### EVM Chain (simplest)

Most L2s are EVM-compatible. Use this template:

```go
package adapters

import (
    "context"

    corev1 "k8s.io/api/core/v1"
    nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

const defaultMyChainImage = "registry/image:tag"

type mychainAdapter struct {
    baseAdapter
}

func init() {
    Register(nodesv1alpha1.ChainMyChain, &mychainAdapter{
        baseAdapter: baseAdapter{livenessPort: 8545},
    })
}

func (a *mychainAdapter) DefaultImage(_ string) string {
    return defaultMyChainImage
}

func (a *mychainAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
    // Return empty if chain needs no config file
    return "", "", nil
}

func (a *mychainAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
    return evmHealthCheck(ctx, rpcURL)
}

func (a *mychainAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
    return evmPorts(30303) // standard EVM P2P port
}
```

### UTXO Chain (Bitcoin-family)

```go
func init() {
    Register(nodesv1alpha1.ChainMyUTXO, &myutxoAdapter{
        utxoAdapter: utxoAdapter{
            baseAdapter:    baseAdapter{livenessPort: 8332},
            rpcUserEnv:     "MYUTXO_RPC_USER",
            rpcPasswordEnv: "MYUTXO_RPC_PASS",
            defaultUser:    "rpc",
            defaultPass:    "rpc",
        },
    })
}

func (a *myutxoAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
    return utxoHealthCheck(ctx, rpcURL, &a.utxoAdapter, "synced-exempt")
}
```

### Custom Chain

For non-EVM, non-UTXO chains, implement `HealthCheck` directly by calling the chain's RPC.

## Step 3: Implement DefaultResources()

Every adapter **must** implement `DefaultResources()`. Return a `ResourceDefaults` struct with the recommended CPU request, memory request, and storage size based on the chain's official node documentation. The validating webhook uses these values to warn operators when a `BlockchainNode` is created with resources below the recommended minimums.

```go
import "k8s.io/apimachinery/pkg/api/resource"

func (a *mychainAdapter) DefaultResources() ResourceDefaults {
    return ResourceDefaults{
        CPURequest:    resource.MustParse("4"),
        MemoryRequest: resource.MustParse("16Gi"),
        Storage:       resource.MustParse("600Gi"),
    }
}
```

Use values from the official chain documentation or node operator guides. When in doubt, err on the side of slightly higher recommendations — the webhook issues a warning, not a rejection, so operators can still override downward.

## Step 4: Implement VersionPolicy()

If the chain's images are published with versioned tags (e.g. `v1.2.3`) on a supported registry, implement `VersionPolicy()` to enable `ChainVersionCatalog` auto-tracking:

```go
func (a *mychainAdapter) VersionPolicy() ChainVersionPolicy {
    return ChainVersionPolicy{
        Registry:   "ghcr.io",
        Repository: "myorg/mychain",
        TagPattern: `^v\d+\.\d+\.\d+$`,
    }
}
```

Supported registries: `docker.io`, `ghcr.io`, `us-docker.pkg.dev` (Google Artifact Registry), `public.ecr.aws` (Amazon ECR Public).

If the chain only publishes a `:latest` tag (no versioned tags), **skip this method** — the base adapter's no-op implementation will be used and the chain will simply not appear in the version catalog.

## Step 5: Optional Interfaces

Implement any of these if needed:

| Interface | When |
|-----------|------|
| `ContainerArgsProvider` | Chain needs CLI flags (e.g. `--datadir`, `--rpc-port`) |
| `ContainerCommandProvider` | Override container entrypoint (e.g. complex bash wrapper) |
| `ContainerEnvProvider` | Inject environment variables |
| `NodePortProvider` | Chain needs external UDP/TCP ports (e.g. TON ADNL) |
| `StartupProbeProvider` | Long startup time (e.g. TRON Java, >5min) |
| `InitContainerProvider` | Custom init containers (e.g. SUI snapshot download) |

## Step 6: Add Snapshot Bucket

Edit `internal/snapshot/snapshot.go`, add to `bucketForChain()`:

```go
case nodesv1alpha1.ChainMyChain:
    return "snapshots-mychain"
```

## Step 7: Add Webhook Validation

Edit `api/v1alpha1/blockchainnode_webhook.go`, add to `validationRegistry`:

```go
nodesv1alpha1.ChainMyChain: {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
```

## Step 8: Create Sample CR

Create `config/samples/nodes_v1alpha1_blockchainnode_mychain.yaml`:

```yaml
apiVersion: nodes.chainplane.io/v1alpha1
kind: BlockchainNode
metadata:
  name: mychain-mainnet-01
  namespace: blockchain-nodes
spec:
  chain: mychain
  network: mainnet
  nodeType: rpc
  nodeGroup: heavy
  rpc:
    enabled: true
    httpPort: 8545
  storage:
    size: "500Gi"
    storageClassName: fast-nvme
  resources:
    requests:
      cpu: "4"
      memory: "16Gi"
    limits:
      cpu: "8"
      memory: "32Gi"
  health:
    blockLagThreshold: 30
    degradedTimeoutMinutes: 15
```

Add it to `config/samples/kustomization.yaml`.

## Step 9: Update Documentation

1. Add chain to `docs/adapters.md` table and per-chain section
2. Add to README.md supported chains table
3. Update Helm CRD if enum changed: copy regenerated CRD to `charts/chainplane/templates/crds/`

## Step 10: Tests

Add to existing test files:
- `internal/adapters/adapters_test.go` — chain is auto-covered by `allChains` loop tests if added to the list
- `api/v1alpha1/blockchainnode_webhook_test.go` — add validation test case

## Checklist

- [ ] Chain constant in `blockchainnode_types.go`
- [ ] Adapter file in `internal/adapters/`
- [ ] `init()` registers adapter
- [ ] `DefaultImage()` returns a real, versioned image
- [ ] `HealthCheck()` works with the chain's RPC
- [ ] `ConfigTemplate()` returns valid config (or empty)
- [ ] `DefaultResources()` returns recommended CPU/memory/storage from official docs
- [ ] `VersionPolicy()` implemented (or intentionally skipped for `:latest`-only images)
- [ ] Snapshot bucket in `snapshot.go`
- [ ] Webhook validation entry
- [ ] Sample CR YAML
- [ ] Added to `allChains` in tests
- [ ] Documentation updated
- [ ] `go build ./...` passes
- [ ] `go test ./internal/adapters/` passes
