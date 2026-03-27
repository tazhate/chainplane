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

package v1alpha1

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newValidNode() *BlockchainNode {
	replicas := int32(1)
	return &BlockchainNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "default",
		},
		Spec: BlockchainNodeSpec{
			Chain:    ChainEthereum,
			Network:  NetworkMainnet,
			Replicas: &replicas,
			Storage: StorageSpec{
				Size: resource.MustParse("100Gi"),
			},
		},
	}
}

func TestValidateCreate_Valid(t *testing.T) {
	v := &BlockchainNodeValidator{}
	node := newValidNode()
	_, err := v.ValidateCreate(context.Background(), node)
	if err != nil {
		t.Errorf("expected no error for valid node, got: %v", err)
	}
}

func TestValidateCreate_InvalidChain(t *testing.T) {
	v := &BlockchainNodeValidator{}
	node := newValidNode()
	node.Spec.Chain = Chain("unsupported-chain")
	_, err := v.ValidateCreate(context.Background(), node)
	if err == nil {
		t.Error("expected error for invalid chain, got nil")
	}
}

func TestValidateCreate_InvalidNetwork(t *testing.T) {
	v := &BlockchainNodeValidator{}
	node := newValidNode()
	node.Spec.Network = Network("stagenet")
	_, err := v.ValidateCreate(context.Background(), node)
	if err == nil {
		t.Error("expected error for invalid network, got nil")
	}
}

func TestValidateCreate_ZeroReplicas(t *testing.T) {
	v := &BlockchainNodeValidator{}
	node := newValidNode()
	zero := int32(0)
	node.Spec.Replicas = &zero
	_, err := v.ValidateCreate(context.Background(), node)
	if err != nil {
		t.Errorf("replicas=0 should be allowed to pause a node, got: %v", err)
	}
}

func TestValidateCreate_InvalidStorage(t *testing.T) {
	v := &BlockchainNodeValidator{}
	node := newValidNode()
	node.Spec.Storage.Size = resource.MustParse("0")
	_, err := v.ValidateCreate(context.Background(), node)
	if err == nil {
		t.Error("expected error for storage size=0, got nil")
	}
}

func TestValidateCreate_AllSupportedChains(t *testing.T) {
	v := &BlockchainNodeValidator{}
	chains := []Chain{
		ChainEthereum, ChainEthereumArchive, ChainBitcoin, ChainSolana,
		ChainBSC, ChainTRON, ChainPolygon, ChainAvalanche,
		ChainLitecoin, ChainXRP, ChainStellar, ChainDash,
		ChainTON, ChainCosmos, ChainNear, ChainSui, ChainAptos, ChainCardano,
	}
	for _, chain := range chains {
		node := newValidNode()
		node.Spec.Chain = chain
		_, err := v.ValidateCreate(context.Background(), node)
		if err != nil {
			t.Errorf("expected no error for chain %s, got: %v", chain, err)
		}
	}
}

func TestValidateCreate_AllSupportedNetworks(t *testing.T) {
	v := &BlockchainNodeValidator{}
	networks := []Network{NetworkMainnet, NetworkTestnet, NetworkDevnet}
	for _, net := range networks {
		node := newValidNode()
		node.Spec.Network = net
		_, err := v.ValidateCreate(context.Background(), node)
		if err != nil {
			t.Errorf("expected no error for network %s, got: %v", net, err)
		}
	}
}

func TestValidateUpdate_ImmutableChain(t *testing.T) {
	v := &BlockchainNodeValidator{}
	oldNode := newValidNode()
	newNode := newValidNode()
	newNode.Spec.Chain = ChainBitcoin // change chain
	_, err := v.ValidateUpdate(context.Background(), oldNode, newNode)
	if err == nil {
		t.Error("expected error when changing immutable chain, got nil")
	}
}

func TestValidateUpdate_ImmutableNetwork(t *testing.T) {
	v := &BlockchainNodeValidator{}
	oldNode := newValidNode()
	newNode := newValidNode()
	newNode.Spec.Network = NetworkTestnet // change network
	_, err := v.ValidateUpdate(context.Background(), oldNode, newNode)
	if err == nil {
		t.Error("expected error when changing immutable network, got nil")
	}
}

func TestValidateUpdate_ValidChange(t *testing.T) {
	v := &BlockchainNodeValidator{}
	oldNode := newValidNode()
	newNode := newValidNode()
	// Change replicas (allowed)
	two := int32(2)
	newNode.Spec.Replicas = &two
	_, err := v.ValidateUpdate(context.Background(), oldNode, newNode)
	if err != nil {
		t.Errorf("expected no error for valid update, got: %v", err)
	}
}

func TestValidateDelete_AlwaysAllowed(t *testing.T) {
	v := &BlockchainNodeValidator{}
	node := newValidNode()
	_, err := v.ValidateDelete(context.Background(), node)
	if err != nil {
		t.Errorf("expected no error for delete, got: %v", err)
	}
}

func TestValidateCreate_TON(t *testing.T) {
	v := &BlockchainNodeValidator{}
	if !supportedChains[ChainTON] {
		t.Fatal("ChainTON should be in supportedChains")
	}
	node := newValidNode()
	node.Spec.Chain = ChainTON
	node.Spec.Storage.Size = resource.MustParse("100Gi")
	_, err := v.ValidateCreate(context.Background(), node)
	if err != nil {
		t.Errorf("expected no error for TON node, got: %v", err)
	}
}

func TestValidateCreate_Cosmos(t *testing.T) {
	v := &BlockchainNodeValidator{}
	if !supportedChains[ChainCosmos] {
		t.Fatal("ChainCosmos should be in supportedChains")
	}
	node := newValidNode()
	node.Spec.Chain = ChainCosmos
	node.Spec.Storage.Size = resource.MustParse("500Gi")
	_, err := v.ValidateCreate(context.Background(), node)
	if err != nil {
		t.Errorf("expected no error for Cosmos node, got: %v", err)
	}
}

func TestValidateCreate_NEAR(t *testing.T) {
	v := &BlockchainNodeValidator{}
	if !supportedChains[ChainNear] {
		t.Fatal("ChainNear should be in supportedChains")
	}
	node := newValidNode()
	node.Spec.Chain = ChainNear
	node.Spec.Storage.Size = resource.MustParse("500Gi")
	_, err := v.ValidateCreate(context.Background(), node)
	if err != nil {
		t.Errorf("expected no error for NEAR node, got: %v", err)
	}
}

func TestValidateCreate_Sui(t *testing.T) {
	v := &BlockchainNodeValidator{}
	if !supportedChains[ChainSui] {
		t.Fatal("ChainSui should be in supportedChains")
	}
	node := newValidNode()
	node.Spec.Chain = ChainSui
	node.Spec.Storage.Size = resource.MustParse("500Gi")
	_, err := v.ValidateCreate(context.Background(), node)
	if err != nil {
		t.Errorf("expected no error for Sui node, got: %v", err)
	}
}

func TestValidateCreate_Aptos(t *testing.T) {
	v := &BlockchainNodeValidator{}
	if !supportedChains[ChainAptos] {
		t.Fatal("ChainAptos should be in supportedChains")
	}
	node := newValidNode()
	node.Spec.Chain = ChainAptos
	node.Spec.Storage.Size = resource.MustParse("500Gi")
	_, err := v.ValidateCreate(context.Background(), node)
	if err != nil {
		t.Errorf("expected no error for Aptos node, got: %v", err)
	}
}

func TestValidateCreate_Cardano(t *testing.T) {
	v := &BlockchainNodeValidator{}
	if !supportedChains[ChainCardano] {
		t.Fatal("ChainCardano should be in supportedChains")
	}
	node := newValidNode()
	node.Spec.Chain = ChainCardano
	node.Spec.Storage.Size = resource.MustParse("100Gi")
	_, err := v.ValidateCreate(context.Background(), node)
	if err != nil {
		t.Errorf("expected no error for Cardano node, got: %v", err)
	}
}
