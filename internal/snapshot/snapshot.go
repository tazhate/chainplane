/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0

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
/*
Package snapshot provides helpers for building snapshot init containers that
pre-populate blockchain node data volumes before the main node starts.

The init container uses python:3.11-slim with minio + requests + lz4 packages.
No apt-get required — lz4 decompression done via Python lz4 library.

Logic:
 1. If /data is non-empty → exit 0 (idempotent).
 2. Try MinIO: find latest object in the bucket and stream it.
 3. If MinIO bucket empty or unreachable → fallback to publicnode/official source.
 4. .tar.lz4: lz4.frame decompress → tarfile extract (pure Python, no system deps).
 5. .tgz/.tar.gz: tarfile extract with gz mode.
*/
package snapshot

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

const defaultSnapshotRestoreImage = "ghcr.io/tazhate/chainplane/snapshot-restore:latest"

// Config holds the MinIO connection and bucket/key settings for snapshot injection.
type Config struct {
	// Endpoint is the MinIO API endpoint (e.g. "http://minio:9000")
	Endpoint string
	// Bucket overrides the default bucket for the chain (optional).
	Bucket string
	// Key overrides the specific object key to download (optional).
	Key string
	// SnapshotType is "full" or "lite". Lite uses smaller snapshots (e.g. TRON LiteFullNode).
	SnapshotType string
}

// bucketForChain returns the default MinIO bucket name for a given chain.
func bucketForChain(chain chainsv1alpha2.Chain) string {
	switch chain {
	case chainsv1alpha2.ChainBitcoin:
		return "snapshots-btc"
	case chainsv1alpha2.ChainTRON:
		return "snapshots-tron"
	case chainsv1alpha2.ChainLitecoin:
		return "snapshots-ltc"
	case chainsv1alpha2.ChainDash:
		return "snapshots-dash"
	case chainsv1alpha2.ChainXRP:
		return "snapshots-xrp"
	case chainsv1alpha2.ChainStellar:
		return "snapshots-xlm"
	case chainsv1alpha2.ChainAvalanche:
		return "snapshots-avax"
	case chainsv1alpha2.ChainPolygon:
		return "snapshots-polygon"
	case chainsv1alpha2.ChainCosmos:
		return "snapshots-cosmos"
	case chainsv1alpha2.ChainNear:
		return "snapshots-near"
	case chainsv1alpha2.ChainSui:
		return "snapshots-sui"
	case chainsv1alpha2.ChainAptos:
		return "snapshots-aptos"
	case chainsv1alpha2.ChainCardano:
		return "snapshots-cardano"
	case chainsv1alpha2.ChainTON:
		return "snapshots-ton"
	case chainsv1alpha2.ChainBSC:
		return "snapshots-bsc"
	case chainsv1alpha2.ChainArbitrum:
		return "snapshots-arbitrum"
	case chainsv1alpha2.ChainOptimism:
		return "snapshots-optimism"
	case chainsv1alpha2.ChainBase:
		return "snapshots-base"
	case chainsv1alpha2.ChainFantom:
		return "snapshots-fantom"
	case chainsv1alpha2.ChainGnosis:
		return "snapshots-gnosis"
	case chainsv1alpha2.ChainMantle:
		return "snapshots-mantle"
	case chainsv1alpha2.ChainBerachain:
		return "snapshots-berachain"
	case chainsv1alpha2.ChainCronos:
		return "snapshots-cronos"
	case chainsv1alpha2.ChainRonin:
		return "snapshots-ronin"
	case chainsv1alpha2.ChainCelo:
		return "snapshots-celo"
	case chainsv1alpha2.ChainBlast:
		return "snapshots-blast"
	case chainsv1alpha2.ChainMode:
		return "snapshots-mode"
	case chainsv1alpha2.ChainZora:
		return "snapshots-zora"
	case chainsv1alpha2.ChainTaiko:
		return "snapshots-taiko"
	case chainsv1alpha2.ChainZkSync:
		return "snapshots-zksync"
	case chainsv1alpha2.ChainLinea:
		return "snapshots-linea"
	case chainsv1alpha2.ChainScroll:
		return "snapshots-scroll"
	case chainsv1alpha2.ChainDogecoin:
		return "snapshots-doge"
	case chainsv1alpha2.ChainOsmosis:
		return "snapshots-osmosis"
	case chainsv1alpha2.ChainSei:
		return "snapshots-sei"
	case chainsv1alpha2.ChainEvmos:
		return "snapshots-evmos"
	case chainsv1alpha2.ChainKava:
		return "snapshots-kava"
	case chainsv1alpha2.ChainPolkadot:
		return "snapshots-polkadot"
	case chainsv1alpha2.ChainStarknet:
		return "snapshots-starknet"
	case chainsv1alpha2.ChainFilecoin:
		return "snapshots-filecoin"
	case chainsv1alpha2.ChainMoonbeam:
		return "snapshots-moonbeam"
	case chainsv1alpha2.ChainMoonriver:
		return "snapshots-moonriver"
	case chainsv1alpha2.ChainPolygonZkEVM:
		return "snapshots-polygon-zkevm"
	case chainsv1alpha2.ChainMantaPacific:
		return "snapshots-manta-pacific"
	case chainsv1alpha2.ChainMetis:
		return "snapshots-metis"
	case chainsv1alpha2.ChainFraxtal:
		return "snapshots-fraxtal"
	case chainsv1alpha2.ChainLisk:
		return "snapshots-lisk"
	case chainsv1alpha2.ChainKroma:
		return "snapshots-kroma"
	case chainsv1alpha2.ChainBob:
		return "snapshots-bob"
	case chainsv1alpha2.ChainBobaEth:
		return "snapshots-boba-eth"
	case chainsv1alpha2.ChainSoneium:
		return "snapshots-soneium"
	case chainsv1alpha2.ChainSwell:
		return "snapshots-swell"
	case chainsv1alpha2.ChainSuperseed:
		return "snapshots-superseed"
	case chainsv1alpha2.ChainInk:
		return "snapshots-ink"
	case chainsv1alpha2.ChainMorph:
		return "snapshots-morph"
	case chainsv1alpha2.ChainAbstract:
		return "snapshots-abstract"
	case chainsv1alpha2.ChainMegaETH:
		return "snapshots-megaeth"
	case chainsv1alpha2.ChainZeroNetwork:
		return "snapshots-zero-network"
	case chainsv1alpha2.ChainZircuit:
		return "snapshots-zircuit"
	case chainsv1alpha2.ChainImmutableZkEVM:
		return "snapshots-immutable-zkevm"
	case chainsv1alpha2.ChainWorldchain:
		return "snapshots-worldchain"
	case chainsv1alpha2.ChainUnichain:
		return "snapshots-unichain"
	case chainsv1alpha2.ChainLens:
		return "snapshots-lens"
	case chainsv1alpha2.ChainPlume:
		return "snapshots-plume"
	case chainsv1alpha2.ChainHemi:
		return "snapshots-hemi"
	case chainsv1alpha2.ChainAurora:
		return "snapshots-aurora"
	case chainsv1alpha2.ChainHarmony:
		return "snapshots-harmony"
	case chainsv1alpha2.ChainRootstock:
		return "snapshots-rootstock"
	case chainsv1alpha2.ChainTelos:
		return "snapshots-telos"
	case chainsv1alpha2.ChainKlaytn:
		return "snapshots-klaytn"
	case chainsv1alpha2.ChainShibarium:
		return "snapshots-shibarium"
	case chainsv1alpha2.ChainCore:
		return "snapshots-core"
	case chainsv1alpha2.ChainHaqq:
		return "snapshots-haqq"
	case chainsv1alpha2.ChainHashKey:
		return "snapshots-hashkey"
	case chainsv1alpha2.ChainEthereumClassic:
		return "snapshots-ethereum-classic"
	case chainsv1alpha2.ChainAxelar:
		return "snapshots-axelar"
	case chainsv1alpha2.ChainDymension:
		return "snapshots-dymension"
	case chainsv1alpha2.ChainBitTorrent:
		return "snapshots-bittorrent"
	case chainsv1alpha2.ChainGravityAlpha:
		return "snapshots-gravity-alpha"
	case chainsv1alpha2.ChainMoca:
		return "snapshots-moca"
	case chainsv1alpha2.ChainEverclear:
		return "snapshots-everclear"
	case chainsv1alpha2.ChainDoma:
		return "snapshots-doma"
	case chainsv1alpha2.ChainOpBNB:
		return "snapshots-opbnb"
	case chainsv1alpha2.ChainFuse:
		return "snapshots-fuse"
	case chainsv1alpha2.ChainThundercore:
		return "snapshots-thundercore"
	case chainsv1alpha2.ChainWemix:
		return "snapshots-wemix"
	case chainsv1alpha2.ChainViction:
		return "snapshots-viction"
	case chainsv1alpha2.ChainEthereumBeacon:
		return "snapshots-ethereum-beacon"
	case chainsv1alpha2.ChainGnosisBeacon:
		return "snapshots-gnosis-beacon"
	case chainsv1alpha2.ChainCronosZkEVM:
		return "snapshots-cronos-zkevm"
	case chainsv1alpha2.ChainSonic:
		return "snapshots-sonic"
	case chainsv1alpha2.ChainGoat:
		return "snapshots-goat"
	case chainsv1alpha2.ChainKatana:
		return "snapshots-katana"
	case chainsv1alpha2.ChainMezo:
		return "snapshots-mezo"
	case chainsv1alpha2.ChainPlasma:
		return "snapshots-plasma"
	case chainsv1alpha2.ChainPlaynance:
		return "snapshots-playnance"
	case chainsv1alpha2.ChainKusama:
		return "snapshots-kusama"
	case chainsv1alpha2.ChainHyperliquid:
		return "snapshots-hyperliquid"
	case chainsv1alpha2.ChainMonad:
		return "snapshots-monad"
	default:
		return "snapshots-eth"
	}
}

// bootstrapPy is the Python bootstrap script. Pure Python, no system package deps.
const bootstrapPy = `
import os, sys, io, tarfile, requests
from minio import Minio
from minio.error import S3Error
from urllib.parse import urlparse
from datetime import datetime, date, timedelta

DATA_DIR       = "/data"
CHAIN          = os.environ.get("CHAIN", "")
BUCKET         = os.environ.get("SNAPSHOT_BUCKET", "")
KEY            = os.environ.get("SNAPSHOT_KEY", "")
SNAPSHOT_TYPE  = os.environ.get("SNAPSHOT_TYPE", "full")  # "full" or "lite"

GB = 1 << 30
# Minimum expected snapshot sizes per chain (bytes). A downloaded file smaller
# than this threshold is almost certainly a partial/corrupt download.
MIN_SNAPSHOT_SIZE = {
    'bitcoin':    500 * GB,
    'ethereum':   500 * GB,
    'tron':        50 * GB,
    'cosmos':      10 * GB,
    'sui':        100 * GB,
    'avalanche':  200 * GB,
    'polygon':    200 * GB,
    'cardano':     50 * GB,
    'near':        50 * GB,
    'bsc':        200 * GB,
}


def cleanup_partial_downloads():
    """Remove .part.minio files left by the old MinIO SDK. The __snapshot_download.tmp
    file is NOT removed here — it is the resume checkpoint for the current download."""
    try:
        for e in os.scandir(DATA_DIR):
            if e.name.endswith(".part.minio"):
                path = os.path.join(DATA_DIR, e.name)
                print(f"Removing partial download: {path}", flush=True)
                try:
                    os.remove(path)
                except OSError as exc:
                    print(f"Warning: could not remove {path}: {exc}", flush=True)
    except FileNotFoundError:
        pass


def validate_snapshot_size(filepath, expected_size=None):
    """Validate downloaded snapshot file size.
    Returns True if valid, False if file should be rejected."""
    try:
        actual = os.path.getsize(filepath)
    except OSError:
        return False
    # Check against expected size from source (MinIO stat or Content-Length)
    if expected_size and expected_size > 0:
        if actual < expected_size * 0.95:  # Allow 5% tolerance for compression differences
            print(f"ERROR: Snapshot size mismatch: got {actual >> 20} MiB, expected {expected_size >> 20} MiB", flush=True)
            return False
    # Check against minimum threshold for the chain
    min_size = MIN_SNAPSHOT_SIZE.get(CHAIN, 0)
    if min_size > 0 and actual < min_size:
        print(f"WARNING: Snapshot too small for {CHAIN}: {actual >> 20} MiB < minimum {min_size >> 20} MiB", flush=True)
        return False
    print(f"Snapshot size OK: {actual >> 20} MiB", flush=True)
    return True


def _dir_has_data(path, min_bytes=1 << 20):
    """Check if a directory tree contains at least min_bytes of actual file data.
    Returns False for dirs that only contain empty subdirs or small metadata files
    (e.g. TRON creates 948K of empty RocksDB structure on first start)."""
    total = 0
    try:
        for root, dirs, files in os.walk(path):
            for f in files:
                try:
                    total += os.path.getsize(os.path.join(root, f))
                except OSError:
                    pass
                if total >= min_bytes:
                    return True
    except (FileNotFoundError, PermissionError):
        pass
    return False


def data_dir_empty():
    """Check if /data has no meaningful content.
    Ignores: lost+found, partial downloads, and directories with less than 1MB
    of actual data (some nodes create empty RocksDB structures on first start
    before the init container runs)."""
    try:
        for e in os.scandir(DATA_DIR):
            if e.name == "lost+found":
                continue
            if e.name.endswith(".part.minio") or e.name.startswith("__snapshot_download.tmp"):
                continue
            if e.is_dir():
                if not _dir_has_data(os.path.join(DATA_DIR, e.name)):
                    print(f"  Ignoring near-empty directory: {e.name}", flush=True)
                    continue
            return False
        return True
    except FileNotFoundError:
        return True


def default_bucket(chain):
    if chain == "bitcoin":
        return "snapshots-btc"
    if chain == "tron":
        return "snapshots-tron"
    return "snapshots-eth"


def extract_stream(stream_iter, name, strip_prefix="", target_dir=None):
    """Extract archive from a streaming iterator without buffering entire archive."""
    target = target_dir or DATA_DIR
    name_lower = name.split("?")[0].rstrip("/").split("/")[-1].lower()
    print(f"Extracting {name_lower} -> {target} (strip_prefix={strip_prefix!r})", flush=True)

    class ChunkReader:
        """Wraps chunk iterator as file-like object for streaming tarfile extraction."""
        def __init__(self, it):
            self._it = it
            self._buf = b''
            self._done = 0
        def _fetch(self):
            chunk = next(self._it)
            prev = self._done
            self._done += len(chunk)
            if (self._done >> 27) != (prev >> 27):  # log every ~128 MiB
                print(f"  downloaded {self._done >> 20} MiB", flush=True)
            self._buf += chunk
        def read(self, size=-1):
            if size < 0:
                try:
                    while True:
                        self._fetch()
                except StopIteration:
                    pass
                data = self._buf
                self._buf = b''
                return data
            while len(self._buf) < size:
                try:
                    self._fetch()
                except StopIteration:
                    break
            result = self._buf[:size]
            self._buf = self._buf[size:]
            return result

    reader = ChunkReader(stream_iter)

    def _strip_members(tf):
        """Yield tar members with optional prefix stripping and normalized ownership.

        Extracted files are normalized to uid=0 / gid=1000 (matching the operator's
        fsGroup=1000 hardcoded in the controller). Directories get at least 0o775
        (group-writable) so the main container (any UID with supplemental gid 1000)
        can traverse, create, and delete entries.
        """
        for member in tf:
            if strip_prefix and member.name.startswith(strip_prefix):
                member.name = member.name[len(strip_prefix):]
                if not member.name:
                    continue
            # Normalize ownership: uid=root so no single container "owns" the files,
            # gid=1000 so any pod with fsGroup=1000 has group access.
            member.uid = 0; member.gid = 1000; member.uname = "root"; member.gname = ""
            if member.isdir():
                member.mode = member.mode | 0o775  # group-writable dirs
            elif not (member.mode & 0o400):
                member.mode = member.mode | 0o440  # ensure files are at least group-readable
            yield member

    if name_lower.endswith(".tar.lz4"):
        import lz4.frame
        with lz4.frame.open(reader) as lz4f:
            with tarfile.open(fileobj=lz4f, mode="r|") as tf:
                tf.extractall(target, members=_strip_members(tf))
    elif name_lower.endswith(".tgz") or name_lower.endswith(".tar.gz"):
        with tarfile.open(fileobj=reader, mode="r|gz") as tf:
            tf.extractall(target, members=_strip_members(tf))
    elif name_lower.endswith(".tar"):
        with tarfile.open(fileobj=reader, mode="r|") as tf:
            tf.extractall(target, members=_strip_members(tf))
    else:
        raise RuntimeError(f"Unknown archive format: {name_lower}")
    print(f"Extraction complete ({reader._done >> 20} MiB downloaded).", flush=True)
    # Add group-write to the target mount root so the main pod (fsGroup=1000) can
    # unlink entries created by this init container (which runs as root).
    try:
        os.chmod(target, os.stat(target).st_mode | 0o020)
    except Exception as _e:
        print(f"Warning: could not set group-write on {target}: {_e}", flush=True)


def stream_iter_from_url(url, headers=None):
    resp = requests.get(url, stream=True, headers=headers or {}, timeout=7200)
    resp.raise_for_status()
    total = int(resp.headers.get("content-length", 0))
    if total:
        print(f"  size: {total >> 20} MiB", flush=True)
    return resp.iter_content(chunk_size=1 << 20)


def stream_iter_from_minio(client, bucket, key):
    # Download to a temp file on /data PVC to avoid IncompleteRead on slow/flaky connections.
    # /data has plenty of space (PVC is 1-2Ti); streaming breaks on large objects.
    tmp_path = os.path.join(DATA_DIR, "__snapshot_download.tmp")
    stat = client.stat_object(bucket, key)
    expected_size = stat.size or 0
    total_mb = expected_size >> 20
    print(f"  Downloading {total_mb} MiB to {tmp_path} ...", flush=True)
    # Retry up to 3 times on connection errors (large files are prone to TCP resets).
    for _attempt in range(3):
        try:
            client.fget_object(bucket, key, tmp_path)
            break
        except Exception as _e:
            if _attempt < 2:
                print(f"  Download attempt {_attempt+1} failed ({_e}), retrying...", flush=True)
                try: os.remove(tmp_path)
                except OSError: pass
            else:
                raise

    print(f"  Download complete. Validating size...", flush=True)
    if not validate_snapshot_size(tmp_path, expected_size):
        try:
            os.remove(tmp_path)
        except OSError:
            pass
        raise RuntimeError(f"Snapshot validation failed for {key}: size mismatch or below minimum threshold")
    print(f"  Streaming from file.", flush=True)
    def _iter():
        try:
            chunk_size = 1 << 20
            with open(tmp_path, "rb") as f:
                while True:
                    chunk = f.read(chunk_size)
                    if not chunk:
                        break
                    yield chunk
        finally:
            try:
                os.remove(tmp_path)
            except OSError:
                pass
    return _iter()


def publicnode_latest_url(chain):
    if chain in ("ethereum", "ethereum-archive"):
        # Try Merkle.io Reth snapshots first (published Mon+Thu)
        for delta in range(0, 8):
            d = (date.today() - timedelta(days=delta)).strftime("%Y-%m-%d")
            url = f"https://downloads.merkle.io/reth-{d}.tar.lz4"
            try:
                head = requests.head(url, timeout=15, allow_redirects=True)
                if head.status_code == 200:
                    return url
            except Exception:
                continue
        raise RuntimeError("no Reth snapshot found on Merkle.io in last 8 days")
    if chain == "bitcoin":
        r = requests.get("https://snapshots.publicnode.com/bitcoin-latest-build-id.txt", timeout=30)
        r.raise_for_status()
        build_id = r.text.strip()
        meta = requests.get(f"https://snapshots.publicnode.com/bitcoin-{build_id}.json", timeout=30).json()
        urls = meta.get("urls", [])
        if not urls:
            raise RuntimeError("no URLs in publicnode bitcoin metadata")
        return urls[0]
    if chain == "tron":
        # SNAPSHOT_TYPE: "lite" → LiteFullNode (~60 GiB), "full" → FullNode (~2.9 TiB)
        if SNAPSHOT_TYPE == "lite":
            prefix = "LiteFullNode_output-directory"
        else:
            prefix = "FullNode_output-directory"
        for delta in range(0, 7):
            d = (date.today() - timedelta(days=delta)).strftime("%Y%m%d")
            url = f"http://34.143.247.77/backup{d}/{prefix}.tgz"
            try:
                head = requests.head(url, timeout=15)
                if head.status_code == 200:
                    return url
            except Exception:
                continue
        raise RuntimeError(f"could not find a recent TRON {prefix} snapshot")
    if chain == "avalanche":
        # PublicNode pruned mainnet snapshot (~543 GB compressed, ~574 GB uncompressed).
        # URL pattern: https://snapshots.publicnode.com/avalanche-pruned-{blockHeight}.tar.lz4
        # Fetch the page index and pick the most recent pruned mainnet snapshot.
        try:
            r = requests.get("https://publicnode.com/snapshots", timeout=30)
            r.raise_for_status()
            import re as _re
            urls = _re.findall(r'https://snapshots\.publicnode\.com/avalanche-pruned-\d+\.tar\.lz4', r.text)
            if urls:
                # Sort by block height (embedded in filename), pick the highest
                urls = sorted(set(urls), key=lambda u: int(_re.search(r'avalanche-pruned-(\d+)', u).group(1)), reverse=True)
                return urls[0]
        except Exception as exc:
            print(f"Warning: could not fetch PublicNode page ({exc}), trying permanent URL", flush=True)
        # Fallback: permanent URL alias (always points to latest)
        url = "https://snapshots.publicnode.com/avalanche-pruned.tar.lz4"
        head = requests.head(url, timeout=15, allow_redirects=True)
        if head.status_code == 200:
            return url
        raise RuntimeError("could not find avalanche pruned snapshot on PublicNode")
    if chain == "cosmos":
        # Polkachu provides cosmos snapshots with daily updates
        try:
            r = requests.get("https://polkachu.com/api/v2/chain_snapshots/cosmos", timeout=30)
            r.raise_for_status()
            data = r.json()
            if data and isinstance(data, list):
                latest = sorted(data, key=lambda x: x.get("snapshot_height", 0) or 0, reverse=True)[0]
                filename = latest["filename"]
                return f"https://snapshots.polkachu.com/snapshots/cosmos/{filename}"
        except Exception as exc:
            print(f"Polkachu API failed ({exc})", flush=True)
        raise RuntimeError("no cosmos snapshot found on Polkachu")
    if chain == "cardano":
        # Mithril aggregator provides official Cardano snapshots
        try:
            r = requests.get(
                "https://aggregator.release.mainnet.api.mithril.network/aggregator/artifact/snapshots",
                timeout=30
            )
            r.raise_for_status()
            snapshots = r.json()
            if snapshots and isinstance(snapshots, list):
                latest = snapshots[0]  # Already sorted newest first
                locations = latest.get("locations", [])
                if locations:
                    return locations[0]
        except Exception as exc:
            print(f"Mithril API failed ({exc})", flush=True)
        raise RuntimeError("no cardano snapshot found on Mithril aggregator")
    if chain == "aptos":
        # Community snapshots - check several providers
        for url in [
            "https://snapshot.nodeinfra.com/aptos/mainnet/latest.tar.lz4",
            "https://aptos-snapshot.nodeinfra.com/latest.tar.lz4",
        ]:
            try:
                head = requests.head(url, timeout=15, allow_redirects=True)
                if head.status_code == 200:
                    return url
            except Exception:
                continue
        raise RuntimeError("no aptos snapshot found — check https://aptosfoundation.org for official backup tools")
    if chain == "bsc":
        # BNB Chain official snapshots: https://github.com/bnb-chain/bsc-snapshots
        # Pruned Geth snapshot ~500 GiB
        import re as _re
        try:
            r = requests.get(
                "https://api.github.com/repos/bnb-chain/bsc-snapshots/releases/latest",
                timeout=30,
                headers={"Accept": "application/vnd.github+json", "User-Agent": "Mozilla/5.0"}
            )
            r.raise_for_status()
            body = r.json().get("body", "")
            urls = _re.findall(r'https://[^\s\)\"\<\>]+\.tar\.gz', body)
            if not urls:
                urls = _re.findall(r'https://[^\s\)\"\<\>]+\.tar\.lz4', body)
            if urls:
                pruned = [u for u in urls if "pruned" in u.lower() or "geth" in u.lower()]
                return pruned[0] if pruned else urls[0]
        except Exception as exc:
            print(f"Warning: GitHub releases API failed ({exc})", flush=True)
        raise RuntimeError("no BSC snapshot URL found in BNB Chain GitHub releases")
    raise RuntimeError(f"no fallback snapshot source for chain: {chain}")


# Compute strip_prefix for archives that wrap content in a top-level directory.
# TRON LiteFullNode_output-directory.tgz contains output-directory/database/...
# TRON FullNode_output-directory.tgz contains output-directory/database/...
# We strip output-directory/ so database/ lands directly in /data/database/.
def compute_strip_prefix(chain, snapshot_type):
    if chain == "tron":
        return "output-directory/"
    if chain == "bsc":
        return ""
    # Avalanche PublicNode snapshot extracts directly (no top-level dir wrapper).
    return ""

STRIP_PREFIX = compute_strip_prefix(CHAIN, SNAPSHOT_TYPE)

# --- main ---

os.makedirs(DATA_DIR, exist_ok=True)

# Clean up partial downloads from previous failed attempts
cleanup_partial_downloads()

if CHAIN != "polygon" and not data_dir_empty():
    print("/data is not empty — skipping snapshot restore.", flush=True)
    sys.exit(0)

endpoint   = os.environ["MINIO_ENDPOINT"]
access_key = os.environ["MINIO_ACCESS_KEY"]
secret_key = os.environ["MINIO_SECRET_KEY"]

parsed = urlparse(endpoint)
secure = parsed.scheme == "https"
host   = parsed.netloc or parsed.path

# Use urllib3 with no read timeout so large snapshot downloads (50+ GiB) don't time out.
import urllib3 as _urllib3
_http = _urllib3.PoolManager(
    timeout=_urllib3.Timeout(connect=60, read=None),
    retries=_urllib3.Retry(total=5, backoff_factor=2, status_forcelist=[500,502,503,504]),
)
client = Minio(host, access_key=access_key, secret_key=secret_key, secure=secure,
               http_client=_http)

# --- Polygon special handling: bor + heimdall archives ---
if CHAIN == "polygon":
    def subdir_empty(path):
        if not os.path.exists(path):
            return True
        entries = [
            e.name for e in os.scandir(path)
            if e.name != "lost+found" and not e.name.endswith(".part.minio")
        ]
        return len(entries) == 0

    bor_empty = subdir_empty("/data/bor")
    heimdall_empty = subdir_empty("/data/heimdall")

    if not bor_empty and not heimdall_empty:
        print("/data/bor and /data/heimdall both non-empty — skipping snapshot restore.", flush=True)
        sys.exit(0)

    os.makedirs("/data/bor", exist_ok=True)
    os.makedirs("/data/heimdall", exist_ok=True)

    poly_bucket = BUCKET or "snapshots-polygon"

    # Try MinIO
    try:
        objs = list(client.list_objects(poly_bucket, recursive=True))
        objs_sorted = sorted(objs, key=lambda o: o.last_modified or datetime.min, reverse=True)
        bor_obj = next((o for o in objs_sorted if "bor" in o.object_name.lower()), None)
        hdl_obj  = next((o for o in objs_sorted if "heimdall" in o.object_name.lower()), None)
        if bor_empty and bor_obj:
            print(f"Restoring Bor from MinIO: {bor_obj.object_name}", flush=True)
            extract_stream(stream_iter_from_minio(client, poly_bucket, bor_obj.object_name),
                           bor_obj.object_name, strip_prefix="", target_dir="/data")
            bor_empty = False
        if heimdall_empty and hdl_obj:
            print(f"Restoring Heimdall from MinIO: {hdl_obj.object_name}", flush=True)
            extract_stream(stream_iter_from_minio(client, poly_bucket, hdl_obj.object_name),
                           hdl_obj.object_name, strip_prefix="", target_dir="/data/heimdall")
            heimdall_empty = False
    except Exception as exc:
        print(f"MinIO error for polygon ({exc})", flush=True)

    # Fallback to public sources — scrape publicnode.com/snapshots for actual URL with block height
    def _polygon_publicnode_url(pattern, permanent):
        import re as _re
        try:
            r = requests.get("https://publicnode.com/snapshots", timeout=30)
            r.raise_for_status()
            urls = _re.findall(pattern, r.text)
            if urls:
                urls = sorted(set(urls), key=lambda u: int(_re.search(r'-(\d+)\.tar', u).group(1)), reverse=True)
                return urls[0]
        except Exception:
            pass
        head = requests.head(permanent, timeout=15, allow_redirects=True)
        if head.status_code == 200:
            return permanent
        raise RuntimeError(f"polygon snapshot not found: {permanent}")

    if bor_empty:
        try:
            url = _polygon_publicnode_url(
                r'https://snapshots\.publicnode\.com/polygon-bor-pruned-\d+\.tar\.lz4',
                "https://snapshots.publicnode.com/polygon-bor-pruned.tar.lz4",
            )
            print(f"Fetching Polygon Bor from publicnode: {url}", flush=True)
            extract_stream(stream_iter_from_url(url), url, strip_prefix="", target_dir="/data")
        except Exception as exc:
            print(f"No Polygon Bor snapshot available ({exc}) — starting without snapshot.", flush=True)
    if heimdall_empty:
        try:
            url = _polygon_publicnode_url(
                r'https://snapshots\.publicnode\.com/polygon-heimdall-\d+\.tar\.gz',
                "https://snapshots.publicnode.com/polygon-heimdall.tar.gz",
            )
            print(f"Fetching Polygon Heimdall from publicnode: {url}", flush=True)
            extract_stream(stream_iter_from_url(url), url, strip_prefix="", target_dir="/data/heimdall")
        except Exception as exc:
            print(f"No Polygon Heimdall snapshot available ({exc}) — starting without snapshot.", flush=True)

    print("Polygon snapshot restore complete.", flush=True)
    sys.exit(0)

bucket = BUCKET or default_bucket(CHAIN)
print(f"Checking MinIO bucket: {bucket}", flush=True)

restored = False

# --- Ethereum: stream download+extract from Merkle.io (no temp file) ---
if CHAIN in ("ethereum", "ethereum-archive"):
    import subprocess as _sp

    # Find latest Merkle.io Reth snapshot
    snap_url = None
    for delta in range(0, 8):
        d = (date.today() - timedelta(days=delta)).strftime("%Y-%m-%d")
        url = f"https://downloads.merkle.io/reth-{d}.tar.lz4"
        try:
            head = requests.head(url, timeout=15, allow_redirects=True)
            if head.status_code == 200:
                snap_url = url
                break
        except Exception:
            continue

    if snap_url:
        snap_file = os.path.join(DATA_DIR, "__eth_snapshot.tar.lz4")
        print(f"Downloading ETH snapshot: {snap_url}", flush=True)
        # aria2c: single connection (Merkle CDN may have range issues like PublicNode)
        rc = _sp.call(["aria2c", "-x", "1", "-s", "1", "-k", "64M",
                       "--max-tries=50", "--retry-wait=15", "--timeout=600",
                       "--continue=true", "--auto-file-renaming=false",
                       "--summary-interval=120", "--allow-overwrite=true",
                       "--file-allocation=none",
                       "-d", DATA_DIR, "-o", "__eth_snapshot.tar.lz4", snap_url])
        if rc == 0 and os.path.exists(snap_file):
            fsize = os.path.getsize(snap_file)
            print(f"Downloaded: {fsize >> 30} GiB. Extracting...", flush=True)
            try:
                erc = _sp.call(f"lz4 -d '{snap_file}' - | tar xf - -C '{DATA_DIR}'", shell=True)
                if erc != 0:
                    raise RuntimeError(f"lz4+tar rc={erc}")
                os.remove(snap_file)
                # Normalize permissions (fsGroup=1000)
                for root, dirs, files in os.walk(DATA_DIR):
                    for d in dirs:
                        p = os.path.join(root, d)
                        try: os.chmod(p, os.stat(p).st_mode | 0o775)
                        except OSError: pass
                try: os.chmod(DATA_DIR, os.stat(DATA_DIR).st_mode | 0o020)
                except OSError: pass
                restored = True
                print("ETH snapshot extracted.", flush=True)
            except Exception as exc:
                print(f"ETH extraction failed ({exc})", flush=True)
                try: os.remove(snap_file)
                except OSError: pass
        else:
            print("ETH download failed (aria2c error).", flush=True)
            try: os.remove(snap_file)
            except OSError: pass
            try: os.remove(snap_file + ".aria2")
            except OSError: pass
    else:
        print("No Merkle.io ETH snapshot found in last 8 days.", flush=True)

    if restored:
        print("Ethereum snapshot restore complete.", flush=True)
    else:
        print("No ETH snapshot — starting node without.", flush=True)
    sys.exit(0)

# --- Litecoin: download via aria2c (multi-connection, resume) then extract ---
if CHAIN == "litecoin":
    import subprocess as _sp

    def _aria2c_download(url, dest):
        """Download URL via aria2c with retry. Uses single connection for
        PublicNode CDN (mirrors have different Content-Length, breaking Range requests)."""
        rc = _sp.call(["aria2c", "-x", "1", "-s", "1", "-k", "64M",
                       "--max-tries=20", "--retry-wait=15", "--timeout=600",
                       "--max-file-not-found=5",
                       "--continue=true", "--auto-file-renaming=false",
                       "--summary-interval=60", "--allow-overwrite=true",
                       "--file-allocation=none",
                       "-d", os.path.dirname(dest), "-o", os.path.basename(dest), url])
        return rc == 0

    def _extract_file(filepath, name):
        """Extract a local .tar.lz4 file into /data."""
        def _file_iter():
            with open(filepath, "rb") as f:
                while True:
                    chunk = f.read(1 << 20)
                    if not chunk:
                        break
                    yield chunk
        extract_stream(_file_iter(), name, strip_prefix="")
        os.remove(filepath)

    print("Restoring LTC snapshot via aria2c from PublicNode...", flush=True)

    # Download and extract base
    base_url = "https://snapshots.publicnode.com/litecoin.tar.lz4"
    base_file = os.path.join(DATA_DIR, "__ltc_base.tar.lz4")
    print(f"Downloading base: {base_url}", flush=True)
    if _aria2c_download(base_url, base_file):
        print(f"Base downloaded: {os.path.getsize(base_file) >> 20} MiB. Extracting...", flush=True)
        try:
            _extract_file(base_file, "litecoin-base.tar.lz4")
            restored = True
            print("Base snapshot extracted.", flush=True)
        except Exception as exc:
            print(f"Base extraction failed ({exc})", flush=True)
            try: os.remove(base_file)
            except OSError: pass
    else:
        print("Base download failed (aria2c error). Cleaning temp...", flush=True)
        try: os.remove(base_file)
        except OSError: pass
        # Also remove .aria2 control file
        try: os.remove(base_file + ".aria2")
        except OSError: pass

    # Find part snapshot URL by probing block heights (PublicNode is SPA, no API).
    # Part filename: litecoin-part-{height}.tar.lz4
    # Probe from network tip downward with coarse step, then refine.
    part_url = None
    try:
        tip_r = requests.get("https://litecoinspace.org/api/blocks/tip/height", timeout=15)
        tip = int(tip_r.text.strip()) if tip_r.status_code == 200 else 3077000
        print(f"Network tip: {tip}. Probing for part snapshot...", flush=True)

        def _probe(height):
            url = f"https://snapshots.publicnode.com/litecoin-part-{height}.tar.lz4"
            r = _sp.run(["curl", "-sL", "-o", "/dev/null", "-w", "%{http_code}", "--max-time", "5",
                         "-r", "0-0", url], capture_output=True, text=True)
            return r.stdout.strip() in ("200", "206")

        # Coarse scan: step 100 over last 2000 blocks
        coarse_hit = None
        for h in range(tip, max(tip - 2000, 3074000), -100):
            if _probe(h):
                coarse_hit = h
                break
        # Fine scan around hit
        if coarse_hit:
            best = coarse_hit
            for h in range(coarse_hit + 99, coarse_hit - 1, -1):
                if _probe(h):
                    best = h
                    break
            part_url = f"https://snapshots.publicnode.com/litecoin-part-{best}.tar.lz4"
            print(f"Found part snapshot: {part_url}", flush=True)
        else:
            print("No part snapshot found in last 2000 blocks.", flush=True)
    except Exception as exc:
        print(f"Could not find part URL ({exc})", flush=True)

    if part_url:
        part_file = os.path.join(DATA_DIR, "__ltc_part.tar.lz4")
        print(f"Downloading part: {part_url}", flush=True)
        if _aria2c_download(part_url, part_file):
            print(f"Part downloaded: {os.path.getsize(part_file) >> 20} MiB. Extracting...", flush=True)
            try:
                _extract_file(part_file, "litecoin-part.tar.lz4")
                restored = True
                print("Part snapshot extracted.", flush=True)
            except Exception as exc:
                print(f"Part extraction failed ({exc}) — node will sync remaining.", flush=True)
        else:
            print("Part download failed (aria2c error) — node will sync remaining.", flush=True)
    else:
        print("No LTC part snapshot URL found — node will sync remaining from peers.", flush=True)

    if restored:
        print("Litecoin snapshot restore complete.", flush=True)
    else:
        print("No LTC snapshot available — starting node without snapshot.", flush=True)
    sys.exit(0)

try:
    objects = list(client.list_objects(bucket, recursive=True))
    if objects:
        if KEY:
            key = KEY
        else:
            key = sorted(objects, key=lambda o: o.last_modified or datetime.min, reverse=True)[0].object_name
        print(f"Using MinIO snapshot: {key}", flush=True)
        extract_stream(stream_iter_from_minio(client, bucket, key), key, STRIP_PREFIX)
        restored = True
    else:
        print(f"MinIO bucket '{bucket}' is empty — falling back to public source.", flush=True)
except S3Error as exc:
    print(f"MinIO S3Error ({exc}) — falling back to public source.", flush=True)
except Exception as exc:
    print(f"MinIO error ({exc}) — falling back to public source.", flush=True)

if not restored:
    print(f"Fetching public snapshot for chain '{CHAIN}'...", flush=True)
    try:
        url = publicnode_latest_url(CHAIN)
        print(f"Snapshot URL: {url}", flush=True)
        extract_stream(stream_iter_from_url(url), url, STRIP_PREFIX)
    except Exception as exc:
        print(f"No public snapshot available ({exc}) — starting node without snapshot.", flush=True)
        sys.exit(0)

print("Snapshot restore complete.", flush=True)
`

// BuildInitContainer returns a Kubernetes init container spec that downloads
// the latest snapshot and extracts it into /data before the main node starts.
//
// Pure Python implementation: no apt-get, lz4 via pip package, no system tools.
func BuildInitContainer(chain chainsv1alpha2.Chain, cfg Config) corev1.Container {
	bucket := cfg.Bucket
	if bucket == "" {
		bucket = bucketForChain(chain)
	}

	// For chains that need aria2c (large snapshots), install it first.
	preInstall := ""
	if chain == chainsv1alpha2.ChainLitecoin || chain == chainsv1alpha2.ChainEthereum {
		preInstall = "apt-get update -qq && apt-get install -y -qq aria2 curl lz4 >/dev/null 2>&1\n"
	}

	shellCmd := `set -e
` + preInstall + `python3 /dev/stdin << 'PYEOF'
` + bootstrapPy + `
PYEOF
`

	env := []corev1.EnvVar{
		{Name: "MINIO_ENDPOINT", Value: cfg.Endpoint},
		{
			Name: "MINIO_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "minio-snapshot-credentials"},
					Key:                  "MINIO_ACCESS_KEY",
				},
			},
		},
		{
			Name: "MINIO_SECRET_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "minio-snapshot-credentials"},
					Key:                  "MINIO_SECRET_KEY",
				},
			},
		},
		{Name: "CHAIN", Value: string(chain)},
		{Name: "SNAPSHOT_BUCKET", Value: bucket},
		{Name: "SNAPSHOT_KEY", Value: cfg.Key},
		{Name: "SNAPSHOT_TYPE", Value: cfg.SnapshotType},
	}

	snapshotImage := os.Getenv("SNAPSHOT_RESTORE_IMAGE")
	if snapshotImage == "" {
		snapshotImage = defaultSnapshotRestoreImage
	}

	return corev1.Container{
		Name:            "snapshot-restore",
		Image:           snapshotImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/bin/sh", "-c", shellCmd},
		Env:             env,
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/data"},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    mustParseQuantity("500m"),
				corev1.ResourceMemory: mustParseQuantity("512Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    mustParseQuantity("2"),
				corev1.ResourceMemory: mustParseQuantity("4Gi"),
			},
		},
	}
}

// mustParseQuantity parses a resource quantity string; panics if invalid.
func mustParseQuantity(s string) resource.Quantity {
	return resource.MustParse(s)
}
