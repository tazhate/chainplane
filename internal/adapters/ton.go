package adapters

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type tonAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainTON, &tonAdapter{
		baseAdapter: baseAdapter{livenessPort: 30003},
	})
}

// --------------------------------------------------------------------------
// TON mainnet global config (static, embedded)
// --------------------------------------------------------------------------

// tonGlobalConfig is the TON mainnet global config (fetched 2026-03-13 from
// https://ton.org/global-config.json). Update occasionally when new validators
// are added to the network. Stored as a raw string to avoid template overhead.
const tonGlobalConfig = `{"@type":"config.global","dht":{"@type":"dht.config.global","k":6,"a":3,"static_nodes":{"@type":"dht.nodes","nodes":[{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"6PGkPQSbyFp12esf1NqmDOaLoFA8i9+Mp5+cAx5wtTU="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":-1185526007,"port":22096}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"L4N1+dzXLlkmT5iPnvsmsixzXU0L6kPKApqMdcrGP5d9ssMhn69SzHFK+yIzvG6zQ9oRb4TnqPBaKShjjj2OBg=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"4R0C/zU56k+x2HGMsLWjX2rP/SpoTPIHSSAmidGlsb8="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":-1952265919,"port":14395}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"0uwWyCFn2KjPnnlbSFYXLZdwIakaSgI9WyRo87J3iCGwb5TvJSztgA224A9kNAXeutOrXMIPYv1b8Zt8ImsrCg=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"/YDNd+IwRUgL0mq21oC0L3RxrS8gTu0nciSPUrhqR78="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":-1402455171,"port":14432}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"6+oVk6HDtIFbwYi9khCc8B+fTFceBUo1PWZDVTkb4l84tscvr5QpzAkdK7sS5xGzxM7V7YYQ6gUQPrsP9xcLAw=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"DA0H568bb+LoO2LGY80PgPee59jTPCqqSJJzt1SH+KE="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":-1402397332,"port":14583}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"cL79gDTrixhaM9AlkCdZWccCts7ieQYQBmPxb/R7d7zHw3bEHL8Le96CFJoB1KHu8C85iDpFK8qlrGl1Yt/ZDg=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"MJr8xja0xpu9DoisFXBrkNHNx1XozR7HHw9fJdSyEdo="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":-2018147130,"port":6302}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"XcR5JaWcf4QMdI8urLSc1zwv5+9nCuItSE1EDa0dSwYF15R/BtJoKU5YHA4/T8SiO18aVPQk2SL1pbhevuMrAQ=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"Fhldu4zlnb20/TUj9TXElZkiEmbndIiE/DXrbGKu+0c="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":-2018147075,"port":6302}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"nUGB77UAkd2+ZAL5PgInb3TvtuLLXJEJ2icjAUKLv4qIGB3c/O9k/v0NKwSzhsMP0ljeTGbcIoMDw24qf3goCg=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"gzUNJnBJhdpooYCE8juKZo2y4tYDIQfoCvFm0yBr7y0="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":89013260,"port":54390}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"LCrCkjmkMn6AZHW2I+oRm1gHK7CyBPfcb6LwsltskCPpNECyBl1GxZTX45n0xZtLgyBd/bOqMPBfawpQwWt1BA=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"jXiLaOQz1HPayilWgBWhV9xJhUIqfU95t+KFKQPIpXg="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":94452896,"port":12485}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"fKSZh9nXMx+YblkQXn3I/bndTD0JZ1yAtK/tXPIGruNglpe9sWMXR+8fy3YogPhLJMdjNiMom1ya+tWG7qvBAQ=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"vhFPq+tgjJi+4ZbEOHBo4qjpqhBdSCzNZBdgXyj3NK8="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":85383775,"port":36752}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"kBwAIgJVkz8AIOGoZcZcXWgNmWq8MSBWB2VhS8Pd+f9LLPIeeFxlDTtwAe8Kj7NkHDSDC+bPXLGQZvPv0+wHCg=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"sbsuMcdyYFSRQ0sG86/n+ZQ5FX3zOWm1aCVuHwXdgs0="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":759132846,"port":50187}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"9FJwbFw3IECRFkb9bA54YaexjDmlNBArimWkh+BvW88mjm3K2i5V2uaBPS3GubvXWOwdHLE2lzQBobgZRGMyCg=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"aeMgdMdkkbkfAS4+n4BEGgtqhkf2/zXrVWWECOJ/h3A="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":-1481887565,"port":25975}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"z5ogivZWpQchkS4UR4wB7i2pfOpMwX9Nd/USxinL9LvJPa+/Aw3F1AytR9FX0BqDftxIYvblBYAB5JyAmlj+AA=="},{"@type":"dht.node","id":{"@type":"pub.ed25519","key":"rNzhnAlmtRn9rTzW6o2568S6bbOXly7ddO1olDws5wM="},"addr_list":{"@type":"adnl.addressList","addrs":[{"@type":"adnl.address.udp","ip":-2134428422,"port":45943}],"version":0,"reinit_date":0,"priority":0,"expire_at":0},"version":-1,"signature":"sn/+ZfkfCSw2bHnEnv04AXX/Goyw7+StHBPQOdPr+wvdbaJ761D7hyiMNdQGbuZv2Ep2cXJpiwylnZItrwdUDg=="}]}},"liteservers":[{"ip":84478511,"port":19949,"id":{"@type":"pub.ed25519","key":"n4VDnSCUuSpjnCyUk9e3QOOd6o0ItSWYbTnW3Wnn8wk="}},{"ip":84478479,"port":48014,"id":{"@type":"pub.ed25519","key":"3XO67K/qi+gu3T9v8G2hx1yNmWZhccL3O7SoosFo8G0="}},{"ip":-2018135749,"port":53312,"id":{"@type":"pub.ed25519","key":"aF91CuUHuuOv9rm2W5+O/4h38M3sRm40DtSdRxQhmtQ="}},{"ip":-2018145068,"port":13206,"id":{"@type":"pub.ed25519","key":"K0t3+IWLOXHYMvMcrGZDPs+pn58a17LFbnXoQkKc2xw="}},{"ip":-2018145059,"port":46995,"id":{"@type":"pub.ed25519","key":"wQE0MVhXNWUXpWiW5Bk8cAirIh5NNG3cZM1/fSVKIts="}},{"ip":1091931625,"port":30131,"id":{"@type":"pub.ed25519","key":"wrQaeIFispPfHndEBc0s0fx7GSp8UFFvebnytQQfc6A="}},{"ip":1091931590,"port":47160,"id":{"@type":"pub.ed25519","key":"vOe1Xqt/1AQ2Z56Pr+1Rnw+f0NmAA7rNCZFIHeChB7o="}},{"ip":1091931623,"port":17728,"id":{"@type":"pub.ed25519","key":"BYSVpL7aPk0kU5CtlsIae/8mf2B/NrBi7DKmepcjX6Q="}},{"ip":1091931589,"port":13570,"id":{"@type":"pub.ed25519","key":"iVQH71cymoNgnrhOT35tl/Y7k86X5iVuu5Vf68KmifQ="}},{"ip":-1539021362,"port":52995,"id":{"@type":"pub.ed25519","key":"QnGFe9kihW+TKacEvvxFWqVXeRxCB6ChjjhNTrL7+/k="}},{"ip":-1539021936,"port":20334,"id":{"@type":"pub.ed25519","key":"gyLh12v4hBRtyBygvvbbO2HqEtgl+ojpeRJKt4gkMq0="}},{"ip":-1136338705,"port":19925,"id":{"@type":"pub.ed25519","key":"ucho5bEkufbKN1JR1BGHpkObq602whJn3Q3UwhtgSo4="}},{"ip":868465979,"port":19434,"id":{"@type":"pub.ed25519","key":"J5CwYXuCZWVPgiFPW+NY2roBwDWpRRtANHSTYTRSVtI="}},{"ip":868466060,"port":23067,"id":{"@type":"pub.ed25519","key":"vX8d0i31zB0prVuZK8fBkt37WnEpuEHrb7PElk4FJ1o="}},{"ip":-2018147130,"port":53560,"id":{"@type":"pub.ed25519","key":"NlYhh/xf4uQpE+7EzgorPHqIaqildznrpajJTRRH2HU="}},{"ip":-2018147075,"port":46529,"id":{"@type":"pub.ed25519","key":"jLO6yoooqUQqg4/1QXflpv2qGCoXmzZCR+bOsYJ2hxw="}},{"ip":908566172,"port":51565,"id":{"@type":"pub.ed25519","key":"TDg+ILLlRugRB4Kpg3wXjPcoc+d+Eeb7kuVe16CS9z8="}},{"ip":-1185526007,"port":4701,"id":{"@type":"pub.ed25519","key":"G6cNAr6wXBBByWDzddEWP5xMFsAcp6y13fXA8Q7EJlM="}}],"validator":{"@type":"validator.config.global","zero_state":{"workchain":-1,"shard":-9223372036854775808,"seqno":0,"root_hash":"F6OpKZKqvqeFp6CQmFomXNMfMj2EnaUSOXN+Mh+wVWk=","file_hash":"XplPz01CXAps5qeSWUtxcyBfdAo5zVb1N979KLSKD24="},"init_block":{"workchain":-1,"shard":-9223372036854775808,"seqno":46894135,"root_hash":"MEjmmhLPlG68mbTPnKYcP/Sz/MiMQBV2OsASBOzBv58=","file_hash":"u9rAtFQ+kUFEnOs3w8Y7punMTiyQTXf1bRfkSs8dG+0="},"hardforks":[{"workchain":-1,"shard":-9223372036854775808,"seqno":8536841,"root_hash":"08Kpc9XxrMKC6BF/FeNHPS3MEL1/Vi/fQU/C9ELUrkc=","file_hash":"t/9VBPODF7Zdh4nsnA49dprO69nQNMqYL+zk5bCjV/8="}]}}
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *tonAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainTON, client)
}

func (a *tonAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "global-config.json", tonGlobalConfig, nil
}

// ContainerCommand redirects /var/ton-work/db to the persistent /data volume,
// installs a fake wget that serves the embedded global-config from the ConfigMap,
// and ensures liteserver is properly configured in config.json using jq.
//
// Dump restore: if DUMP_URL is set (via spec.extraEnv) and the DB is empty,
// the dump is streamed via curl | plzip -d | tar.
//
// Background seqno monitoring: a loop runs lite-client against external liteservers
// every 60 seconds, served via a minimal nc HTTP server on port 8081 for HealthCheck.
func (a *tonAdapter) ContainerCommand(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{
		"bash", "-c",
		`mkdir -p /data/ton-work/db /var/ton-work
rm -rf /var/ton-work/db && ln -sf /data/ton-work/db /var/ton-work/db
mkdir -p /usr/local/bin
cat > /usr/local/bin/wget << 'WGETEOF'
#!/bin/sh
OUTPUT=""
PREV=""
for arg in "$@"; do
  if [ "$PREV" = "-O" ]; then OUTPUT="$arg"; fi
  PREV="$arg"
done
if [ -n "$OUTPUT" ] && echo "$OUTPUT" | grep -q "ton-global"; then
  cp /config/global-config.json "$OUTPUT" && exit 0
fi
exec /bin/wget "$@"
WGETEOF
chmod +x /usr/local/bin/wget

# State dump restore from dump.ton.org (first-start only, skipped on subsequent restarts).
# DUMP_URL must be set in spec.extraEnv (e.g. https://dump.ton.org/dumps/latest.tar.lz).
# The .dump_done marker is written to PVC so re-download is skipped on restart.
DB=/data/ton-work/db
DUMP_URL="${DUMP_URL:-}"
DUMP_MARKER="/data/ton-work/.dump_done"
DUMP_FILE="/data/dump.tar.lz"
if [ -n "$DUMP_URL" ]; then
  DB_STATE_MB=$(du -sm "$DB/state" 2>/dev/null | cut -f1)
  DB_STATE_MB=${DB_STATE_MB:-0}
  # Self-heal: stale marker (written by buggy plzip pipe, state not actually extracted).
  if [ -f "$DUMP_MARKER" ] && [ "$DB_STATE_MB" -lt 1024 ]; then
    echo "[ton] Stale marker detected (state=${DB_STATE_MB}MB <1GiB) — removing to re-extract."
    rm -f "$DUMP_MARKER"
  fi
  if [ ! -f "$DUMP_MARKER" ]; then
    if [ "$DB_STATE_MB" -lt 1024 ]; then
      echo "[ton] State dir is ${DB_STATE_MB}MB (<1GiB), restoring from $DUMP_URL ..."
      # Wipe partial state to avoid ADNL/celldb corruption.
      rm -rf "$DB/state" "$DB/celldb" "$DB/catchains" 2>/dev/null || true
      rm -f "${DUMP_FILE%.lz}" 2>/dev/null || true
      mkdir -p "$DB"
      DUMP_FILE_MB=$(du -sm "$DUMP_FILE" 2>/dev/null | cut -f1)
      DUMP_FILE_MB=${DUMP_FILE_MB:-0}
      if [ "$DUMP_FILE_MB" -gt 184320 ]; then
        echo "[ton] Dump already downloaded (${DUMP_FILE_MB}MB), skipping download."
      else
        if ! command -v aria2c >/dev/null 2>&1; then
          echo "[ton] Installing aria2c..."
          apt-get update -qq && apt-get install -y -qq aria2 >/dev/null 2>&1
        fi
        echo "[ton] Downloading dump via aria2c..."
        aria2c -x 1 -s 1 -k 64M \
          --max-tries=50 --retry-wait=15 --timeout=600 \
          --continue=true --auto-file-renaming=false \
          --file-allocation=none --summary-interval=120 \
          -d "$(dirname "$DUMP_FILE")" -o "$(basename "$DUMP_FILE")" \
          "$DUMP_URL" \
          || echo "[ton] aria2c finished with errors, checking file..."
      fi
      if [ -f "$DUMP_FILE" ]; then
        plzip -d -c -n2 "$DUMP_FILE" | tar -xC "$DB" \
          && rm -f "$DUMP_FILE" "${DUMP_FILE%.lz}" \
          && touch "$DUMP_MARKER" \
          && echo "[ton] Dump restore complete." \
          || echo "[ton] Extraction failed — proceeding without dump."
      else
        echo "[ton] Dump download failed after $MAX_RETRIES attempts — proceeding without dump."
      fi
    else
      echo "[ton] State already present (${DB_STATE_MB}MB), skipping dump."
      touch "$DUMP_MARKER"
    fi
  else
    echo "[ton] Dump restore marker found, skipping."
  fi
fi
# Unset DUMP_URL so init.sh does not attempt a second download.
unset DUMP_URL

# Repair liteserver config if config.json exists but liteservers[] is empty.
if [ -f "$DB/config.json" ] && ! [ -f "$DB/liteserver" ]; then
  LS_COUNT=$(jq '.liteservers | length' "$DB/config.json" 2>/dev/null || echo 0)
  if [ "$LS_COUNT" = "0" ]; then
    echo "[ton-fix] liteservers empty in config.json — repairing"
    cd "$DB"
    read -r LS_ID1 LS_ID2 <<< $(generate-random-id -m keys -n liteserver)
    cp liteserver "$DB/keyring/$LS_ID1"
    jq --arg id "$LS_ID2" '.liteservers = [{"id": $id, "port": 30003}]' \
      "$DB/config.json" > "$DB/config.json.new" \
      && mv "$DB/config.json.new" "$DB/config.json"
    echo "[ton-fix] liteserver key=$LS_ID2 added to config.json"
  fi
fi

# Repair control interface in config.json if empty.
if [ -f "$DB/config.json" ] && [ -f "$DB/server" ] && [ -f "$DB/client" ]; then
  CTRL_COUNT=$(jq '.control | length' "$DB/config.json" 2>/dev/null || echo 0)
  if [ "$CTRL_COUNT" = "0" ]; then
    echo "[ton-fix] control[] empty in config.json — repairing for validator-engine-console"
    SERVER_B64=$(dd if="$DB/server.pub" bs=1 skip=4 count=32 2>/dev/null | base64)
    CLIENT_B64=$(dd if="$DB/client.pub" bs=1 skip=4 count=32 2>/dev/null | base64)
    if [ -n "$SERVER_B64" ] && [ -n "$CLIENT_B64" ]; then
      jq --arg sid "$SERVER_B64" --arg cid "$CLIENT_B64" \
        '.control = [{"id": $sid, "port": 30002, "allowed": [{"id": $cid, "permissions": 15}]}]' \
        "$DB/config.json" > "$DB/config.json.new" \
        && mv "$DB/config.json.new" "$DB/config.json"
      echo "[ton-fix] control interface added (port 30002)"
    fi
  fi
fi

# PUBLIC_IP is injected via Kubernetes Downward API (status.hostIP).
if [ -n "$PUBLIC_IP" ]; then
  _a=$(echo "$PUBLIC_IP" | cut -d. -f1)
  _b=$(echo "$PUBLIC_IP" | cut -d. -f2)
  _c=$(echo "$PUBLIC_IP" | cut -d. -f3)
  _d=$(echo "$PUBLIC_IP" | cut -d. -f4)
  IP_UINT=$(( (_a << 24) | (_b << 16) | (_c << 8) | _d ))
  IP_INT=$(( IP_UINT > 2147483647 ? IP_UINT - 4294967296 : IP_UINT ))
  if [ -f "$DB/config.json" ]; then
    jq --argjson ip "$IP_INT" \
      'if .addrs then .addrs = [.addrs[] | if .port == 30001 then .ip = $ip else . end] else . end' \
      "$DB/config.json" > "$DB/config.json.new" \
      && mv "$DB/config.json.new" "$DB/config.json"
    echo "[ton] ADNL IP set to $PUBLIC_IP ($IP_INT) in config.json"
  fi
fi

# Background seqno updater
(while true; do
  NET_OUT=$(timeout 15 lite-client -C /config/global-config.json --cmd last 2>&1 || true)
  NET_SEQNO=$(echo "$NET_OUT" | grep 'masterchain block' | sed -n 's/.*(-1,[^,]*,\([0-9]*\)).*/\1/p' | head -1)
  [ -n "$NET_SEQNO" ] && echo "$NET_SEQNO" > /tmp/ton_net_seqno

  if [ -f "$DB/config.json" ] && [ -f "$DB/server" ] && [ -f "$DB/client" ]; then
    CTRL_COUNT=$(jq '.control | length' "$DB/config.json" 2>/dev/null || echo 0)
    if [ "$CTRL_COUNT" = "0" ]; then
      SERVER_B64=$(dd if="$DB/server.pub" bs=1 skip=4 count=32 2>/dev/null | base64)
      CLIENT_B64=$(dd if="$DB/client.pub" bs=1 skip=4 count=32 2>/dev/null | base64)
      if [ -n "$SERVER_B64" ] && [ -n "$CLIENT_B64" ]; then
        jq --arg sid "$SERVER_B64" --arg cid "$CLIENT_B64" \
          '.control = [{"id": $sid, "port": 30002, "allowed": [{"id": $cid, "permissions": 15}]}]' \
          "$DB/config.json" > "$DB/config.json.new" \
          && mv "$DB/config.json.new" "$DB/config.json"
        echo "[ton-fix] control interface lazily added (port 30002)"
      fi
    fi
    CONS_OUT=$(timeout 10 validator-engine-console \
      -a 127.0.0.1:30002 \
      -k "$DB/client" \
      -p "$DB/server.pub" \
      -c "getstat" -c "quit" 2>&1 || true)
    LOC_SEQNO=$(echo "$CONS_OUT" | grep -oP 'masterchainblock\s+\(-1,\w+,\K\d+' | head -1)
    if [ -z "$LOC_SEQNO" ]; then
      LOC_SEQNO=$(echo "$CONS_OUT" | sed -n 's/.*(-1,[^,]*,\([0-9]\{6,\}\)).*/\1/p' | head -1)
    fi
    [ -n "$LOC_SEQNO" ] && echo "$LOC_SEQNO" > /tmp/ton_local_seqno
  fi
  sleep 60
done) &

# HTTP server on port 8081 serving "local_seqno/network_seqno" for HealthCheck.
(while true; do
  NET=$(cat /tmp/ton_net_seqno 2>/dev/null || echo "0")
  LOCAL=$(cat /tmp/ton_local_seqno 2>/dev/null || echo "0")
  BODY="$LOCAL/$NET"
  LEN=${#BODY}
  { printf 'HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s' "$LEN" "$BODY"; sleep 1; } \
    | nc -l 8081 2>/dev/null || true
done) &

cd /var/ton-work/db
exec /var/ton-work/scripts/init.sh`,
	}
}

// HealthCheck queries the background seqno HTTP server (port 8081) which is started
// in ContainerCommand. The server returns "local_seqno/network_seqno".
func (a *tonAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	u, err := url.Parse(rpcURL)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("ton parse rpcURL: %w", err)
	}
	seqnoURL := "http://" + u.Hostname() + ":8081"

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, seqnoURL, nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("ton seqno request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return syncingPseudo(), nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))

	parts := strings.SplitN(strings.TrimSpace(string(body)), "/", 2)
	if len(parts) != 2 {
		return syncingPseudo(), nil
	}

	localSeqno := parseDecimalInt64(parts[0])
	netSeqno := parseDecimalInt64(parts[1])

	if netSeqno == 0 {
		return syncingPseudo(), nil
	}
	if localSeqno == 0 {
		p := pseudoBlock()
		return SyncStatus{IsSyncing: true, CurrentBlock: p, HighestBlock: netSeqno, Progress: 0}, nil
	}

	isSyncing := localSeqno < netSeqno-5
	return SyncStatus{
		IsSyncing:    isSyncing,
		CurrentBlock: localSeqno,
		HighestBlock: netSeqno,
		Progress:     progressFromBlocks(localSeqno, netSeqno),
	}, nil
}

// StartupProbe gives TON up to 24 hours (2880x30s) to complete the initial
// state dump download.
func (a *tonAdapter) StartupProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(30003, 300, 30, 10, 2880)
}

func (a *tonAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(30003, 60, 30, 10, 5)
}

// ContainerEnv injects PUBLIC_IP via Kubernetes Downward API (status.hostIP).
func (a *tonAdapter) ContainerEnv(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "PUBLIC_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
			},
		},
	}
}

func (a *tonAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "liteserver", ContainerPort: 30003, Protocol: corev1.ProtocolTCP},
		{Name: "p2p", ContainerPort: 30001, Protocol: corev1.ProtocolUDP},
		{Name: "seqno", ContainerPort: 8081, Protocol: corev1.ProtocolTCP},
	}
}

func (a *tonAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}

func (a *tonAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "ghcr.io",
		Repository: "ton-blockchain/ton",
		TagPattern: `^v\d+`,
	}
}

// NodePorts assigns unique P2P and liteserver ports per TON instance based on
// the node name suffix (-01/-02/-03).
func (a *tonAdapter) NodePorts(_ nodesv1alpha1.BlockchainNodeSpec, name string) map[int32]int32 {
	offset := int32(0)
	switch {
	case strings.HasSuffix(name, "-02"):
		offset = 10
	case strings.HasSuffix(name, "-03"):
		offset = 20
	}
	return map[int32]int32{
		30001: 30001 + offset, // P2P UDP
		30003: 30003 + offset, // liteserver TCP
	}
}
