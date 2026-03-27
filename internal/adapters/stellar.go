package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultStellarImage = "stellar/stellar-core:v19.12.0"

// catchupRe matches stellar-core status lines like:
// "Catching up to ledger 61631039: Applying buckets 89%..."
// "Catching up to ledger 61631039: downloading and verifying buckets: 15/34 (44%)"
var catchupRe = regexp.MustCompile(`Catching up to ledger (\d+).*?(\d+)%`)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type stellarAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainStellar, &stellarAdapter{
		baseAdapter: baseAdapter{livenessPort: 11626},
	})
}

// --------------------------------------------------------------------------
// Config (static raw string -- stellar-core.cfg is not parameterized)
// --------------------------------------------------------------------------

const stellarConfig = `# stellar-core.cfg -- mainnet watcher (non-validator) node
NETWORK_PASSPHRASE="Public Global Stellar Network ; September 2015"
HTTP_PORT=11626
PUBLIC_HTTP_PORT=true
PEER_PORT=11625
LOG_FILE_PATH=""

# Watcher: no NODE_SEED, no NODE_IS_VALIDATOR (defaults false)
# Fast catchup: download only recent ledgers, not full history
CATCHUP_COMPLETE=false
CATCHUP_RECENT=8640
UNSAFE_QUORUM=true

DATABASE="sqlite3:///data/stellar.db"
BUCKET_DIR_PATH="/data/buckets"

# SDF Tier 1
[[HOME_DOMAINS]]
HOME_DOMAIN="www.stellar.org"
QUALITY="HIGH"

[[VALIDATORS]]
NAME="SDF_1"
PUBLIC_KEY="GCGB2S2KGYARPVIA37HYZXVRM2YZUEXA6S33ZU5BUDC6THSB62LZSTYH"
ADDRESS="core-live-a.stellar.org:11625"
HOME_DOMAIN="www.stellar.org"
HISTORY="curl -sf http://history.stellar.org.s3.amazonaws.com/prd/core-live/core_live_001/{0} -o {1}"

[[VALIDATORS]]
NAME="SDF_2"
PUBLIC_KEY="GCM6QMP3DLRPTAZW2UZPCPX2LF3SXWXKPMP3GKFZBDSF3QZGV2G5QSTK"
ADDRESS="core-live-b.stellar.org:11625"
HOME_DOMAIN="www.stellar.org"
HISTORY="curl -sf http://history.stellar.org.s3.amazonaws.com/prd/core-live/core_live_002/{0} -o {1}"

[[VALIDATORS]]
NAME="SDF_3"
PUBLIC_KEY="GABMKJM6I25XI4K7U6XWMULOUQIQ27BCTMLS6BYYSOWKTBUXVRJSXHYQ"
ADDRESS="core-live-c.stellar.org:11625"
HOME_DOMAIN="www.stellar.org"
HISTORY="curl -sf http://history.stellar.org.s3.amazonaws.com/prd/core-live/core_live_003/{0} -o {1}"

# History archives -- direct S3 bucket (Cloudflare CDN unreachable from pod CIDR)
[HISTORY.sdf_1]
get="curl -sf http://history.stellar.org.s3.amazonaws.com/prd/core-live/core_live_001/{0} -o {1}"

[HISTORY.sdf_2]
get="curl -sf http://history.stellar.org.s3.amazonaws.com/prd/core-live/core_live_002/{0} -o {1}"

[HISTORY.sdf_3]
get="curl -sf http://history.stellar.org.s3.amazonaws.com/prd/core-live/core_live_003/{0} -o {1}"
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *stellarAdapter) DefaultImage(_ string) string {
	return defaultStellarImage
}

func (a *stellarAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "stellar-core.cfg", stellarConfig, nil
}

func (a *stellarAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rpcURL+"/info", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("stellar info request: %w", err)
	}
	resp, err := rpcClient.Do(req)
	if err != nil {
		// stellar-core blocks its HTTP API during heavy I/O (e.g. Applying buckets).
		if isContextTimeout(err) {
			return SyncStatus{IsSyncing: true, StallExempt: true, Progress: 50.0}, nil
		}
		return SyncStatus{}, fmt.Errorf("stellar info: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var info struct {
		Info struct {
			State  string   `json:"state"`
			Status []string `json:"status"`
			Ledger struct {
				Num int64 `json:"num"`
			} `json:"ledger"`
			Peers struct {
				AuthenticatedCount int32 `json:"authenticated_count"`
			} `json:"peers"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return SyncStatus{}, fmt.Errorf("stellar info parse: %w", err)
	}

	// Stellar states: Booting -> Joining SCP -> Catching up -> Synced!
	isSyncing := info.Info.State != "Synced!"
	currentBlock := info.Info.Ledger.Num
	highestBlock := currentBlock
	progress := 100.0

	if isSyncing {
		// "Joining SCP" is the state just before "Synced!" -- node has finished history
		// replay and is joining consensus.
		if info.Info.State == "Joining SCP" {
			return SyncStatus{
				IsSyncing:    true,
				CurrentBlock: currentBlock,
				HighestBlock: currentBlock + 1,
				Progress:     99.0,
				Peers:        info.Info.Peers.AuthenticatedCount,
				StallExempt:  true,
			}, nil
		}

		progress = 0
		for _, s := range info.Info.Status {
			if m := catchupRe.FindStringSubmatch(s); m != nil {
				if target, err2 := strconv.ParseInt(m[1], 10, 64); err2 == nil {
					highestBlock = target
				}
				if pct, err2 := strconv.ParseFloat(m[2], 64); err2 == nil {
					progress = pct
				}
				break
			}
		}
		if highestBlock > currentBlock && currentBlock > 0 {
			progress = float64(currentBlock) / float64(highestBlock) * 100.0
		}
	}

	return SyncStatus{
		IsSyncing:    isSyncing,
		CurrentBlock: currentBlock,
		HighestBlock: highestBlock,
		Progress:     progress,
		Peers:        info.Info.Peers.AuthenticatedCount,
		StallExempt:  isSyncing && currentBlock <= 1,
	}, nil
}

// ContainerCommand initializes the DB if missing or too small, then runs stellar-core.
func (a *stellarAdapter) ContainerCommand(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{
		"sh", "-c",
		`size=$(stat -c%s /data/stellar.db 2>/dev/null || echo 0); [ "$size" -lt 1048576 ] && { rm -f /data/stellar.db; rm -rf /data/buckets; stellar-core new-db --conf /config/stellar-core.cfg; }; exec stellar-core run --conf /config/stellar-core.cfg`,
	}
}

func (a *stellarAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(11626, 120, 30, 10, 10)
}

func (a *stellarAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "http", ContainerPort: 11626, Protocol: corev1.ProtocolTCP},
		{Name: "peer", ContainerPort: 11625, Protocol: corev1.ProtocolTCP},
	}
}
