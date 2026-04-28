# chainplane

A Helm chart for the ChainPlane - a Kubernetes operator that manages the full lifecycle of blockchain nodes including StatefulSets, PVCs, Services, ConfigMaps, and health monitoring.

## Prerequisites

- Kubernetes >= 1.26
- Helm >= 3.x
- (Optional) Prometheus Operator for ServiceMonitor support
- (Optional) cert-manager for webhook/metrics TLS certificates

## Installation

```bash
helm repo add chainplane https://tazhate.github.io/chainplane
helm install chainplane chainplane/chainplane
```

Or from local source:

```bash
helm install chainplane ./charts/chainplane
```

## Configuration

### General

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of operator replicas | `2` |
| `image.repository` | Operator image repository | `controller` |
| `image.tag` | Operator image tag (defaults to appVersion) | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets | `[]` |
| `nameOverride` | Override chart name | `""` |
| `fullnameOverride` | Override full release name | `""` |
| `installCRDs` | Install CRDs with the chart | `true` |

### Service Account

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create a service account | `true` |
| `serviceAccount.annotations` | Service account annotations | `{}` |
| `serviceAccount.name` | Service account name override | `""` |

### Pod Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podAnnotations` | Pod annotations | `{}` |
| `podLabels` | Extra pod labels | `{}` |
| `podSecurityContext.runAsNonRoot` | Run as non-root | `true` |
| `securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `resources.requests.cpu` | CPU request | `10m` |
| `resources.requests.memory` | Memory request | `64Mi` |
| `nodeSelector` | Node selector | `{}` |
| `tolerations` | Tolerations | `[]` |
| `affinity` | Affinity rules | `{}` |

### Leader Election

| Parameter | Description | Default |
|-----------|-------------|---------|
| `leaderElection.enabled` | Enable leader election | `true` |

### Metrics

| Parameter | Description | Default |
|-----------|-------------|---------|
| `metrics.enabled` | Enable metrics endpoint | `true` |
| `metrics.port` | Metrics port | `8443` |
| `metrics.secure` | Serve metrics over HTTPS | `true` |
| `metrics.service.type` | Metrics service type | `ClusterIP` |
| `metrics.service.port` | Metrics service port | `8443` |
| `metrics.serviceMonitor.enabled` | Create ServiceMonitor | `false` |
| `metrics.serviceMonitor.labels` | Extra ServiceMonitor labels | `{}` |
| `metrics.serviceMonitor.interval` | Scrape interval | `""` |
| `metrics.serviceMonitor.scheme` | Scrape scheme | `https` |

### Webhook

| Parameter | Description | Default |
|-----------|-------------|---------|
| `webhook.enabled` | Enable validating webhook | `true` |
| `webhook.failurePolicy` | Webhook failure policy | `Fail` |
| `webhook.service.port` | Webhook service port | `443` |
| `webhook.service.targetPort` | Webhook pod port | `9443` |

### Prometheus (Health Checks)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `prometheus.url` | Prometheus URL for node health checks | `http://prometheus:9090` |

### Network Policy

| Parameter | Description | Default |
|-----------|-------------|---------|
| `networkPolicy.enabled` | Create NetworkPolicy | `false` |
| `networkPolicy.metricsNamespaceSelector` | Namespace selector for metrics access | `{metrics: enabled}` |

### Other

| Parameter | Description | Default |
|-----------|-------------|---------|
| `enableHTTP2` | Enable HTTP/2 for servers | `false` |
| `extraEnv` | Extra environment variables | `[]` |
| `extraArgs` | Extra command-line arguments | `[]` |
| `extraVolumes` | Extra volumes | `[]` |
| `extraVolumeMounts` | Extra volume mounts | `[]` |

## Upgrading

### CRD Updates

When `installCRDs: true`, CRDs are managed as part of the Helm release. Note that Helm will not delete CRDs on uninstall to prevent data loss. To update CRDs manually:

```bash
kubectl apply -f charts/chainplane/templates/crds/
```

## Uninstallation

```bash
helm uninstall chainplane
```

CRDs are not removed on uninstall. To remove them manually:

```bash
kubectl delete crd blockchainnodes.nodes.chainplane.io
```
