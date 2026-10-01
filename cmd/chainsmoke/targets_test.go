/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"maps"
	"slices"
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

func TestParseOverride(t *testing.T) {
	tests := []struct {
		in      string
		want    imageOverride
		wantErr bool
	}{
		{in: "bitcoin=lncm/bitcoind:v29.0", want: imageOverride{chain: "bitcoin", ref: "lncm/bitcoind:v29.0"}},
		{in: "ethereum/Geth=geth:v1.18.0", want: imageOverride{chain: "ethereum", client: "geth", ref: "geth:v1.18.0"}},
		{in: "base=gcr.io/x/op-geth@sha256:abc", want: imageOverride{chain: "base", ref: "gcr.io/x/op-geth@sha256:abc"}},
		{in: " dash = dashpay/dashd:24.0.0 ", want: imageOverride{chain: "dash", ref: "dashpay/dashd:24.0.0"}},
		{in: "bitcoin", wantErr: true},
		{in: "=img:1", wantErr: true},
		{in: "bitcoin=", wantErr: true},
		{in: "/geth=img:1", wantErr: true},
	}
	for _, tt := range tests {
		got, err := parseOverride(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseOverride(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseOverride(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestOverrideListFlag(t *testing.T) {
	var l overrideList
	for _, s := range []string{"ethereum=img:all", "ethereum/geth=img:geth", "ethereum=img:all2"} {
		if err := l.Set(s); err != nil {
			t.Fatal(err)
		}
	}
	if got := l.String(); got != "ethereum=img:all,ethereum/geth=img:geth,ethereum=img:all2" {
		t.Errorf("String() = %q", got)
	}
	// The last matching override wins.
	if ref, _ := l.lookup("ethereum", "Geth"); ref != "img:all2" {
		t.Errorf("lookup(ethereum, Geth) = %q", ref)
	}
	if _, ok := l.lookup("bitcoin", ""); ok {
		t.Error("lookup(bitcoin) matched an ethereum override")
	}
}

func TestLevel0Targets(t *testing.T) {
	images := map[chainsv1alpha2.Chain]map[string]string{
		"ethereum": {"": "nethermind:1", "geth": "geth:1"},
		"bitcoin":  {"": "bitcoind:1"},
		"dash":     {"": "dashd:1"},
	}
	overrides := overrideList{
		{chain: "ethereum", client: "geth", ref: "geth:2"},
		{chain: "ethereum", client: "reth", ref: "reth:1"},
		{chain: "bitcoin", ref: "bitcoind:2"},
	}
	selected := func(c chainsv1alpha2.Chain) bool { return c != "dash" }

	got := level0Targets(images, overrides, selected)
	want := []target{
		{chain: "bitcoin", client: "", image: "bitcoind:2"},
		{chain: "ethereum", client: "", image: "nethermind:1"},
		{chain: "ethereum", client: "geth", image: "geth:2"},
		{chain: "ethereum", client: "reth", image: "reth:1"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("level0Targets =\n%+v\nwant\n%+v", got, want)
	}
}

const versionsGenSnippet = `package adapters

var chainDefaultImages = map[chainsv1alpha2.Chain]map[string]string{
	chainsv1alpha2.ChainBitcoin: {
		"": "lncm/bitcoind:v28.0",
	},
	chainsv1alpha2.ChainEthereum: {
		"":     "nethermind/nethermind:1.38.0",
		"geth": "ethereum/client-go:v1.17.1",
	},
}
`

func TestParseImageTable(t *testing.T) {
	got, err := parseImageTable([]byte(versionsGenSnippet))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]string{
		"ChainBitcoin":  {"": "lncm/bitcoind:v28.0"},
		"ChainEthereum": {"": "nethermind/nethermind:1.38.0", "geth": "ethereum/client-go:v1.17.1"},
	}
	if !maps.EqualFunc(got, want, maps.Equal) {
		t.Errorf("parseImageTable = %v, want %v", got, want)
	}

	if _, err := parseImageTable([]byte("package adapters\n")); err == nil {
		t.Error("expected an error when the table is missing")
	}
}

func TestParseChainConsts(t *testing.T) {
	src := `package v1alpha2

type Chain string

const (
	ChainBitcoin    Chain = "bitcoin"
	ChainBitTorrent Chain = "bittorrent"
	NodeTypeRPC           = "rpc"
)
`
	got, err := parseChainConsts([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]chainsv1alpha2.Chain{"ChainBitcoin": "bitcoin", "ChainBitTorrent": "bittorrent"}
	if !maps.Equal(got, want) {
		t.Errorf("parseChainConsts = %v, want %v", got, want)
	}
}

func TestDiffImageTables(t *testing.T) {
	old := map[chainsv1alpha2.Chain]map[string]string{
		"bitcoin":  {"": "bitcoind:1"},
		"ethereum": {"": "nethermind:1", "geth": "geth:1"},
		"dash":     {"": "dashd:1"},
	}
	cur := map[chainsv1alpha2.Chain]map[string]string{
		"bitcoin":  {"": "bitcoind:1"},
		"ethereum": {"": "nethermind:1", "geth": "geth:2"},
		"dash":     {"": "dashd:1", "new": "dash-new:1"},
		"zora":     {"": "zora:1"},
	}
	got := diffImageTables(old, cur)
	want := map[chainsv1alpha2.Chain]bool{"ethereum": true, "dash": true, "zora": true}
	if !maps.Equal(got, want) {
		t.Errorf("diffImageTables = %v, want %v", got, want)
	}
}

func TestParseRealVersionsGen(t *testing.T) {
	// Guards the --changed-since parser against format drift in the
	// generated file and the API constants.
	table, err := parseImageTable(mustRead(t, "../../"+versionsGenPath))
	if err != nil {
		t.Fatal(err)
	}
	consts, err := parseChainConsts(mustRead(t, "../../"+chainTypesPath))
	if err != nil {
		t.Fatal(err)
	}
	old := map[chainsv1alpha2.Chain]map[string]string{}
	for name, clients := range table {
		chain, ok := consts[name]
		if !ok {
			t.Errorf("versions_gen.go key %s has no Chain constant", name)
			continue
		}
		old[chain] = clients
	}
	if changed := diffImageTables(old, adapters.DefaultImages()); len(changed) != 0 {
		t.Errorf("parsed versions_gen.go differs from the compiled table for %v", slices.Sorted(maps.Keys(changed)))
	}
}
