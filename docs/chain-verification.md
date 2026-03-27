# Chain Adapter Verification Report

Generated: 2026-03-27

Methodology: For each of the 102 chains, the adapter file was read and the official Docker Hub /
GitHub releases page was searched to verify image name, version tag, ports, snapshot compatibility,
and CLI flags. Status is based on the worst finding across all four dimensions.

## Summary

- ✅ Verified: 38 chains
- ⚠️ Minor issues: 17 chains
- ❌ Needs fix: 47 chains

## Verification Table

| Chain | Image | Ports | Snapshot | CLI | Status | Notes |
|-------|-------|-------|----------|-----|--------|-------|
| abstract | ❌ | ❌ | ❌ | ❌ | ❌ | `abstractchain/op-geth` doesn't exist; Abstract is ZK Stack, not OP Stack |
| aptos | ✅ | ✅ | ✅ | ✅ | ✅ | |
| arbitrum | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | Image `v3.2.1` outdated → use `v3.9.7+` |
| aurora | ❌ | ✅ | ❌ | ❌ | ❌ | `nearprotocol/aurora-relayer` is deprecated; use `nearaurora/srpc2-relayer` (standalone-rpc) |
| avalanche | ✅ | ✅ | ✅ | ✅ | ✅ | |
| axelar | ❌ | ✅ | ❌ | ❌ | ❌ | Version `v0.35.6` outdated → `v1.3.4`; missing `--home /data` flag (snapshot incompatible) |
| base | ✅ | ✅ | ✅ | ❌ | ⚠️ | Missing `--config /config/config.toml` CLI flag → config file never loaded |
| berachain | ❌ | ✅ | ❌ | ❌ | ❌ | Needs separate EL beacon sidecar; single-container setup non-functional |
| bitcoin | ✅ | ✅ | ✅ | ✅ | ✅ | |
| bittorrent | ❌ | ⚠️ | ❌ | ❌ | ❌ | Wrong org (`tronprotocol` vs `bttcprotocol`); BTTC is Polygon CDK (Bor+Delivery dual-client), not single container; invalid TOML sections |
| blast | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | Tag format wrong: `v1.2.0` → `mainnet-v1.7.0` |
| bob | ✅ | ✅ | ✅ | ✅ | ✅ | |
| boba-eth | ✅ | ✅ | ✅ | ✅ | ✅ | |
| bsc | ✅ | ✅ | ✅ | ✅ | ✅ | |
| cardano | ✅ | ✅ | ✅ | ✅ | ✅ | |
| celo | ❌ | ❌ | ❌ | ❌ | ❌ | Celo migrated to OP Stack L2; adapter needs full rewrite |
| core | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | Version `v1.0.14` outdated → `v1.0.22` (mandatory Hermes hard fork support) |
| cosmos | ✅ | ✅ | ✅ | ✅ | ✅ | Only Cosmos SDK chain with correct `--home /data` flag |
| cronos | ✅ | ✅ | ✅ | ✅ | ✅ | |
| cronos-zkevm | ❌ | ✅ | ❌ | ❌ | ❌ | Wrong image: `cryptocom/cronos-zkevm-node` → `ghcr.io/cronos-labs/external-node:mainnet-v29.6.0` |
| dash | ❌ | ✅ | ✅ | ✅ | ❌ | Tag `v23.1.0` doesn't exist; latest is `v22.1.2` |
| dogecoin | ❌ | ✅ | ❌ | ✅ | ❌ | No official Docker image published by Dogecoin project |
| doma | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ⚠️ | No official documentation found; chain type unverified; cannot confirm OP Stack lineage or chain ID 13143 |
| dymension | ❌ | ✅ | ❌ | ❌ | ❌ | Missing `--home /data` flag → snapshot incompatible (Cosmos SDK chain) |
| ethereum | ✅ | ✅ | ✅ | ✅ | ✅ | |
| ethereum-archive | ✅ | ✅ | ✅ | ✅ | ✅ | |
| ethereum-beacon | ❌ | ✅ | ✅ | ✅ | ❌ | Lighthouse `v6.0.1` critically outdated → `v8.0.0` (post-Fulu hard fork mandatory) |
| ethereum-classic | ⚠️ | ✅ | ✅ | ⚠️ | ⚠️ | Besu config format partially wrong; ETC support deprecated in Besu; consider switching to `etclabscore/core-geth` |
| everclear | ❌ | ⚠️ | ❌ | ❌ | ❌ | Everclear is Arbitrum Orbit (AnyTrust), NOT OP Stack; `op-geth` cannot sync this chain; use `offchainlabs/nitro-node` |
| evmos | ❌ | ✅ | ❌ | ❌ | ❌ | Missing `--home /data` flag → snapshot incompatible (Cosmos SDK chain) |
| fantom | ✅ | ✅ | ✅ | ✅ | ✅ | |
| filecoin | ❌ | ✅ | ✅ | ✅ | ❌ | Version `v1.30.0` outdated → `v1.35.0` |
| fraxtal | ✅ | ✅ | ✅ | ✅ | ✅ | |
| fuse | ❌ | ✅ | ✅ | ❌ | ❌ | `:latest` tag; migrated to Nethermind but config uses geth-style TOML |
| gnosis | ✅ | ✅ | ✅ | ✅ | ✅ | |
| gnosis-beacon | ✅ | ✅ | ✅ | ✅ | ✅ | |
| goat | ⚠️ | ✅ | ✅ | ⚠️ | ⚠️ | Image on GHCR not Docker Hub; `v0.1.0` outdated → `v0.2.3+`; `L1_RPC_URL` → Ethereum wrong (GOAT L1 is Bitcoin) |
| gravity-alpha | ❌ | ❌ | ❌ | ❌ | ❌ | Completely wrong: Gravity Alpha is Arbitrum Nitro (Celestia DA); must use `ghcr.io/celestiaorg/nitro:v3.6.8`, ports 8547/8548 |
| haqq | ❌ | ⚠️ | ✅ | ⚠️ | ❌ | Wrong Docker Hub org: `haqq-network/haqq` → `alhaqq/haqq:v1.8.1`; Cosmos P2P port should be 26656, not 30303 |
| harmony | ✅ | ✅ | ✅ | ✅ | ✅ | |
| hashkey | ❌ | ✅ | ✅ | ⚠️ | ❌ | Unverified org `hashkeychain`; official GitHub org is `hashkey-chain`; image existence unconfirmed |
| hemi | ❌ | ❌ | ❌ | ❌ | ❌ | `hemilabs/heminetwork` is Bitcoin-finality daemon suite, not EVM execution layer; `v0.7.0` outdated → `v0.11.4+` |
| hyperliquid | ❌ | ✅ | ⚠️ | ⚠️ | ❌ | `hyperliquid/hl-visor:latest` doesn't exist on Docker Hub; Hyperliquid distributes binary, no official Docker image |
| immutable-zkevm | ❌ | ✅ | ❌ | ❌ | ❌ | `immutable/zkevm-node:v0.7.0` doesn't exist; use `ghcr.io/immutable/immutable-geth/immutable-geth` |
| ink | ✅ | ✅ | ✅ | ✅ | ✅ | |
| katana | ⚠️ | ✅ | ✅ | ⚠️ | ⚠️ | Katana uses `cdk-opgeth` (Polygon CDK + AggLayer), not vanilla `op-geth` |
| kava | ❌ | ✅ | ❌ | ❌ | ❌ | Missing `--home /data` flag → snapshot incompatible (Cosmos SDK chain) |
| klaytn | ❌ | ✅ | ✅ | ✅ | ❌ | Version `v1.0.3` critically outdated → `v2.2.0` (Kaia rebrand) |
| kroma | ❌ | ✅ | ✅ | ✅ | ❌ | Wrong image name: `op-geth` → correct repo uses `geth` tag in kroma-network org |
| kusama | ✅ | ✅ | ✅ | ✅ | ✅ | |
| lens | ❌ | ❌ | ❌ | ❌ | ❌ | Wrong software: Lens is ZK Stack (Matter Labs), not OP Stack; use `matterlabs/external-node`; ports 3060/3061 |
| linea | ✅ | ✅ | ✅ | ❌ | ⚠️ | Wrong CLI flag: `--l1-rpc-url` → `--plugin-linea-l1-rpc-endpoint` |
| lisk | ❌ | ✅ | ✅ | ✅ | ❌ | Wrong image: `lisk/lisk-core` is pre-L2 migration; Lisk is now OP Stack L2 using `op-geth` |
| litecoin | ✅ | ✅ | ✅ | ✅ | ✅ | |
| manta-pacific | ✅ | ✅ | ✅ | ✅ | ✅ | |
| mantle | ✅ | ✅ | ✅ | ✅ | ✅ | |
| megaeth | ❌ | ✅ | ❌ | ❌ | ❌ | `megaeth/megaeth-node:latest` doesn't exist; MegaETH uses custom revm engine; no public node image available |
| metis | ✅ | ✅ | ✅ | ✅ | ✅ | |
| mezo | ❌ | ✅ | ❌ | ❌ | ❌ | `mezoportal/mezo-node:v0.1.0` doesn't exist; official client is `mezod` (github.com/mezo-org/mezod) |
| moca | ❌ | ❌ | ❌ | ❌ | ❌ | Moca Chain (ID 6342) is Cosmos/EVMOS-based L1, NOT OP Stack; `op-geth` cannot sync this chain; wrong config format, wrong ports |
| mode | ✅ | ✅ | ✅ | ✅ | ✅ | |
| monad | ❌ | ✅ | ❌ | ❌ | ❌ | `monadlabs/monad-node:latest` does not exist; Monad node access is permissioned/restricted; config format speculative |
| moonbeam | ❌ | ✅ | ❌ | ❌ | ❌ | Missing `ContainerArgs` → PVC at `/data` ignored, config file never read |
| moonriver | ❌ | ✅ | ❌ | ❌ | ❌ | Missing `ContainerArgs` → PVC at `/data` ignored, config file never read |
| morph | ❌ | ✅ | ✅ | ✅ | ❌ | Wrong registry: image is on `ghcr.io`, not Docker Hub |
| near | ✅ | ✅ | ✅ | ✅ | ✅ | |
| opbnb | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | Should use `bnb-chain/op-geth` fork (BNB-specific patches), not upstream `oplabs/op-geth` |
| optimism | ✅ | ✅ | ✅ | ❌ | ⚠️ | Missing `--config /config/config.toml` CLI flag → config file never loaded |
| osmosis | ❌ | ✅ | ❌ | ❌ | ❌ | Version `v27` outdated → `v31`; missing `--home /data` → snapshot incompatible |
| plasma | ❌ | ⚠️ | ❌ | ❌ | ❌ | Wrong chain identity: Plasma.to is Bitcoin sidechain (BFT consensus), not OP Stack |
| playnance | ❌ | ❌ | ❌ | ❌ | ❌ | PlayBlock is Arbitrum Orbit L3 (AnyTrust on Arbitrum Nova), NOT OP Stack; use `offchainlabs/nitro-node`; wrong ports (8545/8546 vs Nitro 8547/8548) |
| plume | ❌ | ✅ | ❌ | ❌ | ❌ | Wrong software: Plume is Arbitrum Nitro; use `public.ecr.aws/i6b2w2n6/nitro-node:plume-v2.3.2` |
| polkadot | ✅ | ✅ | ✅ | ✅ | ✅ | |
| polygon | ✅ | ✅ | ✅ | ✅ | ✅ | |
| polygon-zkevm | ❌ | ✅ | ✅ | ✅ | ❌ | Wrong image: uses archived `cdk-validium-node` (Polygon CDK PoC); use current `zkevm-node` or `cdk-erigon` |
| ronin | ❌ | ✅ | ✅ | ✅ | ❌ | Docker org migrated from `axieinfinity/ronin` to `ghcr.io/ronin-chain/ronin` |
| rootstock | ✅ | ✅ | ✅ | ✅ | ✅ | |
| scroll | ✅ | ✅ | ✅ | ✅ | ✅ | |
| sei | ❌ | ✅ | ❌ | ❌ | ❌ | Version `v5.9` outdated → `v6.2`; missing `--home /data` → snapshot incompatible |
| shibarium | ❌ | ✅ | ✅ | ❌ | ❌ | Wrong org (`shibaswap` is DEX, not node); wrong architecture (Polygon Bor, not OP Stack); use `shibaone/bor` |
| solana | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | Archived `solanalabs/validator` image; migrate to `solanalabs/agave:v2.x` |
| soneium | ✅ | ✅ | ✅ | ✅ | ✅ | |
| sonic | ❌ | ✅ | ❌ | ❌ | ❌ | `fantomfoundation/sonic` is legacy Fantom Opera client — explicitly incompatible with Sonic mainnet; must use `0xsoniclabs/sonic:v2.1.6+`; no official pre-built image |
| starknet | ✅ | ✅ | ✅ | ✅ | ✅ | |
| stellar | ❌ | ✅ | ✅ | ✅ | ❌ | `:latest` tag on `stellar/stellar-core`; pin to a specific version |
| sui | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | `mysten/sui-node:mainnet` is not a semver tag; use versioned tag (e.g. `v1.x.y`) |
| superseed | ✅ | ✅ | ✅ | ✅ | ✅ | |
| swell | ✅ | ✅ | ✅ | ✅ | ✅ | |
| taiko | ✅ | ✅ | ✅ | ✅ | ✅ | |
| telos | ⚠️ | ✅ | ❌ | ⚠️ | ⚠️ | Telos EVM requires multi-component stack (nodeos + reth/tevmc + evm-rpc); single container incomplete |
| thundercore | ❌ | ✅ | ✅ | ❌ | ❌ | Wrong image name: `thundercore/thundercore` → `thundercore/thunder`; tag scheme wrong (`v2.21.0` vs official `r4.x.x`); config format may not match Thunder consensus client |
| ton | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | `ghcr.io/ton-blockchain/ton:latest` → pin to `v2026.02-1` |
| tron | ✅ | ✅ | ✅ | ✅ | ✅ | |
| unichain | ⚠️ | ✅ | ✅ | ⚠️ | ⚠️ | Tag `v1.101408.0` stale → `v1.101608.0+` (Isthmus hard fork support); `OP_NODE_L1_ETH_RPC` env var targets wrong process (op-node, not op-geth) |
| viction | ❌ | ✅ | ✅ | ✅ | ❌ | Wrong org after rebrand: `tomochain/tomochain` → `buildonviction/node:v2.5.1` |
| wemix | ❌ | ❌ | ✅ | ⚠️ | ❌ | Unverified image; ports wrong (official: HTTP 8588, WS 8598, P2P 8589, not 8545/8546) |
| worldchain | ⚠️ | ✅ | ✅ | ⚠️ | ⚠️ | Tag `v1.101408.0` stale → `v1.101603.5`; `OP_NODE_L1_ETH_RPC` targets op-node, not op-geth |
| xrp | ✅ | ✅ | ✅ | ✅ | ✅ | |
| zero-network | ❌ | ❌ | ❌ | ❌ | ❌ | Wrong software: ZERO Network is ZK Stack (Zerion/Matter Labs), not OP Stack; use `matterlabs/external-node`; ports 3060/3061 |
| zircuit | ⚠️ | ✅ | ✅ | ⚠️ | ⚠️ | Wrong registry: use `ghcr.io/zircuit-labs/l2-geth-public` not Docker Hub; `:latest` non-semver |
| zksync | ✅ | ✅ | ✅ | ✅ | ✅ | |
| zora | ✅ | ✅ | ✅ | ✅ | ✅ | |

---

## Issues Found

### Critical ❌ — Must Fix Before Deployment (40 chains)

#### Wrong software / wrong chain identity

| Chain | Issue | Fix |
|-------|-------|-----|
| **abstract** | Abstract is ZK Stack (Matter Labs), not OP Stack. `abstractchain/op-geth` non-existent. | Rewrite adapter using `matterlabs/external-node`; ports 3060/3061; ZK Stack config |
| **lens** | Lens Chain is ZK Stack, not OP Stack. | Rewrite adapter using `matterlabs/external-node`; ports 3060/3061 |
| **zero-network** | ZERO Network (Zerion) is ZK Stack, not OP Stack. | Rewrite adapter using `matterlabs/external-node`; ports 3060/3061 |
| **plume** | Plume is Arbitrum Nitro, not OP Stack. | Change image to `public.ecr.aws/i6b2w2n6/nitro-node:plume-v2.3.2`; Nitro config format |
| **everclear** | Everclear (chain ID 25327) is Arbitrum Orbit (AnyTrust), NOT OP Stack. `op-geth` cannot sync. | Use `offchainlabs/nitro-node`; add `--parent-chain.connection.url` instead of `L1_RPC_URL`; ports 8547/8548 |
| **gravity-alpha** | Gravity Alpha is Arbitrum Nitro + Celestia DA, not a standalone EVM chain. | Image: `ghcr.io/celestiaorg/nitro:v3.6.8`; ports 8547/8548; add DAS sidecar `ghcr.io/celestiaorg/nitro-das-celestia:v0.5.4`; add snapshot restore |
| **playnance** | PlayBlock is Arbitrum Orbit L3 (AnyTrust on Arbitrum Nova), NOT OP Stack. | Use `offchainlabs/nitro-node`; ports 8547/8548; AnyTrust DA configuration required |
| **moca** | Moca Chain (ID 6342) is Cosmos/EVMOS-based L1, NOT OP Stack. `op-geth` cannot sync. | Identify correct evmos-based image; Cosmos ports (26656 p2p, 8545 EVM RPC); rewrite config |
| **mezo** | Mezo is Cosmos/EVMOS-based chain. `mezoportal/mezo-node` doesn't exist. | Use `mezo/mezod:v2.0.1+`; Cosmos/EVMOS config format; P2P port 26656 |
| **plasma** | Plasma Next (InternetMaximalism) is ZKP-based Plasma protocol, not OP Stack at all. | Remove adapter or rewrite completely for ZKP-based Plasma architecture |
| **celo** | Celo migrated from standalone EVM to OP Stack L2. Existing adapter is pre-migration. | Rewrite as OP Stack adapter with L1_RPC_URL → Ethereum |
| **shibarium** | Wrong org (`shibaswap` is DEX). Wrong architecture: Shibarium uses Polygon Bor, not OP Stack. | Image: `shibaone/bor:v1.3.7-bone`; remove OP Stack L1_RPC_URL injection; use Bor config |
| **bittorrent** | BTTC is Polygon CDK (Bor+Delivery dual-client), not single container. Wrong org (`tronprotocol` vs `bttcprotocol`). | Use `bttcprotocol/bttc`; add Delivery sidecar; missing Tendermint/REST ports |
| **hemi** | `hemilabs/heminetwork` is Bitcoin-finality daemon suite (bfgd/bssd), not the EVM execution layer. | Separate EVM execution (op-geth based) from Bitcoin-finality daemons |
| **aurora** | `nearprotocol/aurora-relayer` is officially deprecated. Multi-component setup required. | Migrate to `nearaurora/srpc2-relayer` (standalone-rpc project); add refiner + nearcore components |
| **sonic** | `fantomfoundation/sonic` is the legacy Fantom Opera client — explicitly incompatible with Sonic mainnet. | Use `0xsoniclabs/sonic:v2.1.6+`; no official pre-built image (build from source) |

#### Non-existent / unverifiable images

| Chain | Issue | Fix |
|-------|-------|-----|
| **megaeth** | `megaeth/megaeth-node:latest` doesn't exist. MegaETH uses custom revm engine; no public image. | Remove adapter until official image published |
| **hyperliquid** | `hyperliquid/hl-visor:latest` not on Docker Hub. Hyperliquid distributes binary only. | Build custom image wrapping binary from `binaries.hyperliquid.xyz/Mainnet/hl-visor` |
| **monad** | `monadlabs/monad-node:latest` does not exist publicly. Monad node access is restricted/permissioned. | Remove adapter until Monad opens public node access and publishes official image |
| **dogecoin** | No official Docker image published by Dogecoin Core project. | Use community image (e.g. `ruimarinho/dogecoin`) or build from source |
| **immutable-zkevm** | `immutable/zkevm-node:v0.7.0` doesn't exist. Tag copied from wrong project. | Use `ghcr.io/immutable/immutable-geth/immutable-geth`; requires `immutable bootstrap rpc` subcommand |
| **hashkey** | `hashkeychain/hashkey-geth:v1.0.0` unverified. Official GitHub org is `hashkey-chain`. | Verify correct image via `docs.hsk.xyz` before deploying |
| **thundercore** | `thundercore/thundercore:v2.21.0` — image name is `thundercore/thunder`, not `thundercore/thundercore`; `v2.21.0` tag doesn't match official `r4.x.x` scheme. | Use `thundercore/thunder:r4.x.x` (verify latest tag on Docker Hub) |

#### Wrong image org / registry

| Chain | Issue | Fix |
|-------|-------|-----|
| **ronin** | Docker org migrated from `axieinfinity` to `ghcr.io/ronin-chain`. | `ghcr.io/ronin-chain/ronin:latest-mainnet` (pin version) |
| **viction** | Rebranded from TomoChain. `tomochain/tomochain` → `buildonviction/node`. | `buildonviction/node:v2.5.1` |
| **haqq** | Wrong Docker Hub org: `haqq-network/haqq` → `alhaqq/haqq`; Cosmos P2P port 30303 → 26656. | `alhaqq/haqq:v1.8.1` |
| **morph** | Image is on `ghcr.io`, not Docker Hub. | Update to correct `ghcr.io/morphprotocol/...` reference |
| **cronos-zkevm** | `cryptocom/cronos-zkevm-node` wrong org + wrong registry. | `ghcr.io/cronos-labs/external-node:mainnet-v29.6.0` |
| **fuse** | `:latest` tag on `fusenet/node`. Also Nethermind-based now but config is geth-style TOML. | `fusenet/node:nethermind-1.27.0-v6.0.1-alpha`; adapt config format |
| **kroma** | Image name wrong in `kroma-network` org. Adapter uses `op-geth` but correct name is `geth`. | Verify correct image tag at `github.com/kroma-network` |
| **lisk** | `lisk/lisk-core` is the pre-L2 Lisk blockchain client. Lisk is now OP Stack L2. | Use OP Stack `op-geth` image with Lisk-specific config |

#### Wrong version (non-existent or breaking change)

| Chain | Issue | Fix |
|-------|-------|-----|
| **ethereum-beacon** | Lighthouse `v6.0.1` severely outdated. `v8.0.0` required for post-Fulu hard fork. | `sigp/lighthouse:v8.0.0` |
| **dash** | Tag `v23.1.0` doesn't exist. Latest stable is `v22.1.2`. | `dashpay/dashd:v22.1.2` |
| **filecoin** | `v1.30.0` outdated → `v1.35.0`. | `filecoin/lotus:v1.35.0` |
| **osmosis** | `v27` outdated → `v31`; additionally missing `--home /data`. | Update version + add `--home /data` |
| **sei** | `v5.9` outdated → `v6.2`; additionally missing `--home /data`. | Update version + add `--home /data` |
| **klaytn** | `v1.0.3` critically outdated; chain rebranded to Kaia as of `v2.0.0+`. | `klaytn/klaytn:v2.2.0` (or `kaiachain/kaia`) |
| **axelar** | `v0.35.6` outdated → `v1.3.4`; additionally missing `--home /data`. | Update version + add `--home /data` |
| **polygon-zkevm** | Uses archived `cdk-validium-node` (PoC). Current production client is `zkevm-node` or `cdk-erigon`. | Update to `hermeznetwork/zkevm-node` or `0xpolygonhermez/zkevm-node` |

#### Systemic: Cosmos SDK chains missing `--home /data`

All Cosmos SDK chains except `cosmos` (Cosmos Hub) are missing the `--home /data` ContainerArg.
Without this flag, the node ignores the PVC mount and writes data to the image-default path, making
snapshot restore completely non-functional.

Affected chains (7): **axelar**, **dymension**, **evmos**, **kava**, **osmosis**, **sei** (+ likely others)

Fix: Add `ContainerArgsProvider` implementing `ContainerArgs()` returning `["start", "--home", "/data"]`
to each affected adapter (match the pattern in `cosmos.go`).

---

#### Other critical issues

| Chain | Issue | Fix |
|-------|-------|-----|
| **berachain** | Berachain requires a separate execution layer (EL) sidecar (beacon + reth). Single container is non-functional. | Add `InitContainerProvider` or multi-container support |
| **moonbeam** | Missing `ContainerArgsProvider` → PVC at `/data` unused; config file never read. | Add `--base-path /data --config /config/config.toml` ContainerArgs |
| **moonriver** | Same as moonbeam. | Same fix |
| **stellar** | `stellar/stellar-core:latest` — `:latest` is production anti-pattern for mainnet nodes. | Pin to specific version e.g. `stellar/stellar-core:v19.12.0` |

---

### Warnings ⚠️ — Should Fix (22 chains)

| Chain | Issue | Recommended Fix |
|-------|-------|-----------------|
| **arbitrum** | Image `v3.2.1` behind current `v3.9.7` | `offchainlabs/nitro-node:v3.9.7` |
| **base** | Missing `--config /config/config.toml` CLI flag | Add to `ContainerArgs` |
| **doma** | No official documentation found; chain type and chain ID 13143 unconfirmed | Verify against chainlist.org/chain/13143 before deploying |
| **blast** | Tag format `v1.2.0` → should be `mainnet-v1.7.0` | Fix tag format |
| **core** | Version `v1.0.14` outdated; `v1.0.22` includes mandatory Hermes hard fork | `coredao/core-chain:v1.0.22` |
| **ethereum-classic** | Besu config format partially wrong; Besu ETC support is deprecated | Consider `etclabscore/core-geth:v1.12.19` |
| **goat** | Image on GHCR not Docker Hub; `L1_RPC_URL` → Ethereum incorrect (GOAT L1 is Bitcoin); `v0.1.0` outdated | `ghcr.io/goatnetwork/goat-geth:v0.2.3+`; remove incorrect L1 env var |
| **katana** | Uses `cdk-opgeth` (Polygon CDK + AggLayer), not vanilla `op-geth` | Identify Katana-specific execution image |
| **linea** | Wrong CLI flag: `--l1-rpc-url` → `--plugin-linea-l1-rpc-endpoint` | Fix flag name in `ContainerArgs` |
| **opbnb** | Should use `bnb-chain/op-geth` fork (BNB-specific consensus patches) | `ghcr.io/bnb-chain/op-geth:v0.5.x` |
| **optimism** | Missing `--config /config/config.toml` CLI flag | Add to `ContainerArgs` |
| **solana** | `solanalabs/validator` archived; migrate to Agave | `solanalabs/agave:v2.x.y` |
| **sui** | `mysten/sui-node:mainnet` is not a semver tag (mutable) | `mysten/sui-node:v1.x.y` (pin to release) |
| **telos** | Telos EVM requires multi-component stack; single container won't function | Redesign as multi-container or document limitation |
| **ton** | `ghcr.io/ton-blockchain/ton:latest` → pin to specific version | `ghcr.io/ton-blockchain/ton:v2026.02-1` |
| **unichain** | Tag `v1.101408.0` stale (Isthmus hard fork); env var targets wrong process | `v1.101608.0`; verify env var consumption |
| **worldchain** | Tag `v1.101408.0` stale; env var targets wrong process | `v1.101603.5`; verify env var consumption |
| **wemix** | Ports wrong (8545/8546 → official: HTTP 8588, WS 8598); image unverified | Fix ports; verify image at `wemixnetwork/wemix` |
| **zircuit** | Wrong registry (Docker Hub vs GHCR); `:latest` non-semver | `ghcr.io/zircuit-labs/l2-geth-public:vX.Y.Z` |

---

## Systemic Patterns

### 1. OP Stack env var targeting wrong process
Chains using `OP_NODE_L1_ETH_RPC` env var: this variable is consumed by `op-node` (the consensus
client), not by `op-geth` (the execution client). Adapters that configure only the execution layer
should use `GETH_ROLLUP_SEQUENCERHTTP` or pass the L1 endpoint via CLI flags instead.
Affects: **worldchain**, **unichain**, and multiple other OP Stack L2s.

### 2. Missing `--config` flag for OP Stack geth
`optimism` and `base` write a `config.toml` via `ConfigTemplate()` but never pass `--config /config/config.toml`
to the container. The config file is mounted but never loaded.
Likely affects other OP Stack chains with non-empty ConfigTemplate().

### 3. Cosmos SDK `--home /data` missing
7 of 8 Cosmos SDK chains are missing `--home /data`, making snapshot restore non-functional.
Only `cosmos` (Cosmos Hub) is correctly configured.

### 4. ZK Stack chains incorrectly built as OP Stack
`lens`, `zero-network`, `abstract` are ZK Stack (Matter Labs zkSync Era) chains, not OP Stack.
These were likely created by copy-pasting an OP Stack template. ZK Stack external nodes use
different images (`matterlabs/external-node`), different ports (3060/3061), and different config formats.

### 6. Arbitrum Orbit chains incorrectly built as OP Stack
`everclear`, `playnance`, `gravity-alpha`, `plume` are Arbitrum Orbit (Nitro-based) chains, not OP Stack.
These use `offchainlabs/nitro-node` (or chain-specific forks), ports 8547/8548, and a completely different
config model (`--parent-chain.connection.url`, AnyTrust DA parameters). These adapters need full rewrites.

### 7. Cosmos/EVMOS chains incorrectly built as OP Stack
`moca`, `mezo` are Cosmos SDK / EVMOS-based chains. They use CometBFT P2P (port 26656), Cosmos REST (1317),
and gRPC (9090). Using `op-geth` is non-functional. These need rewriting as Cosmos adapters similar to `evmos`.

### 5. Stale version pinning
Several chains were pinned to versions that have since had breaking changes or mandatory hard forks:
`ethereum-beacon` (Fulu), `klaytn` (Kaia rebrand), `core` (Hermes HF), `sei`, `osmosis`, `axelar`.
Consider tracking upstream release feeds.
