# Contributing to blockchain-node-operator

Thank you for your interest in contributing. This document covers the development workflow, testing, and how to add support for new blockchain chains.

## Prerequisites

- **Go 1.23+** (version pinned in `go.mod`)
- **Docker 17.03+** (for building container images)
- **kubectl 1.11+** (for cluster interaction)
- **Kind** (for local development and E2E tests)
- **golangci-lint v1.63.4** (installed automatically by `make lint`)

## Development Setup

```bash
# Clone the repository
git clone https://github.com/tazhate/blockchain-node-operator.git
cd blockchain-node-operator

# Install tool dependencies (kustomize, controller-gen, envtest, golangci-lint)
# These are downloaded to ./bin/ automatically on first use.
make build
```

The project uses [Kubebuilder](https://book.kubebuilder.io/) scaffolding with controller-runtime.

## Running Tests

### Unit tests

```bash
make test
```

This generates CRDs, runs code generation, formats code, runs `go vet`, and then executes all unit tests with coverage (output in `cover.out`). Tests use the envtest framework (a lightweight API server + etcd).

### End-to-end tests

```bash
# Start a Kind cluster (if not already running)
kind create cluster

# Run E2E tests
make test-e2e
```

E2E tests use Ginkgo and require a running Kind cluster. They build and load the operator image locally, install CRDs, deploy the controller, and verify reconciliation against real Kubernetes resources.

### Linting

```bash
make lint          # Run golangci-lint
make lint-fix      # Run golangci-lint with auto-fix
make lint-config   # Verify linter configuration
```

## Running Locally

```bash
# Install CRDs into your current kubeconfig cluster
make install

# Run the controller on your host (outside the cluster)
make run

# In another terminal, apply a sample manifest
kubectl apply -f config/samples/nodes_v1alpha1_blockchainnode_bitcoin.yaml
kubectl get blockchainnodes -w
```

To deploy the controller inside the cluster:

```bash
make deploy IMG=<your-registry>/blockchain-node-operator:latest
```

## Code Generation

After modifying CRD types in `api/v1alpha1/blockchainnode_types.go`:

```bash
make manifests   # Regenerate CRDs, RBAC, webhooks
make generate    # Regenerate DeepCopy methods
```

Always run these before committing changes to the API types.

## Project Structure

```
api/v1alpha1/              CRD type definitions
internal/adapters/         Chain-specific adapter implementations
internal/controller/       Reconciliation controllers
internal/health/           Health trigger system and replacement workflow
internal/metrics/          Prometheus metrics
internal/snapshot/         MinIO snapshot bootstrap
config/crd/               Generated CRD manifests
config/samples/           Example BlockchainNode manifests (one per chain)
charts/                   Helm chart
test/e2e/                 End-to-end tests
```

## How to Add a New Chain Adapter

Each blockchain is supported through an adapter that implements the `ChainAdapter` interface. To add a new chain:

### 1. Add the chain constant

In `api/v1alpha1/blockchainnode_types.go`, add a new `Chain` constant and include it in the `+kubebuilder:validation:Enum` comment on the `Chain` type:

```go
ChainMyChain Chain = "mychain"
```

### 2. Create the adapter file

Create `internal/adapters/mychain.go`:

There are three base patterns depending on the chain type:

**EVM chain (simplest — use `evmHealthCheck`):**

```go
package adapters

import (
    "context"
    corev1 "k8s.io/api/core/v1"
    nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

const defaultMyChainImage = "mychain/op-geth:v1.0.0"

type mychainAdapter struct{ baseAdapter }

func init() {
    Register(nodesv1alpha1.ChainMyChain, &mychainAdapter{
        baseAdapter: baseAdapter{livenessPort: 8545},
    })
}

func (a *mychainAdapter) DefaultImage(_ string) string { return defaultMyChainImage }

func (a *mychainAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
    return "config.toml", mychainConfig, nil
}

func (a *mychainAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
    return evmHealthCheck(ctx, rpcURL)
}

func (a *mychainAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
    return evmPorts(30303)
}

const mychainConfig = `# mychain config
[Eth]
NetworkId = 12345
`
```

**Bitcoin-family UTXO chain (use `utxoAdapter`):**

```go
type mychainAdapter struct{ utxoAdapter }

func init() {
    Register(nodesv1alpha1.ChainMyChain, &mychainAdapter{
        utxoAdapter: utxoAdapter{
            baseAdapter:    baseAdapter{livenessPort: 8332},
            rpcUserEnv:     "MYCHAIN_RPC_USER",
            rpcPasswordEnv: "MYCHAIN_RPC_PASSWORD",
            defaultUser:    "rpc",
            defaultPass:    "rpc",
        },
    })
}

func (a *mychainAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
    return utxoHealthCheck(ctx, rpcURL, &a.utxoAdapter, "synced-exempt")
}
```

**Custom chain (implement `HealthCheck` directly):**

```go
func (a *mychainAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
    result, err := callRPC(ctx, rpcURL, "mychain_getStatus", nil)
    // parse and return SyncStatus{...}
}
```

`baseAdapter` satisfies the `LivenessProbe`, `NodeSelector`, and several other interface methods
with sensible defaults. `init()` with `Register()` is required — it wires the adapter into the
global registry so the controller can look it up by chain name.

### 3. Implement optional interfaces (if needed)

Depending on your chain's requirements, implement any of these optional interfaces on your adapter:

| Interface | When to implement |
|-----------|-------------------|
| `ContainerCommandProvider` | Chain needs a custom entrypoint (most chains do) |
| `ContainerArgsProvider` | Chain needs CLI arguments beyond the command |
| `ContainerEnvProvider` | Chain needs specific environment variables |
| `StartupProbeProvider` | Chain has a long startup (snapshot download, DB replay) |
| `NodePortProvider` | Chain uses UDP P2P that requires host-accessible ports |
| `InitContainerProvider` | Chain needs a pre-start init container (e.g., snapshot download tool) |

### 4. Add a sample manifest

Create `config/samples/nodes_v1alpha1_blockchainnode_mychain.yaml` and add it to `config/samples/kustomization.yaml`.

### 5. Regenerate and test

```bash
make manifests generate
make test
```

### 6. Document the adapter

Add a section for your chain in `docs/adapters.md` covering: default image, ports, health check method, configuration notes, storage requirements, and any special features.

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use [Ginkgo](https://onsi.github.io/ginkgo/) + [Gomega](https://onsi.github.io/gomega/) for tests
- Keep adapter files self-contained -- one file per chain in `internal/adapters/`
- Use `slog` for structured logging (not `fmt.Println` or `log.Printf`)
- Error messages should be lowercase without trailing punctuation
- Wrap errors with context: `fmt.Errorf("doing thing: %w", err)`

## Commit Message Format

Use conventional commit format:

```
type: short description

Context:
- describe what was investigated, debugged, or analyzed
- mention concrete steps taken
- write from first person as a developer
- note approximate time spent
```

Types: `fix`, `feat`, `refactor`, `config`, `deploy`, `docs`, `test`

Examples:
- `feat: add Aptos chain adapter with state sync support`
- `fix: prevent false stall detection during Ethereum pipeline stages`
- `refactor: extract common UTXO health check logic into shared helper`

## Pull Request Process

1. Fork the repository and create a feature branch
2. Ensure all tests pass: `make test && make lint`
3. If adding a new chain, include the adapter, sample manifest, and adapter docs
4. If modifying CRD types, run `make manifests generate` and commit the generated files
5. Keep PRs focused -- one feature or fix per PR
6. CI runs automatically on push and PR: unit tests, E2E tests, and lint

## CI Pipeline

The project has three GitHub Actions workflows:

- **Tests** (`test.yml`) -- runs `make test` on every push and PR
- **E2E Tests** (`test-e2e.yml`) -- creates a Kind cluster and runs `make test-e2e`
- **Lint** (`lint.yml`) -- runs golangci-lint v1.63.4

## License

By contributing, you release your contributions into the public domain under the [Unlicense](LICENSE). No restrictions apply.
