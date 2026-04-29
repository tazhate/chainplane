package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// xrpSyncedStates are server_state values where the node is healthy for RPC.
var xrpSyncedStates = map[string]bool{
	"full":       true,
	"validating": true,
	"proposing":  true,
}

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type xrpAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainXRP, &xrpAdapter{
		baseAdapter: baseAdapter{livenessPort: 5005},
	})
}

// --------------------------------------------------------------------------
// Config (static raw string -- rippled.cfg is not parameterized)
// --------------------------------------------------------------------------

const xrpConfig = `# rippled.cfg -- mainnet stock (non-validator) node
[server]
port_rpc
port_ws
port_peer

[port_rpc]
port = 5005
ip = 0.0.0.0
protocol = http

[port_ws]
port = 6006
ip = 0.0.0.0
protocol = ws

[port_peer]
port = 51235
ip = 0.0.0.0
protocol = peer

[node_db]
type = NuDB
path = /data/nudb
# online_delete keeps only recent ledgers (512 ~ 30 min of history at 3.5s/ledger)
online_delete = 512
advisory_delete = 0

[database_path]
/data/db

[debug_logfile]
/data/log/debug.log

[sntp_servers]
time.windows.com
time.apple.com
time.nist.gov
pool.ntp.org

# Persistent connections to Ripple's servers -- critical for initial sync.
[ips_fixed]
s1.ripple.com 51235
s2.ripple.com 51235

# Additional bootstrap peers for peer discovery
[ips]
r.ripple.com 51235
zaphod.alloy.ee 51235
sahyadri.isrdc.in 51235

[validator_list_sites]
https://vl.ripple.com
https://unl.xrplf.org

[validator_list_keys]
ED2677ABFFD1B33AC6FBC3062B71F1E8397C1505E1C42C64D11AD1B28FF73F4734
ED42AEC58B701EEBB77356FFFEC26F83C1F0407263530F068C7C73D392C7E06FD1

[validator_list_threshold]
0

# none = do not acquire historical ledger data, sync from current validated ledger
[ledger_history]
none

# huge: maximizes in-memory caches (~16GB+), reduces disk I/O during state tree acquisition.
[node_size]
huge
`

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *xrpAdapter) DefaultImage(client string) string {
	return DefaultImageFor(nodesv1alpha1.ChainXRP, client)
}

func (a *xrpAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "rippled.cfg", xrpConfig, nil
}

func (a *xrpAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	result, err := callRPC(ctx, rpcURL, "server_info", nil)
	if err != nil {
		return SyncStatus{}, fmt.Errorf("server_info: %w", err)
	}

	var resp struct {
		Info struct {
			ServerState     string `json:"server_state"`
			CompleteLedgers string `json:"complete_ledgers"`
			Peers           int32  `json:"peers"`
			ValidatedLedger struct {
				Seq int64 `json:"seq"`
				Age int64 `json:"age"`
			} `json:"validated_ledger"`
			ClosedLedger struct {
				Seq int64 `json:"seq"`
			} `json:"closed_ledger"`
		} `json:"info"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return SyncStatus{}, fmt.Errorf("parse server_info: %w", err)
	}

	info := resp.Info
	currentLedger := info.ValidatedLedger.Seq
	if currentLedger == 0 || info.CompleteLedgers == "empty" {
		return SyncStatus{
			IsSyncing:    true,
			CurrentBlock: info.ClosedLedger.Seq,
			Progress:     0,
			Peers:        info.Peers,
		}, nil
	}

	isSynced := xrpSyncedStates[info.ServerState] && info.ValidatedLedger.Age < 60
	progress := 99.0
	if isSynced {
		progress = 100.0
	}
	return SyncStatus{
		IsSyncing:    !isSynced,
		CurrentBlock: currentLedger,
		HighestBlock: currentLedger,
		Peers:        info.Peers,
		Progress:     progress,
	}, nil
}

func (a *xrpAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(5005, 60, 30, 10, 3)
}

// ContainerCommand overrides the default CMD so --conf is not duplicated.
// --net fetches the current validated ledger from peers on startup.
func (a *xrpAdapter) ContainerCommand(_ nodesv1alpha1.BlockchainNodeSpec) []string {
	return []string{"rippled", "--conf=/config/rippled.cfg", "--net"}
}

func (a *xrpAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 5005, Protocol: corev1.ProtocolTCP},
		{Name: "peer", ContainerPort: 51235, Protocol: corev1.ProtocolTCP},
	}
}

func (a *xrpAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "xrpllabsofficial/xrpld",
		TagPattern: `^\d+\.\d+`,
	}
}

func (a *xrpAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("100Gi"),
	}
}
