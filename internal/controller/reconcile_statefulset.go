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
/*
Copyright 2026.

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

package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

const (
	// mainContainerName is the canonical name of the primary blockchain node
	// container inside every pod.
	mainContainerName = "node"

	// dataVolumeName is the PVC-backed volume that stores chain data.
	dataVolumeName = "data"
	// dataVolumeMountPath is where the data volume is mounted inside the container.
	dataVolumeMountPath = "/data"

	// configVolumeName is the ConfigMap-projected volume for chain configuration.
	configVolumeName = "config"
	// configVolumeMountPath is where the config volume is mounted (read-only).
	configVolumeMountPath = "/config"

	// configHashAnnotation triggers a rolling restart when the config content changes.
	configHashAnnotation = "chains.chainplane.io/config-hash"

	// podFSGroup is the supplemental group applied to the pod security context
	// so mounted volumes are group-writable by the node process.
	podFSGroup = int64(1000)
)

// ensureStatefulSet creates or updates the StatefulSet that runs the blockchain
// node pod(s). The pod template includes the main container, any init
// containers (snapshot bootstrap + adapter-specific), sidecars, volumes, and
// probes resolved from the adapter.
func (r *ChainInstanceReconciler) ensureStatefulSet(
	ctx context.Context,
	node *chainsv1alpha2.ChainInstance,
	adapter adapters.ChainAdapter,
	cfgHash string,
) error {
	desired := r.buildStatefulSetSpec(node, adapter, cfgHash)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: node.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		// Preserve pod-template annotations set by reconcileUpgrade (e.g. restarted-at).
		// desired.Template.Annotations only carries config-hash; don't clobber the rest.
		existingTemplateAnns := sts.Spec.Template.Annotations

		sts.Labels = coreLabels(node)
		sts.Spec = desired

		if sts.Spec.Template.Annotations == nil {
			sts.Spec.Template.Annotations = map[string]string{}
		}
		for k, v := range existingTemplateAnns {
			if _, ok := sts.Spec.Template.Annotations[k]; !ok {
				sts.Spec.Template.Annotations[k] = v
			}
		}
		return controllerutil.SetControllerReference(node, sts, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("upserting StatefulSet for %s/%s: %w", node.Namespace, node.Name, err)
	}
	return nil
}

// buildStatefulSetSpec assembles the full StatefulSetSpec from the node CR and
// adapter. Keeping this in a pure function simplifies testing and readability.
func (r *ChainInstanceReconciler) buildStatefulSetSpec(
	node *chainsv1alpha2.ChainInstance,
	adapter adapters.ChainAdapter,
	cfgHash string,
) appsv1.StatefulSetSpec {
	replicas := int32(1)
	if node.Spec.Replicas != nil {
		replicas = *node.Spec.Replicas
	}

	image := resolveContainerImage(node, adapter)
	ports := adapter.ContainerPorts(node.Spec)
	liveness := adapter.LivenessProbe(node.Spec)

	var startup *corev1.Probe
	if sp, ok := adapter.(adapters.StartupProbeProvider); ok {
		startup = sp.StartupProbe(node.Spec)
	}

	initContainers := r.snapshotInitContainers(node)
	if icp, ok := adapter.(adapters.InitContainerProvider); ok {
		initContainers = append(initContainers, icp.InitContainers(node.Spec)...)
	}

	var storageClassName *string
	if sc := node.Spec.Storage.StorageClass; sc != "" {
		storageClassName = &sc
	}
	volumeMode := corev1.PersistentVolumeFilesystem
	fsGroup := podFSGroup

	return appsv1.StatefulSetSpec{
		Replicas:    &replicas,
		ServiceName: node.Name,
		Selector: &metav1.LabelSelector{
			MatchLabels: selectorLabels(node),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      coreLabels(node),
				Annotations: map[string]string{configHashAnnotation: cfgHash},
			},
			Spec: corev1.PodSpec{
				SecurityContext: &corev1.PodSecurityContext{
					FSGroup: &fsGroup,
				},
				NodeSelector:   adapter.NodeSelector(node.Spec.NodeGroup),
				InitContainers: initContainers,
				Containers:     r.podContainers(node, image, ports, liveness, startup, adapter),
				Volumes:        r.podVolumes(node),
			},
		},
		VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{Name: dataVolumeName},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					StorageClassName: storageClassName,
					VolumeMode:       &volumeMode,
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: node.Spec.Storage.Size,
						},
					},
				},
			},
		},
	}
}

// podContainers returns the ordered slice of containers: the main node
// container followed by any user-defined sidecars.
func (r *ChainInstanceReconciler) podContainers(
	node *chainsv1alpha2.ChainInstance,
	image string,
	ports []corev1.ContainerPort,
	liveness, startup *corev1.Probe,
	adapter adapters.ChainAdapter,
) []corev1.Container {
	mounts := make([]corev1.VolumeMount, 0, 2+len(node.Spec.ExtraVolumeMounts))
	mounts = append(mounts,
		corev1.VolumeMount{Name: dataVolumeName, MountPath: dataVolumeMountPath},
		corev1.VolumeMount{Name: configVolumeName, MountPath: configVolumeMountPath, ReadOnly: true},
	)
	mounts = append(mounts, node.Spec.ExtraVolumeMounts...)

	main := corev1.Container{
		Name:            mainContainerName,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         containerCommand(adapter, node),
		Args:            containerArgs(adapter, node),
		Ports:           ports,
		Resources:       node.Spec.Resources,
		LivenessProbe:   liveness,
		StartupProbe:    startup,
		Env:             containerEnv(node),
		VolumeMounts:    mounts,
	}

	containers := make([]corev1.Container, 0, 1+len(node.Spec.Sidecars))
	containers = append(containers, main)
	if sp, ok := adapter.(adapters.SidecarProvider); ok {
		containers = append(containers, sp.Sidecars(node.Spec)...)
	}
	containers = append(containers, node.Spec.Sidecars...)
	return containers
}

// podVolumes returns the ordered slice of pod volumes: the config ConfigMap
// volume followed by any user-defined extra volumes. The "data" volume is
// provided by VolumeClaimTemplates and must not appear here.
func (r *ChainInstanceReconciler) podVolumes(node *chainsv1alpha2.ChainInstance) []corev1.Volume {
	vols := make([]corev1.Volume, 0, 1+len(node.Spec.ExtraVolumes))
	vols = append(vols, corev1.Volume{
		Name: configVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: node.Name + "-config",
				},
			},
		},
	})
	return append(vols, node.Spec.ExtraVolumes...)
}
