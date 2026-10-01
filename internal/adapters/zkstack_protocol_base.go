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
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// ZK Stack external node (zksync_external_node) ports. The JSON-RPC ports only
// open once the node storage is initialised, which takes hours on snapshot
// recovery, so liveness is probed on the healthcheck port that opens early.
const (
	zkStackHTTPPort    = 3060
	zkStackWSPort      = 3061
	zkStackHealthPort  = 3081
	zkStackMetricsPort = 3312
)

const (
	defaultZkStackL1URL = "http://ethereum:8545"

	// zkStackPostgresImage runs the pod-local database the external node keeps
	// its state in (upstream supports Postgres 14-16 and newer).
	zkStackPostgresImage = "postgres:16.15"
	// zkStackPostgresMountPath is where the "postgres" subPath of the data
	// volume is mounted, so the database lives and dies with the node PVC.
	zkStackPostgresMountPath = "/var/lib/postgresql/data"
	// zkStackDatabaseURL has no password: Postgres listens on 127.0.0.1 only,
	// which is reachable just from containers of the same pod, and uses trust
	// auth. The database name matches the upstream examples.
	zkStackDatabaseURL = "postgres://postgres@127.0.0.1:5432/zksync_local_ext_node"
)

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

// zkStackProtocolAdapter is the shared base for chains that run the ZK Stack
// external node (matterlabs/external-node and its forks). The node is
// configured through EN_* env vars only and needs Postgres, which it gets as a
// native sidecar.
type zkStackProtocolAdapter struct {
	protocolAdapter
	mainNodeURL string
	l2ChainID   int64
	// snapshotsBucket is the GCS bucket with external node snapshots. When
	// empty the node always syncs from genesis.
	snapshotsBucket string
}

// newZkStackProtocolAdapter returns the base with liveness on the healthcheck port.
func newZkStackProtocolAdapter(mainNodeURL string, l2ChainID int64, snapshotsBucket string) zkStackProtocolAdapter {
	return zkStackProtocolAdapter{
		protocolAdapter: protocolAdapter{livenessPort: zkStackHealthPort},
		mainNodeURL:     mainNodeURL,
		l2ChainID:       l2ChainID,
		snapshotsBucket: snapshotsBucket,
	}
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (b zkStackProtocolAdapter) ConfigTemplate(_ chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return "", "", nil
}

func (b zkStackProtocolAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

// StartupProbe gives the node up to 10 minutes to wire its components; the
// healthcheck server only starts after the L1 and main node clients answered.
func (b zkStackProtocolAdapter) StartupProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(zkStackHealthPort, 0, 10, 5, 60)
}

func (b zkStackProtocolAdapter) ContainerPorts(_ chainsv1alpha2.ChainInstanceSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: zkStackHTTPPort, Protocol: corev1.ProtocolTCP},
		{Name: "ws", ContainerPort: zkStackWSPort, Protocol: corev1.ProtocolTCP},
		{Name: "health", ContainerPort: zkStackHealthPort, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: zkStackMetricsPort, Protocol: corev1.ProtocolTCP},
	}
}

// ContainerEnv configures the external node. EN_ETH_CLIENT_URL defaults to an
// in-cluster Ethereum node; override it via spec.extraEnv. Archive nodes skip
// snapshot recovery, which only restores state from the snapshot batch on.
func (b zkStackProtocolAdapter) ContainerEnv(spec chainsv1alpha2.ChainInstanceSpec) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "DATABASE_URL", Value: zkStackDatabaseURL},
		{Name: "DATABASE_POOL_SIZE", Value: "50"},
		{Name: "EN_HTTP_PORT", Value: strconv.Itoa(zkStackHTTPPort)},
		{Name: "EN_WS_PORT", Value: strconv.Itoa(zkStackWSPort)},
		{Name: "EN_HEALTHCHECK_PORT", Value: strconv.Itoa(zkStackHealthPort)},
		{Name: "EN_PROMETHEUS_PORT", Value: strconv.Itoa(zkStackMetricsPort)},
		{Name: "EN_ETH_CLIENT_URL", Value: defaultZkStackL1URL},
		{Name: "EN_MAIN_NODE_URL", Value: b.mainNodeURL},
		{Name: "EN_L1_CHAIN_ID", Value: "1"},
		{Name: "EN_L2_CHAIN_ID", Value: strconv.FormatInt(b.l2ChainID, 10)},
		{Name: "EN_STATE_CACHE_PATH", Value: "/data/state_keeper"},
		{Name: "EN_MERKLE_TREE_PATH", Value: "/data/tree"},
	}
	if b.snapshotsBucket == "" || spec.NodeType == chainsv1alpha2.NodeTypeArchive {
		return append(env, corev1.EnvVar{Name: "EN_SNAPSHOTS_RECOVERY_ENABLED", Value: "false"})
	}
	return append(env,
		corev1.EnvVar{Name: "EN_SNAPSHOTS_RECOVERY_ENABLED", Value: "true"},
		corev1.EnvVar{Name: "EN_SNAPSHOTS_OBJECT_STORE_BUCKET_BASE_URL", Value: b.snapshotsBucket},
		corev1.EnvVar{Name: "EN_SNAPSHOTS_OBJECT_STORE_MODE", Value: "GCSAnonymousReadOnly"},
	)
}

// InitContainers adds Postgres as a native sidecar (restartPolicy Always), so
// it is ready before the node starts and outlives it on shutdown. The image
// entrypoint runs as root and resets PGDATA to 0700 on every start, undoing
// the group-write bits that fsGroup ownership adds to the volume.
func (b zkStackProtocolAdapter) InitContainers(_ chainsv1alpha2.ChainInstanceSpec) []corev1.Container {
	return []corev1.Container{
		{
			Name:          "postgres",
			Image:         zkStackPostgresImage,
			RestartPolicy: new(corev1.ContainerRestartPolicyAlways),
			Args: []string{
				"-c", "listen_addresses=127.0.0.1",
				"-c", "max_connections=200",
				// The pod /dev/shm is 64Mi; keep parallel query segments off it.
				"-c", "dynamic_shared_memory_type=sysv",
			},
			Env: []corev1.EnvVar{
				{Name: "POSTGRES_HOST_AUTH_METHOD", Value: "trust"},
				{Name: "PGDATA", Value: zkStackPostgresMountPath + "/pgdata"},
			},
			StartupProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{"pg_isready", "-h", "127.0.0.1", "-U", "postgres"},
					},
				},
				PeriodSeconds:    2,
				TimeoutSeconds:   2,
				FailureThreshold: 150,
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data", MountPath: zkStackPostgresMountPath, SubPath: "postgres"},
			},
		},
	}
}
