# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog 1.1](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

The CRD API is currently `v1alpha2` — breaking changes may occur in any minor
release until the API is promoted to `v1beta1`.

## [Unreleased]

## [0.5.0] - 2026-10-02

A smoke-test pass over every chain (new `cmd/chainsmoke`) showed that many
adapters ran but not as the chain they claimed, or wrote outside the PVC.
This release fixes those adapters, moves OP Stack to op-reth and makes
version tracking reliable.

### Upgrade notes

- **UTXO chains (bitcoin, dash, litecoin, dogecoin)** restart once: the
  config now uses `rpcauth` instead of a plaintext password. Nodes that ran
  without a `<name>-rpc-credentials` Secret (all dogecoin, any node on
  `rpc`/`rpc`) get a generated random password; external clients using
  `rpc`/`rpc` stop working.
- **ethereum / ethereum-archive** and **OP Stack** nodes resync from
  scratch on `/data`. Before, every ethereum client wrote to the container
  filesystem, and OP chains ran op-geth as Ethereum mainnet; there is
  nothing on the PVC to migrate.
- **optimism** on op-reth needs `spec.snapshot` (pre-Bedrock state).
- Chains without a usable public image now require `spec.image`; the
  webhook rejects them otherwise and the controller marks them
  `Degraded/ImageRequired`. See the README chain table.

### Security

- UTXO RPC credentials are no longer copied into the operator's process
  environment, where instances leaked them into each other. Each node has
  its own Secret, the ConfigMap holds only an `rpcauth` hash, and the
  exporter reads the Secret via `secretKeyRef` (#25).
- grpc v1.84.0 and golang.org/x/text v0.42.0 (GO-2026-6348, GO-2026-6061,
  GO-2026-5970) (#24).

### Added

- `cmd/chainsmoke`: laptop smoke tests. Level 0 checks every default image
  manifest; level 1 runs the rendered pod (with sidecars) under docker and
  verifies that data lands on the PVC and the chain id / CometBFT network
  matches `internal/adapters/chain_identity.go` (#26, #40, #45, #48, #51, #52).
- OP Stack chains run op-reth with an op-node sidecar; Base runs
  `ghcr.io/base/node` (#50).
- Postgres sidecar and full external-node config for ZK Stack chains
  (zksync, abstract, lens, cronos-zkevm) (#37).
- Shared Cosmos bootstrap: `init`, chain-registry peers, state sync and
  sha256-verified genesis, so Cosmos nodes start from an empty PVC (#46).
- `versioncheck`: per-client images, `NO MATCH` and `MAJOR AVAILABLE`
  statuses, same-line updates under a held major, `--allow-major`, exit 1
  on blind spots, named `version` group in tag patterns (#16, #17).

### Changed

- Kubernetes libraries 0.34, controller-runtime 0.22, Go 1.26 (#14).
- Default images refreshed and repointed to live repositories, including
  major bumps verified by smoke tests: nethermind 2.0, gaia v28, cardano 11,
  stellar-core 29 (Protocol 29), kaia v2.2.2, harmony v2026.1.3 (#13, #18,
  #34, #36, #39).
- Health traffic validation uses EndpointSlices (#20).
- CI: actions pinned by SHA, no auto-merge, drift and govulncheck steps,
  release builds the snapshot-restore image (#21, #22, #28).

### Fixed

- ethereum: every client (nethermind, geth, erigon, reth) now runs on
  `/data` with the right network; the geth TOML was invalid (#47).
- Adapters that passed geth-only metrics flags to other clients or never
  passed their config: gnosis, fuse, ethereum-classic (core-geth), core,
  bsc, bor, metis, the Nitro family, goat, taiko, rootstock, viction,
  kusama, moonbeam, moonriver, lighthouse, juno, solana (which ran a private
  dev cluster), sui and more (#41, #44, #51).
- Images without an entrypoint ran our args as the command (haqq, kava,
  dymension, axelar, filecoin, thundercore, morph, plasma) (#46).
- dogecoin ignored its config and PVC; the litecoin exporter clashed with
  RPC on 9332; UTXO metrics were never scraped (#27).
- `versioncheck` read only the first page of GHCR tags and a random sample
  of GAR tags, and lost Docker Hub chains to rate limits (#12, #15).
- Controller: adapters without a config file failed reconcile on an empty
  ConfigMap key (#43); dropped errors and dead `RequeueAfter` (#19);
  `make generate` failure on the validator (#38).

### Removed

- Default images for chains with no usable public image, a shut-down
  network or a stack the adapter does not model yet; they now require
  `spec.image` (#33, #39, #49, #50). Follow-ups: #29, #30, #31, #32, #35, #42.
- Stale `nodes.k8s-bch.io` CRDs (#23).

## [0.4.0] - 2026-06-14

### Added

- `polygon-zkevm` migrated from the archived `zkevm-node` to `cdk-erigon`
  (Erigon-based client). New `hermez-mainnet` config, image
  `ghcr.io/0xpolygon/cdk-erigon`, and bumped resource defaults.

### Changed

- `versioncheck` now picks the semver-max stable tag instead of the first
  stable tag, paginates Docker Hub so the newest release is actually seen,
  rejects op-stack service-build tags, and talks to Google Artifact
  Registry without a token (parsing its nested tag shape).
- Refreshed ~48 default node images to their latest stable tags.
- Repointed 13 chains whose image repositories had moved or were renamed:
  morph, dogecoin, kava, sei, dymension, solana, core, metis, taiko,
  manta-pacific, plasma, boba-eth, hashkey.

### Removed

- Dropped automatic version tracking for chains that publish no public,
  version-tagged image (cronos, telos, fantom, wemix, shibarium,
  bittorrent, sonic, zircuit, moca). Their pinned defaults are unchanged.

## [0.3.0] - 2026-05-06

### Changed

- Updated blockchain node default images.

## [0.2.3] - 2026-04-29

### Fixed

- Helm chart now ships the `ChainVersionCatalog` CRD too. Previously only
  `chaininstances.chains.chainplane.io` was packaged; the operator started
  but logged `no matches for kind "ChainVersionCatalog"` repeatedly because
  controller-runtime expects the CRD to exist in the cluster at startup.
  Workaround for v0.2.0–v0.2.2 installs:
  `kubectl apply -f https://raw.githubusercontent.com/tazhate/chainplane/master/config/crd/bases/chains.chainplane.io_chainversioncatalogs.yaml`

## [0.2.2] - 2026-04-29

### Fixed

- Operator container image now carries `org.opencontainers.image.source`
  and other OCI labels, so the GHCR package gets correctly associated with
  the GitHub repository and becomes visible in the repo Packages sidebar.
  Without this label, GHCR creates the package as orphaned/private — even
  though `docker buildx --push` reports success — and `helm install` fails
  with `403 Forbidden` from the anonymous pull token endpoint.
- `release.yml`: disabled `provenance` and `sbom` attestations (they create
  extra manifests unrelated to package association). Added explicit OCI
  labels via `docker/build-push-action` `labels:` input as belt-and-braces.

## [0.2.1] - 2026-04-29

### Fixed

- Helm chart `appVersion` aligned with the container image tag (`v0.2.1`).
  Previously the chart shipped with `appVersion: "0.1.0"` while the image
  was published as `v0.1.0` — this caused `ImagePullBackOff` on default
  install because the chart attempted to pull `:0.1.0` (no `v` prefix)
  which never existed.
- `gofmt` formatting of license-header comment blocks (tab vs 4-space
  indent before the LICENSE-2.0 URL inside `/* */`).
- `staticcheck` ST1005 in `internal/adapters/cosmos.go` — error string
  decapitalised (`"Cosmos health probe"` → `"cosmos health probe"`).
- `--version` references in README install snippets bumped to `0.2.1`.

## [0.2.0] - 2026-04-29

### Changed (BREAKING)

- **API group renamed** `nodes.chainplane.io` → `chains.chainplane.io`.
- **API version bumped** `v1alpha1` → `v1alpha2`.
- **Kind renamed** `BlockchainNode` → `ChainInstance`. The `Chain` enum
  (chain protocol identifiers such as `bitcoin`, `ethereum`) is preserved.
- **Go types renamed** in `api/v1alpha2`:
  `BlockchainNode` → `ChainInstance`, `BlockchainNodeSpec` → `ChainInstanceSpec`,
  `BlockchainNodeStatus` → `ChainInstanceStatus`, `BlockchainNodeList` → `ChainInstanceList`,
  `BlockchainNodeReconciler` → `ChainInstanceReconciler`.
- **Adapter base types renamed**: `baseAdapter` → `protocolAdapter`,
  `utxoAdapter` → `utxoProtocolAdapter`. Files renamed accordingly.
- **License changed** from Unlicense to **Apache License 2.0**. Every Go source
  file now carries an SPDX header. `NOTICE` and `AUTHORS` files added.

Migration: edit existing manifests to use the new `apiVersion`, `kind` and the
new file/folder paths; the spec and status field shapes are unchanged.

## [0.1.0] - 2026-04-28

Initial OSS release.

### Added

- **102 blockchain adapters** spanning Ethereum L1/archive/beacon, Bitcoin and
  UTXO-family, BSC, TRON, Solana, Cosmos ecosystem (Cosmos Hub, Osmosis, Sei,
  Evmos, Kava, Axelar, Dymension), Polkadot/Kusama, Substrate parachains
  (Moonbeam, Moonriver), 46 EVM L2s (Arbitrum, Optimism, Base, zkSync, Linea,
  Scroll, Mantle, Taiko, all OP Stack chains, etc.) and others (Aptos, Sui,
  NEAR, TON, Cardano, Stellar, Filecoin, XRP, etc.).
- **`ChainInstance` CRD** for declarative node lifecycle: chain, network,
  client, image, storage, RPC, snapshot bootstrap, health monitoring.
- **`ChainVersionCatalog` CRD** for tracking the latest container image
  versions of supported chains via a configurable polling interval.
- **`DefaultResources()` interface** on every adapter — returns recommended
  CPU, memory and storage based on official documentation.
- **`VersionPolicy()` interface** on 96/101 adapters — drives auto-tracking of
  upstream image releases through `ChainVersionCatalog`.
- **OCI v2 registry client** in `internal/registry/oci.go` — supports Google
  Artifact Registry (`us-docker.pkg.dev`) and Amazon ECR Public
  (`public.ecr.aws`) alongside the existing Docker Hub and GHCR clients.
- **Auto-upgrade reconciler** with rolling restart and automatic rollback on
  `CrashLoopBackOff` (≥3 container restarts).
- **Snapshot bootstrap** through MinIO-backed init containers; supports `full`
  and `lite` snapshot variants.
- **Health monitoring** with chain-specific block-lag thresholds, sync stall
  detection, peer count tracking, and auto-restart on degraded timeout.
- **Validating admission webhook** with per-chain resource recommendation
  warnings; defaulting webhook for common spec fields.
- **Prometheus metrics**: `blockchain_node_block_height`,
  `blockchain_node_sync_progress`, `blockchain_node_peers_count`,
  `blockchain_node_phase`, `blockchain_node_restarts_total`,
  `blockchain_node_degraded_duration_seconds`.
- **Fleet Status dashboard** — embedded HTML/JS UI with real-time node table,
  per-node detail, namespace filtering, JSON API and Prometheus metrics.
- **Helm chart** at `charts/chainplane` with HA defaults,
  webhook + cert-manager integration, optional `ServiceMonitor`.
- **Multi-arch container images** (linux/amd64, linux/arm64) published to
  `ghcr.io/tazhate/chainplane`.
- **CI workflows** for unit tests, golangci-lint, Helm validation and
  tag-driven releases.

[Unreleased]: https://github.com/tazhate/chainplane/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/tazhate/chainplane/releases/tag/v0.1.0
