# Fleet Status Dashboard

A lightweight read-only web dashboard for visualising all `ChainInstance` CRs in a Kubernetes cluster.

## Architecture

```
cmd/dashboard/main.go          — entry point (HTTP server + k8s client setup)
internal/dashboard/
  server.go                    — HTTP handlers, cache, k8s List loop
  metrics.go                   — Prometheus gauges/counters
  types.go                     — API response types
  embed.go                     — re-exports web.FS()
  server_test.go               — unit tests (fake k8s client)
web/
  index.html                   — single-file frontend (embedded in binary)
  embed.go                     — //go:embed index.html
```

## Building

```bash
make build-dashboard
# → bin/dashboard
```

## Running

```bash
# Against current kubeconfig context
./bin/dashboard --port 8090

# Watch a specific namespace only
./bin/dashboard --namespace blockchain --port 8090

# Explicit kubeconfig path
./bin/dashboard --kubeconfig ~/.kube/config --port 8090

# Faster refresh
./bin/dashboard --refresh 5s
```

### Flags

| Flag          | Default | Description                                      |
|---------------|---------|--------------------------------------------------|
| `--port`      | `8090`  | HTTP listen port                                 |
| `--namespace` | `""`    | Namespace to watch; empty = all namespaces       |
| `--kubeconfig`| `""`    | Path to kubeconfig; empty = in-cluster detection |
| `--refresh`   | `15s`   | Kubernetes List interval (cache TTL)             |

## Endpoints

| Path                          | Method | Description                          |
|-------------------------------|--------|--------------------------------------|
| `/`                           | GET    | Web UI (embedded `index.html`)       |
| `/api/nodes`                  | GET    | JSON list of all nodes + summary     |
| `/api/nodes/{namespace}/{name}` | GET  | JSON for a single node               |
| `/healthz`                    | GET    | Liveness probe — returns `ok`        |
| `/metrics`                    | GET    | Prometheus metrics                   |

## API Response

### `GET /api/nodes`

```json
{
  "nodes": [
    {
      "name": "ethereum-mainnet-0",
      "namespace": "blockchain",
      "chain": "ethereum",
      "network": "mainnet",
      "nodeType": "rpc",
      "phase": "Healthy",
      "blockHeight": 21500000,
      "syncProgress": "",
      "syncETA": "",
      "peersCount": 42,
      "ready": true,
      "age": "14d",
      "client": "nethermind",
      "replicas": 1
    }
  ],
  "summary": {
    "total": 1,
    "healthy": 1,
    "syncing": 0,
    "degraded": 0,
    "failed": 0,
    "pending": 0
  },
  "fetchedAt": "2026-03-28T10:00:00Z"
}
```

## Prometheus Metrics

| Metric                                    | Type    | Description                            |
|-------------------------------------------|---------|----------------------------------------|
| `bch_dashboard_nodes_total{phase}`        | Gauge   | Number of nodes per phase              |
| `bch_dashboard_block_height{chain,network,node}` | Gauge | Latest block height           |
| `bch_dashboard_sync_progress{chain,network,node}` | Gauge | Sync progress (0–100)        |
| `bch_dashboard_peers_total{chain,network,node}` | Gauge | Connected p2p peers            |
| `bch_dashboard_scrape_errors_total`       | Counter | Errors during k8s List calls           |
| `bch_dashboard_last_scrape_timestamp`     | Gauge   | Unix timestamp of last successful scrape |

## Web UI Features

- **Dark theme** with green (`#00ff88`) / red (`#ff4757`) accents
- **Summary bar** — total / healthy / syncing / degraded / failed / pending chips
- **Chain cards** — per-chain health ratio with progress bar; click to filter the table
- **Nodes table** — sortable columns, filter by text / phase / chain / namespace
- **Expandable rows** — click a row to see namespace, network, node type, client, age
- **Auto-refresh** every 15 s; "Last updated Xs ago" indicator in toolbar

## In-Cluster Deployment

The dashboard only needs **read** access to `ChainInstance` resources.
Add the following RBAC to your Helm values or directly to the cluster:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: bch-dashboard
rules:
- apiGroups: ["chainplane.io"]
  resources: ["chaininstances"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: bch-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: bch-dashboard
subjects:
- kind: ServiceAccount
  name: bch-dashboard
  namespace: default
```
