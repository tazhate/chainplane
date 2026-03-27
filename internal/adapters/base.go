package adapters

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// baseAdapter provides default implementations for NodeSelector and LivenessProbe
// that most chain adapters share. Embed it into concrete adapter types to avoid
// repetition; override methods when chain-specific behavior is needed.
type baseAdapter struct {
	livenessPort int32
}

// NodeSelector delegates to the package-level DefaultNodeSelector.
func (b baseAdapter) NodeSelector(nodeGroup nodesv1alpha1.NodeGroup) map[string]string {
	return DefaultNodeSelector(nodeGroup)
}

// LivenessProbe returns a TCP probe on the adapter's configured liveness port.
func (b baseAdapter) LivenessProbe(_ nodesv1alpha1.BlockchainNodeSpec) *corev1.Probe {
	return tcpProbe(b.livenessPort, 30, 30, 10, 5)
}

// --------------------------------------------------------------------------
// Probe constructors
// --------------------------------------------------------------------------

// tcpProbe builds a TCPSocket probe with the given parameters.
func tcpProbe(port, initialDelay, period, timeout, failureThreshold int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(port),
			},
		},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       period,
		TimeoutSeconds:      timeout,
		FailureThreshold:    failureThreshold,
	}
}

// httpProbe builds an HTTPGet probe with the given parameters.
func httpProbe(path string, port, initialDelay, period, timeout, failureThreshold int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt32(port),
			},
		},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       period,
		TimeoutSeconds:      timeout,
		FailureThreshold:    failureThreshold,
	}
}

// syncingPseudo returns a SyncStatus representing an initializing node with a
// time-varying pseudo-block to prevent stall detector from firing.
func syncingPseudo() SyncStatus {
	p := pseudoBlock()
	return SyncStatus{IsSyncing: true, CurrentBlock: p, HighestBlock: p + 1, Progress: 0}
}
