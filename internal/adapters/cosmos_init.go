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
package adapters

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// cosmosToolboxImage provides a static busybox for node images that lack a
// shell or download tools. busybox wget speaks TLS without certificate
// validation; the genesis it fetches is still checked against its pinned
// SHA-256, and the light client re-verifies the state sync trust hash.
const cosmosToolboxImage = "busybox:1.37.0-musl"

// cosmosToolboxDir holds the busybox binary and its applet symlinks on the
// data volume, where the node container picks them up.
const cosmosToolboxDir = "/data/.chainplane"

// genesisFormat tells the init script how to turn the downloaded genesis
// file into /data/config/genesis.json.
type genesisFormat int

const (
	// genesisPlain is a genesis.json served as is.
	genesisPlain genesisFormat = iota
	// genesisGzip is a gzip-compressed genesis.json.
	genesisGzip
	// genesisRPC is a CometBFT RPC /genesis response; the genesis document
	// is its result.genesis field. Used when a chain publishes no static
	// genesis file.
	genesisRPC
)

// cosmosNode describes how to bring up a Cosmos SDK / CometBFT full node
// from an empty data volume. Its command is shared by the Cosmos-family
// adapters.
type cosmosNode struct {
	// Binary is the node daemon inside the image, e.g. "gaiad".
	Binary string
	// ChainID is passed to `<binary> init`.
	ChainID string
	// InitArgs are extra flags for `<binary> init`.
	InitArgs []string

	// GenesisURL is the official mainnet genesis, pinned to a commit or a
	// release so its content cannot move under GenesisSHA256.
	GenesisURL string
	// GenesisSHA256 is the SHA-256 of the file as downloaded, before any
	// decompression or unwrapping. Genesis is consensus-critical: a
	// mismatch stops the node instead of starting it on another chain.
	GenesisSHA256 string
	GenesisFormat genesisFormat

	// Seeds and PersistentPeers are comma-separated node_id@host:port
	// lists written into config.toml on first start.
	Seeds           string
	PersistentPeers string

	// StateSyncRPC are CometBFT RPC endpoints the light client verifies
	// state sync snapshots against (CometBFT wants at least two; one
	// endpoint may be listed twice). Empty disables state sync and the
	// node replays the chain from genesis.
	StateSyncRPC []string

	// Toolbox is set for images without sh, curl/wget, sha256sum or the
	// coreutils the script needs (distroless osmosis, mezo, sei): a
	// sidecar copies busybox to the data volume and the script runs on it,
	// with the image's own tools still taking precedence.
	Toolbox bool
}

// Command returns the main container command. On first start (no
// /data/config/genesis.json) it runs `<binary> init`, writes seeds and
// persistent peers into config.toml, configures CometBFT state sync from a
// trust height 2000 blocks below the current tip (unless a snapshot
// restore left application.db behind), turns on the CometBFT Prometheus
// endpoint (:26660, the "metrics" port) and finally installs the verified
// genesis. genesis.json is written last, so an interrupted first start
// is redone from scratch on the next one. On every start the keys of the
// mounted /config/app.toml are merged into /data/config/app.toml: the
// mounted file only carries overrides, and the node cannot start on a
// partial app.toml. The container args (adapter args plus spec.extraArgs)
// follow `<binary> start --home /data`.
func (n cosmosNode) Command() []string {
	if n.Toolbox {
		return []string{cosmosToolboxDir + "/busybox", "sh", "-c", n.script(), "--"}
	}
	return []string{"sh", "-c", n.script(), "--"}
}

// InitContainers installs busybox into cosmosToolboxDir when Toolbox is set.
// It is a native sidecar (restartPolicy Always) that idles after the copy:
// its startup probe holds the node container back until busybox is in
// place, and unlike a plain init container it also runs under chainsmoke.
func (n cosmosNode) InitContainers() []corev1.Container {
	if !n.Toolbox {
		return nil
	}
	return []corev1.Container{{
		Name:          "cosmos-toolbox",
		Image:         cosmosToolboxImage,
		RestartPolicy: new(corev1.ContainerRestartPolicyAlways),
		Command: []string{"sh", "-c", `set -e
D=` + cosmosToolboxDir + `
rm -rf $D
mkdir -p $D/bin
cp /bin/busybox $D/busybox
$D/busybox --install -s $D/bin
exec sleep 2147483647`},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{Command: []string{"test", "-x", cosmosToolboxDir + "/bin/wget"}},
			},
			PeriodSeconds:    1,
			FailureThreshold: 60,
		},
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
	}}
}

func (n cosmosNode) script() string {
	var b strings.Builder
	b.WriteString("set -e\n")
	if n.Toolbox {
		b.WriteString("PATH=$PATH:" + cosmosToolboxDir + "/bin\n")
	}
	b.WriteString(`B=` + n.Binary + `
H=/data
C=$H/config
get() {
  if command -v curl >/dev/null 2>&1; then curl -fsSL --retry 3 -o "$2" "$1"; else wget -qO "$2" "$1"; fi
}
# tset FILE SECTION KEY VALUE replaces KEY in [SECTION]; KEY matches with
# "_" or "-" (CometBFT forks differ), a missing key is left alone.
tset() {
  awk -v s="[$2]" -v k="$3" -v v="$4" '
    BEGIN { gsub(/[_-]/, "[_-]", k); re = "^[ \t]*" k "[ \t]*=" }
    /^[ \t]*\[/ { cur = $0; sub(/[ \t]*#.*/, "", cur); gsub(/[ \t]/, "", cur) }
    cur == s && $0 ~ re { sub(/=.*/, "= " v) }
    { print }' "$1" > "$1.tmp"
  mv "$1.tmp" "$1"
}
# tmerge OVERRIDE TARGET writes every key of OVERRIDE into TARGET: replaced
# in place when TARGET has it, appended to its section otherwise.
tmerge() {
  awk '
    function sect(l) { sub(/[ \t]*#.*/, "", l); gsub(/[ \t]/, "", l); return l }
    function key(l) { sub(/[ \t]*=.*/, "", l); sub(/^[ \t]+/, "", l); return l }
    function flush(  i, p) {
      for (i = 1; i <= n; i++) {
        split(order[i], p, SUBSEP)
        if (p[1] == cur && !(order[i] in done)) { print ov[order[i]]; done[order[i]] = 1 }
      }
    }
    FNR == NR {
      if ($0 ~ /^[ \t]*(#|$)/) next
      if ($0 ~ /^[ \t]*\[/) { s = sect($0); next }
      k = s SUBSEP key($0); if (!(k in ov)) order[++n] = k; ov[k] = $0; next
    }
    function blanks() { for (; nb > 0; nb--) print "" }
    /^[ \t]*$/ { nb++; next }
    /^[ \t]*\[/ { flush(); blanks(); print; cur = sect($0); next }
    { blanks() }
    /^[ \t]*[A-Za-z0-9_.-]+[ \t]*=/ { k = cur SUBSEP key($0); if (k in ov) { print ov[k]; done[k] = 1; next } }
    { print }
    END {
      flush()
      blanks()
      for (i = 1; i <= n; i++) {
        if (order[i] in done) continue
        split(order[i], p, SUBSEP)
        if (p[1] != last) { print ""; print p[1]; last = p[1] }
        print ov[order[i]]
      }
    }' "$1" "$2" > "$2.tmp"
  mv "$2.tmp" "$2"
}
if [ ! -f $C/genesis.json ]; then
  echo "initializing $H for ` + n.ChainID + `"
  $B init "${HOSTNAME:-chainplane}" --chain-id ` + n.ChainID + ` --home $H`)
	for _, a := range n.InitArgs {
		b.WriteString(" " + a)
	}
	b.WriteString(` > $H/init.log 2>&1 || { cat $H/init.log >&2; exit 1; }
  rm -f $C/genesis.json
  tset $C/config.toml instrumentation prometheus true
`)
	if n.Seeds != "" {
		// Sei's CometBFT fork has no seeds key and takes seeds as bootstrap peers.
		b.WriteString(`  tset $C/config.toml p2p seeds '"` + n.Seeds + `"'
  tset $C/config.toml p2p bootstrap_peers '"` + n.Seeds + `"'
`)
	}
	if n.PersistentPeers != "" {
		b.WriteString(`  tset $C/config.toml p2p persistent_peers '"` + n.PersistentPeers + `"'
`)
	}
	if len(n.StateSyncRPC) > 0 {
		b.WriteString(`  if [ -d $H/data/application.db ]; then
    echo "application.db present (snapshot restore), state sync stays off"
  else
    TRUST=
    for R in ` + strings.Join(n.StateSyncRPC, " ") + `; do
      TIP=$(get "$R/block" - 2>/dev/null | grep -oE '"height":"[0-9]+"' | head -n 1 | grep -oE '[0-9]+') || true
      [ -n "$TIP" ] && [ "$TIP" -gt 2000 ] || continue
      HASH=$(get "$R/block?height=$((TIP - 2000))" - 2>/dev/null | grep -oE '"hash":"[0-9A-F]{64}"' | head -n 1 | grep -oE '[0-9A-F]{64}') || true
      [ -n "$HASH" ] && { TRUST=$((TIP - 2000)); break; }
    done
    if [ -n "$TRUST" ]; then
      tset $C/config.toml statesync enable true
      tset $C/config.toml statesync rpc_servers '"` + strings.Join(n.StateSyncRPC, ",") + `"'
      tset $C/config.toml statesync trust_height $TRUST
      tset $C/config.toml statesync trust_hash '"'$HASH'"'
      echo "state sync from trust height $TRUST ($HASH)"
    else
      echo "no state sync RPC answered, syncing from genesis" >&2
    fi
  fi
`)
	}
	b.WriteString(`  get ` + n.GenesisURL + ` $C/genesis.dl
  if ! echo "` + n.GenesisSHA256 + `  $C/genesis.dl" | sha256sum -c - >/dev/null 2>&1; then
    rm -f $C/genesis.dl
    echo "genesis from ` + n.GenesisURL + ` does not match pinned sha256 ` + n.GenesisSHA256 + `" >&2
    exit 1
  fi
`)
	switch n.GenesisFormat {
	case genesisGzip:
		b.WriteString(`  gunzip -c $C/genesis.dl > $C/genesis.part
  rm -f $C/genesis.dl
`)
	case genesisRPC:
		// The RPC serves compact JSON on one line: strip the JSON-RPC
		// envelope around result.genesis.
		b.WriteString(`  sed -e 's/^{"jsonrpc":"2.0","id":-1,"result":{"genesis"://' -e 's/}}[[:space:]]*$//' $C/genesis.dl > $C/genesis.part
  rm -f $C/genesis.dl
`)
	case genesisPlain:
		b.WriteString(`  mv $C/genesis.dl $C/genesis.part
`)
	}
	b.WriteString(`  mv $C/genesis.part $C/genesis.json
fi
[ -s /config/app.toml ] && tmerge /config/app.toml $C/app.toml
exec $B start --home $H "$@"`)
	return b.String()
}
