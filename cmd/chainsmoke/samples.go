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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// sampleGlob matches the per-chain ChainInstance samples.
const sampleGlob = "chains_v1alpha2_chaininstance_*.yaml"

// sample is one decoded config/samples ChainInstance. A sample that fails to
// decode keeps its error so the report can show it as FAIL.
type sample struct {
	path  string
	chain chainsv1alpha2.Chain
	node  *chainsv1alpha2.ChainInstance
	err   error
}

// loadSamples decodes every per-chain sample in dir.
func loadSamples(dir string) ([]sample, error) {
	paths, err := filepath.Glob(filepath.Join(dir, sampleGlob))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no %s files in %s", sampleGlob, dir)
	}
	out := make([]sample, 0, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		s := sample{path: p, chain: chainFromSamplePath(p)}
		s.node, s.err = decodeSample(data)
		if s.err == nil {
			s.chain = s.node.Spec.Chain
		}
		out = append(out, s)
	}
	return out, nil
}

// chainFromSamplePath guesses the chain of an undecodable sample from its
// file name: chains_v1alpha2_chaininstance_boba_eth.yaml -> boba-eth.
func chainFromSamplePath(p string) chainsv1alpha2.Chain {
	name := strings.TrimSuffix(filepath.Base(p), ".yaml")
	name = strings.TrimPrefix(name, "chains_v1alpha2_chaininstance_")
	return chainsv1alpha2.Chain(strings.ReplaceAll(name, "_", "-"))
}

// decodeSample strictly decodes a ChainInstance manifest, so a sample field
// the API type does not know is reported instead of silently dropped.
func decodeSample(data []byte) (*chainsv1alpha2.ChainInstance, error) {
	var node chainsv1alpha2.ChainInstance
	if err := yaml.UnmarshalStrict(data, &node); err != nil {
		return nil, err
	}
	if node.Kind != "ChainInstance" {
		return nil, fmt.Errorf("kind %q, want ChainInstance", node.Kind)
	}
	if node.Spec.Chain == "" {
		return nil, fmt.Errorf("spec.chain is empty")
	}
	if node.Namespace == "" {
		node.Namespace = smokeNamespace
	}
	return &node, nil
}
