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

package v1alpha2

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var webhookLog = logf.Log.WithName("chaininstance-webhook")

// ---------------------------------------------------------------------------
// Validation registry — per-chain resource recommendations
// ---------------------------------------------------------------------------

// chainConstraints holds per-chain recommended resource minimums.
// Violations are surfaced as admission warnings, not hard errors,
// so that development/test workloads are not blocked.
type chainConstraints struct {
	// MinStorage is the recommended minimum PVC size for this chain.
	MinStorage resource.Quantity
	// MinMemory is the recommended minimum memory (zero means no recommendation).
	MinMemory resource.Quantity
}

// validationRegistry maps every supported chain to its resource recommendations.
// Chains absent from this map are rejected by the webhook (hard error).
var validationRegistry = map[Chain]chainConstraints{
	ChainEthereum:        {MinStorage: resource.MustParse("50Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainEthereumArchive: {MinStorage: resource.MustParse("2Ti"), MinMemory: resource.MustParse("8Gi")},
	ChainBitcoin:         {MinStorage: resource.MustParse("500Gi")},
	ChainSolana:          {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("64Gi")},
	ChainBSC:             {MinStorage: resource.MustParse("1Ti"), MinMemory: resource.MustParse("16Gi")},
	ChainTRON:            {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("16Gi")},
	ChainPolygon:         {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainAvalanche:       {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainLitecoin:        {MinStorage: resource.MustParse("100Gi")},
	ChainXRP:             {MinStorage: resource.MustParse("100Gi")},
	ChainStellar:         {MinStorage: resource.MustParse("100Gi")},
	ChainDash:            {MinStorage: resource.MustParse("50Gi")},
	ChainTON:             {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainCosmos:          {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainNear:            {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainSui:             {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainAptos:           {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainCardano:         {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainArbitrum:        {MinStorage: resource.MustParse("2Ti"), MinMemory: resource.MustParse("16Gi")},
	ChainOptimism:        {MinStorage: resource.MustParse("1Ti"), MinMemory: resource.MustParse("16Gi")},
	ChainBase:            {MinStorage: resource.MustParse("2Ti"), MinMemory: resource.MustParse("16Gi")},
	ChainBerachain:       {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainCronos:          {MinStorage: resource.MustParse("1Ti"), MinMemory: resource.MustParse("8Gi")},
	ChainRonin:           {MinStorage: resource.MustParse("1Ti"), MinMemory: resource.MustParse("8Gi")},
	ChainCelo:            {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainFantom:          {MinStorage: resource.MustParse("2Ti"), MinMemory: resource.MustParse("16Gi")},
	ChainGnosis:          {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainMantle:          {MinStorage: resource.MustParse("1Ti"), MinMemory: resource.MustParse("16Gi")},
	ChainBlast:           {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainMode:            {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainZora:            {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainTaiko:           {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainZkSync:          {MinStorage: resource.MustParse("1Ti"), MinMemory: resource.MustParse("16Gi")},
	ChainLinea:           {MinStorage: resource.MustParse("1Ti"), MinMemory: resource.MustParse("8Gi")},
	ChainScroll:          {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainDogecoin:        {MinStorage: resource.MustParse("100Gi")},
	ChainOsmosis:         {MinStorage: resource.MustParse("1Ti"), MinMemory: resource.MustParse("8Gi")},
	ChainSei:             {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainEvmos:           {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainKava:            {MinStorage: resource.MustParse("1Ti"), MinMemory: resource.MustParse("8Gi")},
	ChainPolkadot:        {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainStarknet:        {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainFilecoin:        {MinStorage: resource.MustParse("2Ti"), MinMemory: resource.MustParse("32Gi")},
	ChainMoonbeam:        {MinStorage: resource.MustParse("2000Gi"), MinMemory: resource.MustParse("16Gi")},
	ChainMoonriver:       {MinStorage: resource.MustParse("1000Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainPolygonZkEVM:    {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainMantaPacific:    {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainMetis:           {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainFraxtal:         {MinStorage: resource.MustParse("300Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainLisk:            {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainKroma:           {MinStorage: resource.MustParse("300Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainBob:             {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainBobaEth:         {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainSoneium:         {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainSwell:           {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainSuperseed:       {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainInk:             {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainMorph:           {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainAbstract:        {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainMegaETH:         {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainZeroNetwork:     {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainZircuit:         {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainImmutableZkEVM:  {MinStorage: resource.MustParse("300Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainWorldchain:      {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainUnichain:        {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainLens:            {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainPlume:           {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainHemi:            {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainEthereumBeacon:  {MinStorage: resource.MustParse("2000Gi"), MinMemory: resource.MustParse("16Gi")},
	ChainGnosisBeacon:    {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainAxelar:          {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainDymension:       {MinStorage: resource.MustParse("300Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainAurora:          {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainHarmony:         {MinStorage: resource.MustParse("2000Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainRootstock:       {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainTelos:           {MinStorage: resource.MustParse("300Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainKlaytn:          {MinStorage: resource.MustParse("2000Gi"), MinMemory: resource.MustParse("16Gi")},
	ChainShibarium:       {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainCore:            {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainHaqq:            {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainHashKey:         {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainEthereumClassic: {MinStorage: resource.MustParse("1000Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainBitTorrent:      {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainGravityAlpha:    {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainMoca:            {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainEverclear:       {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainDoma:            {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainCronosZkEVM:     {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainSonic:           {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainGoat:            {MinStorage: resource.MustParse("200Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainKatana:          {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainMezo:            {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainPlasma:          {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainPlaynance:       {MinStorage: resource.MustParse("100Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainKusama:          {MinStorage: resource.MustParse("2000Gi"), MinMemory: resource.MustParse("16Gi")},
	ChainHyperliquid:     {MinStorage: resource.MustParse("2000Gi"), MinMemory: resource.MustParse("32Gi")},
	ChainMonad:           {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("16Gi")},
	ChainOpBNB:           {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainFuse:            {MinStorage: resource.MustParse("300Gi"), MinMemory: resource.MustParse("4Gi")},
	ChainThundercore:     {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainWemix:           {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
	ChainViction:         {MinStorage: resource.MustParse("500Gi"), MinMemory: resource.MustParse("8Gi")},
}

// supportedNetworks lists all valid network values.
var supportedNetworks = map[Network]bool{
	NetworkMainnet: true,
	NetworkTestnet: true,
	NetworkDevnet:  true,
}

// supportedChains is derived from validationRegistry for backward-compatible lookups.
var supportedChains = func() map[Chain]bool {
	m := make(map[Chain]bool, len(validationRegistry))
	for c := range validationRegistry {
		m[c] = true
	}
	return m
}()

// ---------------------------------------------------------------------------
// Webhook setup
// ---------------------------------------------------------------------------

// SetupWebhookWithManager registers the validating webhook with the controller manager.
func (r *ChainInstance) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&ChainInstanceValidator{}).
		Complete()
}

// ChainInstanceValidator implements webhook.CustomValidator for ChainInstance.
// +kubebuilder:webhook:path=/validate-chains-chainplane-io-v1alpha2-chaininstance,mutating=false,failurePolicy=fail,sideEffects=None,groups=chains.chainplane.io,resources=chaininstances,verbs=create;update,versions=v1alpha2,name=vchaininstance.kb.io,admissionReviewVersions=v1
type ChainInstanceValidator struct{}

var _ webhook.CustomValidator = &ChainInstanceValidator{}

// ValidateCreate validates a new ChainInstance resource.
func (v *ChainInstanceValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	node, ok := obj.(*ChainInstance)
	if !ok {
		return nil, fmt.Errorf("expected a ChainInstance object but got %T", obj)
	}
	webhookLog.Info("validate create", "name", node.Name)
	warnings, err := validateChainInstanceSpec(node)
	return warnings, err
}

// ValidateUpdate validates an update to an existing ChainInstance resource.
// Chain and network fields are immutable.
func (v *ChainInstanceValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldNode, ok := oldObj.(*ChainInstance)
	if !ok {
		return nil, fmt.Errorf("expected a ChainInstance object but got %T", oldObj)
	}
	newNode, ok := newObj.(*ChainInstance)
	if !ok {
		return nil, fmt.Errorf("expected a ChainInstance object but got %T", newObj)
	}
	webhookLog.Info("validate update", "name", newNode.Name)

	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	// Immutability: chain cannot change after creation.
	if oldNode.Spec.Chain != newNode.Spec.Chain {
		allErrs = append(allErrs, field.Forbidden(
			specPath.Child("chain"),
			"chain is immutable and cannot be changed after creation",
		))
	}

	// Immutability: network cannot change after creation.
	if oldNode.Spec.Network != newNode.Spec.Network {
		allErrs = append(allErrs, field.Forbidden(
			specPath.Child("network"),
			"network is immutable and cannot be changed after creation",
		))
	}

	// Validate the new spec fields.
	warnings, specErr := validateChainInstanceSpec(newNode)
	if specErr != nil {
		return warnings, specErr
	}

	if len(allErrs) > 0 {
		return warnings, allErrs.ToAggregate()
	}
	return warnings, nil
}

// ValidateDelete validates deletion of a ChainInstance (always permitted).
func (v *ChainInstanceValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	node, ok := obj.(*ChainInstance)
	if !ok {
		return nil, fmt.Errorf("expected a ChainInstance object but got %T", obj)
	}
	webhookLog.Info("validate delete", "name", node.Name)
	return nil, nil
}

// ---------------------------------------------------------------------------
// Spec validation
// ---------------------------------------------------------------------------

// validateChainInstanceSpec performs field-level validation on the ChainInstance spec.
// Hard constraint violations are returned as errors; resource recommendations
// that are below the per-chain minimum are returned as warnings.
func validateChainInstanceSpec(node *ChainInstance) (admission.Warnings, error) {
	var (
		allErrs  field.ErrorList
		warnings admission.Warnings
	)
	specPath := field.NewPath("spec")

	// --- chain ---
	constraints, chainSupported := validationRegistry[node.Spec.Chain]
	if !chainSupported {
		allErrs = append(allErrs, field.NotSupported(
			specPath.Child("chain"),
			node.Spec.Chain,
			supportedChainNames(),
		))
	}

	// --- network ---
	if !supportedNetworks[node.Spec.Network] {
		allErrs = append(allErrs, field.NotSupported(
			specPath.Child("network"),
			node.Spec.Network,
			[]string{string(NetworkMainnet), string(NetworkTestnet), string(NetworkDevnet)},
		))
	}

	// --- storage.size > 0 (hard error) ---
	zero := resource.MustParse("0")
	if node.Spec.Storage.Size.Cmp(zero) <= 0 {
		allErrs = append(allErrs, field.Invalid(
			specPath.Child("storage").Child("size"),
			node.Spec.Storage.Size.String(),
			"storage size must be greater than 0",
		))
	}

	// --- per-chain minimum storage (warning) ---
	if chainSupported && node.Spec.Storage.Size.Cmp(zero) > 0 &&
		node.Spec.Storage.Size.Cmp(constraints.MinStorage) < 0 {
		warnings = append(warnings, fmt.Sprintf(
			"spec.storage.size %s is below the recommended minimum of %s for chain %q",
			node.Spec.Storage.Size.String(), constraints.MinStorage.String(), node.Spec.Chain,
		))
	}

	// --- per-chain minimum memory (warning) ---
	if chainSupported && !constraints.MinMemory.IsZero() {
		limMem := node.Spec.Resources.Limits.Memory()
		reqMem := node.Spec.Resources.Requests.Memory()

		if limMem != nil && !limMem.IsZero() && limMem.Cmp(constraints.MinMemory) < 0 {
			warnings = append(warnings, fmt.Sprintf(
				"spec.resources.limits.memory %s is below the recommended minimum of %s for chain %q",
				limMem.String(), constraints.MinMemory.String(), node.Spec.Chain,
			))
		}
		if reqMem != nil && !reqMem.IsZero() && reqMem.Cmp(constraints.MinMemory) < 0 {
			warnings = append(warnings, fmt.Sprintf(
				"spec.resources.requests.memory %s is below the recommended minimum of %s for chain %q",
				reqMem.String(), constraints.MinMemory.String(), node.Spec.Chain,
			))
		}
	}

	// Replicas=0 is explicitly allowed to pause a node.

	if len(allErrs) > 0 {
		return warnings, allErrs.ToAggregate()
	}
	return warnings, nil
}

// supportedChainNames returns a sorted list of supported chain names for error messages.
func supportedChainNames() []string {
	return []string{
		string(ChainEthereum),
		string(ChainEthereumArchive),
		string(ChainBitcoin),
		string(ChainSolana),
		string(ChainBSC),
		string(ChainTRON),
		string(ChainPolygon),
		string(ChainAvalanche),
		string(ChainLitecoin),
		string(ChainXRP),
		string(ChainStellar),
		string(ChainDash),
		string(ChainTON),
		string(ChainCosmos),
		string(ChainNear),
		string(ChainSui),
		string(ChainAptos),
		string(ChainCardano),
		string(ChainArbitrum),
		string(ChainOptimism),
		string(ChainBase),
		string(ChainBerachain),
		string(ChainCronos),
		string(ChainRonin),
		string(ChainCelo),
		string(ChainFantom),
		string(ChainGnosis),
		string(ChainMantle),
		string(ChainBlast),
		string(ChainMode),
		string(ChainZora),
		string(ChainTaiko),
		string(ChainZkSync),
		string(ChainLinea),
		string(ChainScroll),
		string(ChainDogecoin),
		string(ChainOsmosis),
		string(ChainSei),
		string(ChainEvmos),
		string(ChainKava),
		string(ChainPolkadot),
		string(ChainStarknet),
		string(ChainFilecoin),
		string(ChainFraxtal),
		string(ChainLisk),
		string(ChainKroma),
		string(ChainBob),
		string(ChainBobaEth),
		string(ChainSoneium),
		string(ChainSwell),
		string(ChainSuperseed),
		string(ChainInk),
		string(ChainMorph),
		string(ChainAbstract),
		string(ChainMegaETH),
		string(ChainZeroNetwork),
		string(ChainZircuit),
		string(ChainImmutableZkEVM),
		string(ChainWorldchain),
		string(ChainUnichain),
		string(ChainLens),
		string(ChainPlume),
		string(ChainHemi),
		string(ChainAxelar),
		string(ChainDymension),
		string(ChainAurora),
		string(ChainHarmony),
		string(ChainRootstock),
		string(ChainTelos),
		string(ChainKlaytn),
		string(ChainShibarium),
		string(ChainCore),
		string(ChainHaqq),
		string(ChainHashKey),
		string(ChainEthereumClassic),
		string(ChainBitTorrent),
		string(ChainGravityAlpha),
		string(ChainMoca),
		string(ChainEverclear),
		string(ChainDoma),
		string(ChainCronosZkEVM),
		string(ChainSonic),
		string(ChainGoat),
		string(ChainKatana),
		string(ChainMezo),
		string(ChainPlasma),
		string(ChainPlaynance),
		string(ChainKusama),
		string(ChainHyperliquid),
		string(ChainMonad),
		string(ChainEthereumBeacon),
		string(ChainGnosisBeacon),
		string(ChainOpBNB),
		string(ChainFuse),
		string(ChainThundercore),
		string(ChainWemix),
		string(ChainViction),
	}
}
