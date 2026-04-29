# Registry Support

## Supported Registries

| Registry | Client | Auth method | Example images |
|---|---|---|---|
| `docker.io` | `dockerHubClient` | Docker Hub v2 API, anonymous | `dashpay/dashd`, `parity/polkadot` |
| `ghcr.io` | `ghcrClient` | OCI v2 + anonymous Bearer token | `ghcr.io/bnb-chain/bsc` |
| `us-docker.pkg.dev` | `ociClient` | OCI v2 + anonymous Bearer token | Google Artifact Registry |
| `public.ecr.aws` | `ociClient` | OCI v2 + anonymous Bearer token | Amazon ECR Public |

All clients are read-only and use only public/anonymous endpoints — no credentials are required in the operator.

## How VersionPolicy Works

Each chain adapter declares a `VersionPolicy` struct:

```go
type VersionPolicy struct {
    Registry   string // e.g. "docker.io", "ghcr.io"
    Image      string // e.g. "bnb-chain/bsc"
    TagPattern string // regex, e.g. "^v\\d+\\.\\d+\\.\\d+$"
}
```

When the catalog controller polls a registry:

1. All available tags for the image are fetched via the registry's tag listing API.
2. Tags are filtered by `TagPattern` (regex match). Tags that do not match are discarded.
3. Remaining tags are normalized to semver (stripping leading `v`, handling two-part versions like `1.4` → `1.4.0`).
4. Tags are sorted in descending semver order.
5. The highest version becomes the `latestTag` written into the `ChainVersionCatalog` status for that chain.

The `ChainInstanceReconciler` compares the running pod's image tag against `latestTag`. If they differ and `spec.autoUpgrade` is enabled, a rolling restart is triggered with the new tag.

## Chains Without Auto-Tracking

The following 5 adapters publish only a `:latest` tag and do not include version information in their image tags. Auto-tracking via `VersionPolicy` is not possible for these chains:

| Chain | Image |
|---|---|
| Aptos | varies |
| Aurora | varies |
| HyperLiquid | varies |
| MegaETH | varies |
| Monad | varies |

For these chains, `VersionPolicy` is set to `nil` in the adapter. Users must pin an explicit image in `spec.image` and update it manually.

## Adding a New Registry

To add support for a registry not listed above:

1. **Implement the `TagLister` interface** in `internal/registry/`:

    ```go
    type TagLister interface {
        ListTags(ctx context.Context, image string) ([]string, error)
    }
    ```

2. **Register the client** in `internal/registry/client.go` inside `NewClient()`:

    ```go
    func NewClient(registry string) (TagLister, error) {
        switch {
        case registry == "docker.io":
            return newDockerHubClient(), nil
        case registry == "ghcr.io":
            return newGHCRClient(), nil
        case strings.HasSuffix(registry, ".pkg.dev") ||
             registry == "public.ecr.aws":
            return newOCIClient(registry), nil
        // add your case here
        default:
            return nil, fmt.Errorf("unsupported registry: %s", registry)
        }
    }
    ```

3. **Use the new registry** in the adapter's `VersionPolicy`:

    ```go
    func (a *MyChainAdapter) VersionPolicy() *registry.VersionPolicy {
        return &registry.VersionPolicy{
            Registry:   "my.registry.io",
            Image:      "org/mychain",
            TagPattern: `^v\d+\.\d+\.\d+$`,
        }
    }
    ```

4. Add a unit test in `internal/registry/` that mocks the HTTP responses for the new registry's tag listing endpoint.
