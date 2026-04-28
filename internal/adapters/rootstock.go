package adapters

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

const defaultRootstockImage = "rsksmart/rskj:ARROWHEAD-6.4.0"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type rootstockAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainRootstock, &rootstockAdapter{
		baseAdapter: baseAdapter{livenessPort: 4444},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *rootstockAdapter) DefaultImage(_ string) string {
	return defaultRootstockImage
}

func (a *rootstockAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "rsk.conf", rootstockConfig, nil
}

func (a *rootstockAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return evmHealthCheck(ctx, rpcURL)
}

func (a *rootstockAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "rpc", ContainerPort: 4444, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-tcp", ContainerPort: 5050, Protocol: corev1.ProtocolTCP},
		{Name: "p2p-udp", ContainerPort: 5050, Protocol: corev1.ProtocolUDP},
		{Name: "metrics", ContainerPort: 6060, Protocol: corev1.ProtocolTCP},
	}
}

func (a *rootstockAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "rsksmart/rskj",
		TagPattern: `^ARROWHEAD-\d+`,
		TagPrefix:  "ARROWHEAD-",
	}
}

func (a *rootstockAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("2"),
		MemoryRequest: resource.MustParse("4Gi"),
		Storage:       resource.MustParse("200Gi"),
	}
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

const rootstockConfig = `# Rootstock (RSK) mainnet RPC node
blockchain.config.name = "main"

database.dir = "/data"

rpc {
  providers {
    web {
      cors = "*"
      http {
        enabled = true
        bind_address = "0.0.0.0"
        port = 4444
        hosts = ["*"]
      }
      ws {
        enabled = true
        bind_address = "0.0.0.0"
        port = 4445
      }
    }
  }
  modules = [
    { name: "eth", version: "1.0", enabled: true },
    { name: "net", version: "1.0", enabled: true },
    { name: "web3", version: "1.0", enabled: true },
    { name: "rsk", version: "1.0", enabled: true }
  ]
}

peer {
  port = 5050
  discovery.enabled = true
}

metrics {
  enabled = true
  server {
    bind_address = "0.0.0.0"
    port = 6060
  }
}
`
