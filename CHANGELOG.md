# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog 1.1](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

The CRD API is currently `v1alpha2` — breaking changes may occur in any minor
release until the API is promoted to `v1beta1`.

## [Unreleased]

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
