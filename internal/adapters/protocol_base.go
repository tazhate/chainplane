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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// protocolAdapter provides default implementations for NodeSelector and LivenessProbe
// that most chain adapters share. Embed it into concrete adapter types to avoid
// repetition; override methods when chain-specific behavior is needed.
type protocolAdapter struct {
	livenessPort int32
}

// NodeSelector delegates to the package-level DefaultNodeSelector.
func (b protocolAdapter) NodeSelector(nodeGroup chainsv1alpha2.NodeGroup) map[string]string {
	return DefaultNodeSelector(nodeGroup)
}

// LivenessProbe returns a TCP probe on the adapter's configured liveness port.
func (b protocolAdapter) LivenessProbe(_ chainsv1alpha2.ChainInstanceSpec) *corev1.Probe {
	return tcpProbe(b.livenessPort, 30, 30, 10, 5)
}

// --------------------------------------------------------------------------
// Probe constructors
// --------------------------------------------------------------------------

// tcpProbe builds a TCPSocket probe with the given parameters.
func tcpProbe(port, initialDelay, period, timeout, failureThreshold int32) *corev1.Probe { //nolint:unparam
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
func httpProbe(path string, port, initialDelay, period, timeout, failureThreshold int32) *corev1.Probe { //nolint:unparam
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
