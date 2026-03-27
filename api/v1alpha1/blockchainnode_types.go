/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---------------------------------------------------------------------------
// Chain — supported blockchain protocols
// ---------------------------------------------------------------------------

// Chain identifies the blockchain protocol that the node participates in.
// The value is used to select the correct adapter, default image, ports, and
// storage requirements.
// +kubebuilder:validation:Enum=ethereum;ethereum-archive;ethereum-beacon;bitcoin;solana;bsc;tron;polygon;avalanche;litecoin;xrp;stellar;dash;ton;cosmos;near;sui;aptos;cardano;arbitrum;optimism;base;fantom;gnosis;gnosis-beacon;mantle;zksync;linea;scroll;berachain;cronos;ronin;celo;blast;mode;zora;taiko;dogecoin;osmosis;sei;evmos;kava;polkadot;starknet;filecoin;moonbeam;moonriver;polygon-zkevm;manta-pacific;metis;fraxtal;lisk;kroma;bob;boba-eth;soneium;swell;superseed;ink;morph;worldchain;unichain;lens;plume;hemi;abstract;megaeth;zero-network;zircuit;immutable-zkevm;axelar;dymension;aurora;harmony;rootstock;telos;klaytn;shibarium;core;haqq;hashkey;ethereum-classic;opbnb;fuse;thundercore;wemix;viction;cronos-zkevm;sonic;goat;katana;mezo;plasma;playnance;kusama;hyperliquid;monad
type Chain string

const (
	// ChainEthereum runs a standard pruned Ethereum execution-layer client.
	ChainEthereum Chain = "ethereum"
	// ChainEthereumArchive runs a full-archive Ethereum execution-layer client.
	ChainEthereumArchive Chain = "ethereum-archive"
	// ChainEthereumBeacon runs an Ethereum Beacon Chain (consensus layer) node.
	ChainEthereumBeacon Chain = "ethereum-beacon"
	// ChainBitcoin runs a Bitcoin Core full node.
	ChainBitcoin Chain = "bitcoin"
	// ChainSolana runs a Solana validator or RPC node.
	ChainSolana Chain = "solana"
	// ChainBSC runs a BNB Smart Chain (BSC) full node.
	ChainBSC Chain = "bsc"
	// ChainTRON runs a TRON full or lite node.
	ChainTRON Chain = "tron"
	// ChainPolygon runs a Polygon PoS full node.
	ChainPolygon Chain = "polygon"
	// ChainAvalanche runs an Avalanche full node.
	ChainAvalanche Chain = "avalanche"
	// ChainLitecoin runs a Litecoin Core full node.
	ChainLitecoin Chain = "litecoin"
	// ChainXRP runs an XRP Ledger (rippled) node.
	ChainXRP Chain = "xrp"
	// ChainStellar runs a Stellar Core validator or watcher node.
	ChainStellar Chain = "stellar"
	// ChainDash runs a Dash Core full node.
	ChainDash Chain = "dash"
	// ChainTON runs a TON (The Open Network) full node.
	ChainTON Chain = "ton"
	// ChainCosmos runs a Cosmos SDK-based chain node.
	ChainCosmos Chain = "cosmos"
	// ChainNear runs a NEAR Protocol node.
	ChainNear Chain = "near"
	// ChainSui runs a Sui full node.
	ChainSui Chain = "sui"
	// ChainAptos runs an Aptos full node.
	ChainAptos Chain = "aptos"
	// ChainCardano runs a Cardano node.
	ChainCardano Chain = "cardano"
	// ChainArbitrum runs an Arbitrum One (Nitro) full node.
	ChainArbitrum Chain = "arbitrum"
	// ChainOptimism runs an Optimism (OP Mainnet) full node.
	ChainOptimism Chain = "optimism"
	// ChainBase runs a Base (OP Stack) full node.
	ChainBase Chain = "base"
	// ChainFantom runs a Fantom (Sonic) Opera full node.
	ChainFantom Chain = "fantom"
	// ChainGnosis runs a Gnosis Chain full node.
	ChainGnosis Chain = "gnosis"
	// ChainGnosisBeacon runs a Gnosis Beacon Chain (consensus layer) node.
	ChainGnosisBeacon Chain = "gnosis-beacon"
	// ChainMantle runs a Mantle (OP Stack) full node.
	ChainMantle Chain = "mantle"
	// ChainZkSync runs a zkSync Era external node.
	ChainZkSync Chain = "zksync"
	// ChainLinea runs a Linea full node.
	ChainLinea Chain = "linea"
	// ChainScroll runs a Scroll L2 full node.
	ChainScroll Chain = "scroll"
	// ChainBerachain runs a Berachain Beacon Kit full node.
	ChainBerachain Chain = "berachain"
	// ChainCronos runs a Cronos EVM full node.
	ChainCronos Chain = "cronos"
	// ChainRonin runs a Ronin (Axie Infinity) full node.
	ChainRonin Chain = "ronin"
	// ChainCelo runs a Celo L1 full node.
	ChainCelo Chain = "celo"
	// ChainBlast runs a Blast (OP Stack) L2 full node.
	ChainBlast Chain = "blast"
	// ChainMode runs a Mode (OP Stack) L2 full node.
	ChainMode Chain = "mode"
	// ChainZora runs a Zora (OP Stack) L2 full node.
	ChainZora Chain = "zora"
	// ChainTaiko runs a Taiko L2 full node.
	ChainTaiko Chain = "taiko"
	// ChainDogecoin runs a Dogecoin Core full node.
	ChainDogecoin Chain = "dogecoin"
	// ChainOsmosis runs an Osmosis DEX full node.
	ChainOsmosis Chain = "osmosis"
	// ChainSei runs a Sei high-performance L1 full node.
	ChainSei Chain = "sei"
	// ChainEvmos runs an Evmos EVM-compatible Cosmos chain node.
	ChainEvmos Chain = "evmos"
	// ChainKava runs a Kava DeFi hub full node.
	ChainKava Chain = "kava"
	// ChainPolkadot runs a Polkadot relay chain node.
	ChainPolkadot Chain = "polkadot"
	// ChainStarknet runs a Starknet full node (Juno client).
	ChainStarknet Chain = "starknet"
	// ChainFilecoin runs a Filecoin full node (Lotus client).
	ChainFilecoin Chain = "filecoin"
	// ChainMoonbeam runs a Moonbeam (Polkadot parachain, EVM-compatible) full node.
	ChainMoonbeam Chain = "moonbeam"
	// ChainMoonriver runs a Moonriver (Kusama parachain, EVM-compatible) full node.
	ChainMoonriver Chain = "moonriver"
	// ChainPolygonZkEVM runs a Polygon zkEVM (CDK Validium) full node.
	ChainPolygonZkEVM Chain = "polygon-zkevm"
	// ChainMantaPacific runs a Manta Pacific (OP Stack) L2 full node.
	ChainMantaPacific Chain = "manta-pacific"
	// ChainMetis runs a Metis Andromeda L2 full node.
	ChainMetis Chain = "metis"
	// ChainFraxtal runs a Fraxtal (OP Stack) L2 full node.
	ChainFraxtal Chain = "fraxtal"
	// ChainLisk runs a Lisk (OP Stack) L2 full node.
	ChainLisk Chain = "lisk"
	// ChainKroma runs a Kroma (OP Stack) L2 full node.
	ChainKroma Chain = "kroma"
	// ChainBob runs a BOB (Build on Bitcoin, OP Stack) L2 full node.
	ChainBob Chain = "bob"
	// ChainBobaEth runs a Boba Network ETH (OP Stack) L2 full node.
	ChainBobaEth Chain = "boba-eth"
	// ChainSoneium runs a Soneium (OP Stack) L2 full node.
	ChainSoneium Chain = "soneium"
	// ChainSwell runs a Swell (OP Stack) L2 full node.
	ChainSwell Chain = "swell"
	// ChainSuperseed runs a Superseed (OP Stack) L2 full node.
	ChainSuperseed Chain = "superseed"
	// ChainInk runs an Ink (OP Stack) L2 full node.
	ChainInk Chain = "ink"
	// ChainMorph runs a Morph L2 full node.
	ChainMorph Chain = "morph"
	// ChainWorldchain runs a Worldchain (OP Stack) L2 full node.
	ChainWorldchain Chain = "worldchain"
	// ChainUnichain runs a Unichain (OP Stack) L2 full node.
	ChainUnichain Chain = "unichain"
	// ChainLens runs a Lens (OP Stack) L2 full node.
	ChainLens Chain = "lens"
	// ChainPlume runs a Plume (OP Stack) L2 full node.
	ChainPlume Chain = "plume"
	// ChainHemi runs a Hemi L2 full node.
	ChainHemi Chain = "hemi"
	// ChainAbstract runs an Abstract (ZK-stack based on Ethereum) L2 full node.
	ChainAbstract Chain = "abstract"
	// ChainMegaETH runs a MegaETH L2 full node.
	ChainMegaETH Chain = "megaeth"
	// ChainZeroNetwork runs a Zero Network (OP-stack) L2 full node.
	ChainZeroNetwork Chain = "zero-network"
	// ChainZircuit runs a Zircuit L2 full node.
	ChainZircuit Chain = "zircuit"
	// ChainImmutableZkEVM runs an Immutable zkEVM L2 full node.
	ChainImmutableZkEVM Chain = "immutable-zkevm"
	// ChainAxelar runs an Axelar cross-chain gateway protocol node.
	ChainAxelar Chain = "axelar"
	// ChainDymension runs a Dymension modular rollup hub node.
	ChainDymension Chain = "dymension"
	// ChainShibarium runs a Shibarium (Shiba Inu L2, OP-stack) full node.
	ChainShibarium Chain = "shibarium"
	// ChainCore runs a Core Chain (CoreDAO) EVM full node.
	ChainCore Chain = "core"
	// ChainHaqq runs a Haqq Network (Islamic Coin, Cosmos EVM) full node.
	ChainHaqq Chain = "haqq"
	// ChainHashKey runs a HashKey Chain EVM full node.
	ChainHashKey Chain = "hashkey"
	// ChainEthereumClassic runs an Ethereum Classic full node (Besu client).
	ChainEthereumClassic Chain = "ethereum-classic"
	// ChainBitTorrent runs a BitTorrent Chain (BTTC) EVM-compatible sidechain node.
	ChainBitTorrent Chain = "bittorrent"
	// ChainGravityAlpha runs a Gravity Alpha EVM-compatible L1 full node.
	ChainGravityAlpha Chain = "gravity-alpha"
	// ChainMoca runs a Moca Network (OP Stack) L2 full node.
	ChainMoca Chain = "moca"
	// ChainEverclear runs an Everclear (OP Stack) L2 full node.
	ChainEverclear Chain = "everclear"
	// ChainDoma runs a Doma (OP Stack) L2 full node.
	ChainDoma Chain = "doma"
	// ChainAurora runs an Aurora EVM (on NEAR Protocol) relayer node.
	ChainAurora Chain = "aurora"
	// ChainHarmony runs a Harmony ONE full node.
	ChainHarmony Chain = "harmony"
	// ChainRootstock runs a Rootstock (RSK) Bitcoin sidechain EVM node.
	ChainRootstock Chain = "rootstock"
	// ChainTelos runs a Telos EVM RPC full node.
	ChainTelos Chain = "telos"
	// ChainKlaytn runs a Kaia (Klaytn) endpoint node.
	ChainKlaytn Chain = "klaytn"
	// ChainOpBNB runs an opBNB (BNB Chain L2, OP-stack) full node.
	ChainOpBNB Chain = "opbnb"
	// ChainFuse runs a Fuse Network full node.
	ChainFuse Chain = "fuse"
	// ChainThundercore runs a ThunderCore full node.
	ChainThundercore Chain = "thundercore"
	// ChainWemix runs a WEMIX full node.
	ChainWemix Chain = "wemix"
	// ChainViction runs a Viction (TomoChain) full node.
	ChainViction Chain = "viction"
	// ChainKusama runs a Kusama (Polkadot canary network, Substrate) full node.
	ChainKusama Chain = "kusama"
	// ChainHyperliquid runs a Hyperliquid full node.
	ChainHyperliquid Chain = "hyperliquid"
	// ChainMonad runs a Monad (parallel EVM) full node.
	ChainMonad Chain = "monad"
	// ChainCronosZkEVM runs a Cronos zkEVM L2 full node.
	ChainCronosZkEVM Chain = "cronos-zkevm"
	// ChainSonic runs a Sonic (Opera EVM) full node.
	ChainSonic Chain = "sonic"
	// ChainGoat runs a Goat Network (Bitcoin L2 EVM) full node.
	ChainGoat Chain = "goat"
	// ChainKatana runs a Katana (OP-stack) L2 full node.
	ChainKatana Chain = "katana"
	// ChainMezo runs a Mezo L2 full node.
	ChainMezo Chain = "mezo"
	// ChainPlasma runs a Plasma (OP-stack) L2 full node.
	ChainPlasma Chain = "plasma"
	// ChainPlaynance runs a Playnance (OP-stack) L2 full node.
	ChainPlaynance Chain = "playnance"
)

// String returns the lowercase string representation of the Chain.
func (c Chain) String() string { return string(c) }

// ---------------------------------------------------------------------------
// Network — blockchain network environments
// ---------------------------------------------------------------------------

// Network identifies the target network environment for the blockchain node.
// +kubebuilder:validation:Enum=mainnet;testnet;devnet
type Network string

const (
	// NetworkMainnet is the production blockchain network.
	NetworkMainnet Network = "mainnet"
	// NetworkTestnet is the public test network.
	NetworkTestnet Network = "testnet"
	// NetworkDevnet is a development/local network.
	NetworkDevnet Network = "devnet"
)

// String returns the string representation of the Network.
func (n Network) String() string { return string(n) }

// ---------------------------------------------------------------------------
// NodeType — blockchain node roles
// ---------------------------------------------------------------------------

// NodeType describes the operational role of the blockchain node.
// +kubebuilder:validation:Enum=rpc;archive;validator;light
type NodeType string

const (
	// NodeTypeRPC serves JSON-RPC queries for recent state.
	NodeTypeRPC NodeType = "rpc"
	// NodeTypeArchive retains the full historical state of the chain.
	NodeTypeArchive NodeType = "archive"
	// NodeTypeValidator participates in consensus and block production.
	NodeTypeValidator NodeType = "validator"
	// NodeTypeLight only downloads block headers and validates on demand.
	NodeTypeLight NodeType = "light"
)

// String returns the string representation of the NodeType.
func (t NodeType) String() string { return string(t) }

// ---------------------------------------------------------------------------
// NodeGroup — hardware scheduling tiers
// ---------------------------------------------------------------------------

// NodeGroup assigns the node to a hardware tier, which maps to Kubernetes
// node affinity rules for scheduling.
// +kubebuilder:validation:Enum=light;medium;heavy;archive;storage;blockchain
type NodeGroup string

const (
	// NodeGroupLight targets low-resource nodes (light clients, testnets).
	NodeGroupLight NodeGroup = "light"
	// NodeGroupMedium targets general-purpose nodes.
	NodeGroupMedium NodeGroup = "medium"
	// NodeGroupHeavy targets high-CPU/memory nodes (Solana, BSC).
	NodeGroupHeavy NodeGroup = "heavy"
	// NodeGroupArchive targets high-storage nodes with fast NVMe.
	NodeGroupArchive NodeGroup = "archive"
	// NodeGroupStorage targets bulk-storage nodes (HDD-backed).
	NodeGroupStorage NodeGroup = "storage"
	// NodeGroupBlockchain is a generic blockchain-workload tier.
	NodeGroupBlockchain NodeGroup = "blockchain"
)

// String returns the string representation of the NodeGroup.
func (g NodeGroup) String() string { return string(g) }

// ---------------------------------------------------------------------------
// NodePhase — lifecycle phases
// ---------------------------------------------------------------------------

// NodePhase represents the high-level lifecycle phase of a blockchain node.
// The phase is derived from the node's sync state and health checks.
type NodePhase string

const (
	// NodePhasePending means the StatefulSet/Pod has not started yet.
	NodePhasePending NodePhase = "Pending"
	// NodePhaseSyncing means the node is running but still catching up with the chain tip.
	NodePhaseSyncing NodePhase = "Syncing"
	// NodePhaseHealthy means the node is fully synced and serving traffic.
	NodePhaseHealthy NodePhase = "Healthy"
	// NodePhaseDegraded means the node is running but failing health checks.
	NodePhaseDegraded NodePhase = "Degraded"
	// NodePhaseFailed means the node has encountered an unrecoverable error.
	NodePhaseFailed NodePhase = "Failed"
)

// String returns the string representation of the NodePhase.
func (p NodePhase) String() string { return string(p) }

// ---------------------------------------------------------------------------
// Condition types and well-known constants
// ---------------------------------------------------------------------------

const (
	// ConditionReady indicates the node is fully synced and able to serve requests.
	ConditionReady = "Ready"
	// ConditionSyncing indicates the node is catching up with the chain tip.
	ConditionSyncing = "Syncing"
	// ConditionDegraded indicates the node is failing health checks.
	ConditionDegraded = "Degraded"

	// FinalizerName is the finalizer added to BlockchainNode resources to
	// ensure proper cleanup of owned sub-resources before deletion.
	FinalizerName = "nodes.k8s-bch.io/finalizer"
)

// ---------------------------------------------------------------------------
// Spec sub-types
// ---------------------------------------------------------------------------

// ImageSpec defines the container image configuration for the node.
type ImageSpec struct {
	// Repository is the container image repository (e.g. "nethermind/nethermind").
	// +kubebuilder:validation:MinLength=1
	Repository string `json:"repository"`

	// Tag is the image tag or digest (e.g. "1.26.0").
	// +kubebuilder:validation:MinLength=1
	Tag string `json:"tag"`

	// PullPolicy overrides the default image pull policy.
	// +kubebuilder:default=IfNotPresent
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

// StorageSpec defines the persistent volume configuration for chain data.
type StorageSpec struct {
	// Size is the requested PVC capacity (e.g. "4Ti", "500Gi").
	// Must be greater than zero.
	Size resource.Quantity `json:"size"`

	// StorageClass overrides the default Kubernetes StorageClass.
	// When empty the cluster default is used.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// SnapshotType selects the snapshot variant for node bootstrapping.
// +kubebuilder:validation:Enum=full;lite
type SnapshotType string

const (
	// SnapshotTypeFull uses the full node snapshot (complete history).
	SnapshotTypeFull SnapshotType = "full"
	// SnapshotTypeLite uses a pruned/lite snapshot (current state only).
	// For TRON this means LiteFullNode (~60 GiB) instead of FullNode (~2.9 TiB).
	SnapshotTypeLite SnapshotType = "lite"
)

// String returns the string representation of the SnapshotType.
func (s SnapshotType) String() string { return string(s) }

// SnapshotSpec configures snapshot-based bootstrapping for the node.
// When nil in the parent spec, the operator uses default MinIO-based bootstrap
// if the MINIO_ENDPOINT environment variable is set.
type SnapshotSpec struct {
	// Disabled skips snapshot bootstrapping entirely; the node syncs from genesis.
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	// Type selects the snapshot variant: "full" (default) or "lite".
	// Lite snapshots are smaller but may lack full transaction history.
	// +kubebuilder:default=full
	// +optional
	Type SnapshotType `json:"type,omitempty"`

	// Bucket overrides the default MinIO bucket name ("snapshots-{chain}").
	// +optional
	Bucket string `json:"bucket,omitempty"`

	// Key overrides the specific object key inside the MinIO bucket.
	// +optional
	Key string `json:"key,omitempty"`
}

// HealthSpec configures health-monitoring thresholds for the blockchain node.
type HealthSpec struct {
	// BlockLagThreshold is the maximum allowed block lag behind the chain tip
	// before the node is marked Degraded.
	// When unset, a chain-specific default applies (ETH=30, BTC=2, TRON=200).
	// +optional
	BlockLagThreshold *int64 `json:"blockLagThreshold,omitempty"`

	// DegradedTimeoutMinutes is how many minutes a node may remain Degraded
	// before the controller deletes its pod to trigger a restart.
	// Set to 0 to disable auto-restart. Defaults to 15.
	// +kubebuilder:default=15
	// +optional
	DegradedTimeoutMinutes *int32 `json:"degradedTimeoutMinutes,omitempty"`
}

// RPCSpec defines the RPC endpoint exposure for the blockchain node.
type RPCSpec struct {
	// Enabled controls whether RPC endpoints are exposed via a Service.
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Port is the HTTP JSON-RPC listen port.
	// +kubebuilder:default=8545
	// +optional
	Port int32 `json:"port,omitempty"`

	// WSPort is the WebSocket RPC listen port. When zero, WebSocket is disabled.
	// +optional
	WSPort int32 `json:"wsPort,omitempty"`
}

// ---------------------------------------------------------------------------
// BlockchainNodeSpec — desired state
// ---------------------------------------------------------------------------

// BlockchainNodeSpec defines the desired state of a BlockchainNode resource.
// It captures the full configuration needed to provision and operate a
// blockchain node: chain type, client software, resource requirements,
// storage, health monitoring, and sidecar containers.
type BlockchainNodeSpec struct {
	// Chain selects the blockchain protocol (e.g. "ethereum", "bitcoin", "solana").
	// This field is immutable after creation.
	// +kubebuilder:validation:Required
	Chain Chain `json:"chain"`

	// Network selects the target network (mainnet, testnet, or devnet).
	// This field is immutable after creation.
	// +kubebuilder:default=mainnet
	Network Network `json:"network"`

	// NodeType determines the operational role of the node.
	// +kubebuilder:default=rpc
	NodeType NodeType `json:"nodeType"`

	// Client overrides the default client software for the chain
	// (e.g. "nethermind", "geth", "reth", "erigon" for Ethereum).
	// When empty the adapter's default client is used.
	// +optional
	Client string `json:"client,omitempty"`

	// Image overrides the default container image for this chain/client.
	// When nil the adapter's default image is used.
	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// Resources defines the CPU and memory requests/limits for the node container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Storage defines persistent volume configuration for chain data.
	// +kubebuilder:validation:Required
	Storage StorageSpec `json:"storage"`

	// NodeGroup selects the hardware scheduling tier.
	// +kubebuilder:default=medium
	NodeGroup NodeGroup `json:"nodeGroup"`

	// Replicas is the number of node instances to run.
	// Set to 0 to pause the node (scale-down without data loss).
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// RPC configures the JSON-RPC endpoint exposure.
	// +optional
	RPC RPCSpec `json:"rpc,omitempty"`

	// ExtraArgs are additional command-line arguments passed to the node process.
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`

	// ExtraEnv are additional environment variables injected into the node container.
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`

	// Health configures health-monitoring thresholds.
	// +optional
	Health HealthSpec `json:"health,omitempty"`

	// Snapshot configures snapshot-based bootstrapping.
	// When nil, the operator uses its default MinIO-based bootstrap if MINIO_ENDPOINT is set.
	// +optional
	Snapshot *SnapshotSpec `json:"snapshot,omitempty"`

	// Sidecars are additional containers running alongside the node container.
	// Common use case: consensus-layer clients (e.g. Lighthouse for Ethereum EL+CL).
	// Sidecars share the "data" volume with the main container.
	// +optional
	Sidecars []corev1.Container `json:"sidecars,omitempty"`

	// ExtraVolumes are additional pod-level volumes (e.g. JWT secrets for Engine API).
	// They are appended to the default "data" and "config" volumes.
	// +optional
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts are additional volume mounts for the main node container.
	// Use together with ExtraVolumes to expose secrets or configmaps.
	// +optional
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`
}

// ---------------------------------------------------------------------------
// BlockchainNodeStatus — observed state
// ---------------------------------------------------------------------------

// BlockchainNodeStatus defines the observed state of BlockchainNode as
// reported by the controller's reconciliation loop and health checks.
type BlockchainNodeStatus struct {
	// Phase is the high-level lifecycle phase of the node.
	// +optional
	Phase NodePhase `json:"phase,omitempty"`

	// BlockHeight is the latest confirmed block number known to the node.
	// +optional
	BlockHeight int64 `json:"blockHeight,omitempty"`

	// SyncProgress is the human-readable sync percentage (e.g. "98.5%").
	// Empty when fully synced.
	// +optional
	SyncProgress string `json:"syncProgress,omitempty"`

	// SyncETA is the estimated time remaining until the node is fully synced
	// (e.g. "2h15m"). Empty when synced, stalled, or the rate cannot be
	// calculated yet.
	// +optional
	SyncETA string `json:"syncETA,omitempty"`

	// PeersCount is the number of connected p2p peers.
	// +optional
	PeersCount int32 `json:"peersCount,omitempty"`

	// Conditions represent the latest available observations of the node state.
	// Supported condition types: Ready, Syncing, Degraded.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the .metadata.generation that was last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ---------------------------------------------------------------------------
// Status helper methods
// ---------------------------------------------------------------------------

// IsHealthy returns true when the node is in the Healthy phase.
func (s *BlockchainNodeStatus) IsHealthy() bool {
	return s.Phase == NodePhaseHealthy
}

// IsSyncing returns true when the node is still catching up with the chain tip.
func (s *BlockchainNodeStatus) IsSyncing() bool {
	return s.Phase == NodePhaseSyncing
}

// IsDegraded returns true when the node is running but failing health checks.
func (s *BlockchainNodeStatus) IsDegraded() bool {
	return s.Phase == NodePhaseDegraded
}

// IsFailed returns true when the node has encountered an unrecoverable error.
func (s *BlockchainNodeStatus) IsFailed() bool {
	return s.Phase == NodePhaseFailed
}

// IsReady returns true when the Ready condition is set to True.
func (s *BlockchainNodeStatus) IsReady() bool {
	for i := range s.Conditions {
		if s.Conditions[i].Type == ConditionReady {
			return s.Conditions[i].Status == metav1.ConditionTrue
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Root types
// ---------------------------------------------------------------------------

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Chain",type="string",JSONPath=".spec.chain"
// +kubebuilder:printcolumn:name="Network",type="string",JSONPath=".spec.network"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.nodeType"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Height",type="integer",JSONPath=".status.blockHeight"
// +kubebuilder:printcolumn:name="Peers",type="integer",JSONPath=".status.peersCount"
// +kubebuilder:printcolumn:name="Sync",type="string",JSONPath=".status.syncProgress"
// +kubebuilder:printcolumn:name="ETA",type="string",JSONPath=".status.syncETA"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// BlockchainNode manages the full lifecycle of a blockchain node: StatefulSet,
// PVC, Services, ConfigMap, and health monitoring. It is the primary custom
// resource of the blockchain-node-operator.
type BlockchainNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BlockchainNodeSpec   `json:"spec,omitempty"`
	Status BlockchainNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BlockchainNodeList contains a list of BlockchainNode resources.
type BlockchainNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlockchainNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BlockchainNode{}, &BlockchainNodeList{})
}
