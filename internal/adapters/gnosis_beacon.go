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

const defaultGnosisBeaconImage = "sigp/lighthouse:v6.0.1"

// --------------------------------------------------------------------------
// Type
// --------------------------------------------------------------------------

type gnosisBeaconAdapter struct {
	baseAdapter
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

func init() {
	Register(nodesv1alpha1.ChainGnosisBeacon, &gnosisBeaconAdapter{
		baseAdapter: baseAdapter{livenessPort: 5052},
	})
}

// --------------------------------------------------------------------------
// Interface methods
// --------------------------------------------------------------------------

func (a *gnosisBeaconAdapter) DefaultImage(_ string) string {
	return defaultGnosisBeaconImage
}

func (a *gnosisBeaconAdapter) ConfigTemplate(_ nodesv1alpha1.BlockchainNodeSpec) (string, string, error) {
	return "lighthouse.toml", gnosisBeaconConfig, nil
}

func (a *gnosisBeaconAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return beaconHealthCheck(ctx, rpcURL)
}

func (a *gnosisBeaconAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(5052, 300, 30, 10, 5)
}

func (a *gnosisBeaconAdapter) ContainerPorts(_ nodesv1alpha1.BlockchainNodeSpec) []corev1.ContainerPort {
	return beaconPorts()
}

func (a *gnosisBeaconAdapter) DefaultResources() ResourceDefaults {
	return ResourceDefaults{
		CPURequest:    resource.MustParse("4"),
		MemoryRequest: resource.MustParse("8Gi"),
		Storage:       resource.MustParse("500Gi"),
	}
}

func (a *gnosisBeaconAdapter) VersionPolicy() ChainVersionPolicy {
	return ChainVersionPolicy{
		Registry:   "docker.io",
		Repository: "sigp/lighthouse",
		TagPattern: `^v\d+\.\d+\.\d+$`,
	}
}

// --------------------------------------------------------------------------
// Config (Lighthouse TOML for Gnosis network)
// --------------------------------------------------------------------------

const gnosisBeaconConfig = `# Lighthouse beacon node config for Gnosis Chain
network = "gnosis"
datadir = "/data"
http = true
http-address = "0.0.0.0"
http-port = 5052
metrics = true
metrics-address = "0.0.0.0"
metrics-port = 5054
port = 9000
discovery-port = 9000
`
