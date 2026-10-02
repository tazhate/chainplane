/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
	"github.com/tazhate/chainplane/internal/controller"
)

func TestParseChainID(t *testing.T) {
	tests := []struct {
		body    string
		want    uint64
		wantErr string
	}{
		{`{"jsonrpc":"2.0","id":1,"result":"0x1"}`, 1, ""},
		{`{"jsonrpc":"2.0","id":1,"result":"0x76adf1"}`, 7777777, ""},
		{`{"jsonrpc":"2.0","id":1,"result":"0x63564c40"}`, 1666600000, ""},
		{`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`, 0, "method not found"},
		{`{"jsonrpc":"2.0","id":1,"result":"1"}`, 0, "not a hex quantity"},
		{`{"jsonrpc":"2.0","id":1,"result":"0x"}`, 0, "not a hex quantity"},
		{`{"jsonrpc":"2.0","id":1,"result":"0xzz"}`, 0, "invalid syntax"},
		{`404 page not found`, 0, "not JSON-RPC"},
	}
	for _, tt := range tests {
		got, err := parseChainID(tt.body)
		switch {
		case tt.wantErr == "" && err != nil:
			t.Errorf("parseChainID(%s): %v", tt.body, err)
		case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
			t.Errorf("parseChainID(%s) error = %v, want %q", tt.body, err, tt.wantErr)
		case got != tt.want:
			t.Errorf("parseChainID(%s) = %d, want %d", tt.body, got, tt.want)
		}
	}
}

func TestParseCometBFTNetwork(t *testing.T) {
	for body, want := range map[string]string{
		`{"jsonrpc":"2.0","id":-1,"result":{"node_info":{"network":"cosmoshub-4"}}}`: "cosmoshub-4",
		`{"node_info":{"network":"osmosis-1"}}`:                                      "osmosis-1",
	} {
		if got, err := parseCometBFTNetwork(body); err != nil || got != want {
			t.Errorf("parseCometBFTNetwork(%s) = %q, %v, want %q", body, got, err, want)
		}
	}
	for _, body := range []string{`{"result":{}}`, `<html>`} {
		if got, err := parseCometBFTNetwork(body); err == nil {
			t.Errorf("parseCometBFTNetwork(%s) = %q, want error", body, got)
		}
	}
}

func TestParseDu(t *testing.T) {
	if got, err := parseDu("52344\t/proc/1/root/data\n"); err != nil || got != 52344 {
		t.Errorf("parseDu = %d, %v", got, err)
	}
	for _, out := range []string{"", "du: /proc/1/root/data: Permission denied"} {
		if _, err := parseDu(out); err == nil {
			t.Errorf("parseDu(%q) accepted", out)
		}
	}
}

func TestFindings(t *testing.T) {
	if fail, _ := dataFinding("/data", 0); fail != "node wrote nothing to /data (0 KiB): config/datadir not applied" {
		t.Errorf("empty data: fail = %q", fail)
	}
	if fail, _ := dataFinding("/data", minDataKiB-1); fail == "" {
		t.Error("data below minDataKiB passed")
	}
	if fail, fact := dataFinding("/data", 52344); fail != "" || fact != "/data 51 MiB" {
		t.Errorf("data present: %q, %q", fail, fact)
	}
	if fail, _ := chainIDFinding(1, 7777777); fail != "chain id 1, expected 7777777" {
		t.Errorf("chain id mismatch: fail = %q", fail)
	}
	if fail, fact := chainIDFinding(1, 1); fail != "" || fact != "chain id 1" {
		t.Errorf("chain id match: %q, %q", fail, fact)
	}
	if fail, _ := networkFinding("cosmoshub-4", "osmosis-1"); fail != "network cosmoshub-4, expected osmosis-1" {
		t.Errorf("network mismatch: fail = %q", fail)
	}
}

func TestApplyIdentity(t *testing.T) {
	const running = "running after 1m30s"
	tests := []struct {
		name       string
		status     string
		detail     string
		f          identityFindings
		wantStatus string
		wantDetail string
		wantNotes  []string
	}{
		{
			name:   "verified pass shows facts",
			status: statusPass, detail: running,
			f:          identityFindings{facts: []string{"/data 51 MiB", "chain id 1"}},
			wantStatus: statusPass, wantDetail: running + "; /data 51 MiB; chain id 1",
		},
		{
			// The zora case: up for the whole window, as Ethereum mainnet.
			name:   "wrong chain and no data fail a clean pass",
			status: statusPass, detail: running,
			f: identityFindings{fails: []string{
				"node wrote nothing to /data (0 KiB): config/datadir not applied",
				"chain id 1, expected 7777777",
			}},
			wantStatus: statusFail,
			wantDetail: "node wrote nothing to /data (0 KiB): config/datadir not applied; " +
				"chain id 1, expected 7777777; " + running,
		},
		{
			name:   "fail overrides warn, warns and facts become notes",
			status: statusWarn, detail: "3 error line(s), first: x",
			f: identityFindings{
				fails: []string{"chain id 1, expected 10"},
				warns: []string{"cometbft rpc not ready: refused"},
				facts: []string{"/data 2 MiB"},
			},
			wantStatus: statusFail, wantDetail: "chain id 1, expected 10; 3 error line(s), first: x",
			wantNotes: []string{"cometbft rpc not ready: refused", "/data 2 MiB"},
		},
		{
			name:   "rpc not ready downgrades a pass",
			status: statusPass, detail: running,
			f:          identityFindings{warns: []string{"rpc not ready: Connection refused"}, facts: []string{"/data 2 MiB"}},
			wantStatus: statusWarn, wantDetail: "rpc not ready: Connection refused; " + running,
			wantNotes: []string{"/data 2 MiB"},
		},
		{
			name:   "rpc not ready does not hide a log fail",
			status: statusFail, detail: "unknown flag: --x",
			f:          identityFindings{warns: []string{"rpc not ready: refused"}, notes: []string{"data check: denied"}},
			wantStatus: statusFail, wantDetail: "unknown flag: --x",
			wantNotes: []string{"data check: denied", "rpc not ready: refused"},
		},
		{
			name:   "nothing checked",
			status: statusPass, detail: running,
			wantStatus: statusPass, wantDetail: running,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, detail, notes := applyIdentity(tt.status, tt.detail, tt.f)
			if status != tt.wantStatus || detail != tt.wantDetail || !slices.Equal(notes, tt.wantNotes) {
				t.Errorf("applyIdentity = %s %q %q, want %s %q %q",
					status, detail, notes, tt.wantStatus, tt.wantDetail, tt.wantNotes)
			}
		})
	}
}

func TestEVMRPCPort(t *testing.T) {
	tcp := corev1.ProtocolTCP
	tests := []struct {
		ports []corev1.ContainerPort
		want  int32
	}{
		{[]corev1.ContainerPort{
			{Name: "rpc", ContainerPort: 26657, Protocol: tcp},
			{Name: "evm-rpc", ContainerPort: 8545, Protocol: tcp},
		}, 8545},
		{[]corev1.ContainerPort{
			{Name: "p2p", ContainerPort: 30303},
			{Name: "rpc", ContainerPort: 8547, Protocol: tcp},
		}, 8547},
		{[]corev1.ContainerPort{{Name: "http", ContainerPort: 8588, Protocol: tcp}}, 8588},
		{[]corev1.ContainerPort{{Name: "p2p", ContainerPort: 30303}}, 0},
	}
	for _, tt := range tests {
		if got, _ := evmRPCPort(tt.ports); got != tt.want {
			t.Errorf("evmRPCPort(%v) = %d, want %d", tt.ports, got, tt.want)
		}
	}
}

// TestIdentityTargetsFromSamples renders real samples: the expectations must
// come out of the pod the operator renders, data mount and RPC port included.
func TestIdentityTargetsFromSamples(t *testing.T) {
	samples, err := loadSamples(samplesDir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[chainsv1alpha2.Chain]identityTarget{
		chainsv1alpha2.ChainZora:      {dataPath: "/data", chainID: 7777777, evmRPC: "http://127.0.0.1:8545/"},
		chainsv1alpha2.ChainAvalanche: {dataPath: "/data", chainID: 43114, evmRPC: "http://127.0.0.1:9650/ext/bc/C/rpc"},
		chainsv1alpha2.ChainMezo:      {dataPath: "/data", chainID: 31612, evmRPC: "http://127.0.0.1:8545/"},
		chainsv1alpha2.ChainCosmos:    {dataPath: "/data", cometBFTNet: "cosmoshub-4"},
		chainsv1alpha2.ChainDash:      {dataPath: "/data"},
	}
	seen := map[chainsv1alpha2.Chain]bool{}
	var evmMissing []string
	for _, s := range samples {
		if s.err != nil {
			continue
		}
		adapter, ok := adapters.Get(s.chain)
		if !ok {
			continue
		}
		pod := controller.RenderPodTemplate(s.node, adapter, "")
		mc := pod.Spec.Containers[slices.IndexFunc(pod.Spec.Containers, func(c corev1.Container) bool {
			return c.Name == controller.MainContainerName
		})]
		got := newIdentityTarget(s.node.Spec, mc, "m", "m", "p")
		if _, known := adapters.ExpectedEVMChainID(s.chain, s.node.Spec.Network); known && got.chainID == 0 {
			evmMissing = append(evmMissing, string(s.chain))
		}
		w, ok := want[s.chain]
		if !ok || seen[s.chain] {
			continue
		}
		seen[s.chain] = true
		got.chain, got.main, got.netOwner, got.prefix = "", "", "", ""
		if got != w {
			t.Errorf("%s: target = %+v, want data %q, chain id %d, rpc %q, network %q",
				s.chain, got, w.dataPath, w.chainID, w.evmRPC, w.cometBFTNet)
		}
	}
	for chain := range want {
		if !seen[chain] {
			t.Errorf("no sample for %s", chain)
		}
	}
	// A chain with a known id but no RPC port would silently skip the check.
	if len(evmMissing) > 0 {
		t.Errorf("chain id known but no evm-rpc/rpc/http port: %s", strings.Join(evmMissing, ", "))
	}
}

func TestParseNetVersion(t *testing.T) {
	tests := []struct {
		body    string
		want    uint64
		wantErr bool
	}{
		{`{"jsonrpc":"2.0","id":1,"result":"88"}`, 88, false},
		{`{"jsonrpc":"2.0","id":1,"result":"0x58"}`, 0, true},
		{`not json`, 0, true},
	}
	for _, tt := range tests {
		got, err := parseNetVersion(tt.body)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("parseNetVersion(%s) = %d, %v; want %d, err=%v", tt.body, got, err, tt.want, tt.wantErr)
		}
	}
}

// fakeRPC answers JSON-RPC methods from a table; a missing method is a
// transport error, like a node whose RPC port is not open yet.
func fakeRPC(results map[string]string) jsonRPC {
	return func(method string) (string, error) {
		body, ok := results[method]
		if !ok {
			return "", errors.New("connection refused")
		}
		return body, nil
	}
}

func rpcResult(result string) string {
	return `{"jsonrpc":"2.0","id":1,"result":"` + result + `"}`
}

func TestCheckEVMChainID(t *testing.T) {
	const rpcError = `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`
	tests := []struct {
		name      string
		want      uint64
		rpc       map[string]string
		wantFail  string
		wantFact  string
		wantWarns []string
	}{
		{
			name: "match", want: 1,
			rpc:      map[string]string{"eth_chainId": rpcResult("0x1")},
			wantFact: "chain id 1",
		},
		{
			name: "mismatch", want: 7777777,
			rpc:      map[string]string{"eth_chainId": rpcResult("0x1")},
			wantFail: "chain id 1, expected 7777777",
		},
		{
			name: "rpc not open", want: 1,
			rpc:       map[string]string{},
			wantWarns: []string{"rpc not ready: connection refused"},
		},
		{
			name: "rpc error response", want: 1,
			rpc:       map[string]string{"eth_chainId": rpcError},
			wantWarns: []string{"rpc not ready: eth_chainId: method not found"},
		},
		{
			name: "pre-EIP155, net_version matches", want: 88,
			rpc:      map[string]string{"eth_chainId": rpcResult("0x0"), "net_version": rpcResult("88")},
			wantFact: "chain id 88 via net_version (eth_chainId=0 before EIP155)",
		},
		{
			name: "pre-EIP155, net_version mismatch", want: 88,
			rpc:      map[string]string{"eth_chainId": rpcResult("0x0"), "net_version": rpcResult("1")},
			wantFail: "chain id 1, expected 88",
		},
		{
			name: "pre-EIP155, net_version missing", want: 88,
			rpc:       map[string]string{"eth_chainId": rpcResult("0x0")},
			wantWarns: []string{"chain id not reported yet (eth_chainId=0)"},
		},
		{
			name: "pre-EIP155, net_version zero", want: 88,
			rpc:       map[string]string{"eth_chainId": rpcResult("0x0"), "net_version": rpcResult("0")},
			wantWarns: []string{"chain id not reported yet (eth_chainId=0)"},
		},
		{
			name: "pre-EIP155, net_version error", want: 88,
			rpc:       map[string]string{"eth_chainId": rpcResult("0x0"), "net_version": rpcError},
			wantWarns: []string{"chain id not reported yet (eth_chainId=0)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f identityFindings
			f.checkEVMChainID(tt.want, fakeRPC(tt.rpc))
			if got := strings.Join(f.fails, "; "); got != tt.wantFail {
				t.Errorf("fails = %q, want %q", got, tt.wantFail)
			}
			if got := strings.Join(f.facts, "; "); got != tt.wantFact {
				t.Errorf("facts = %q, want %q", got, tt.wantFact)
			}
			if !slices.Equal(f.warns, tt.wantWarns) {
				t.Errorf("warns = %q, want %q", f.warns, tt.wantWarns)
			}
		})
	}
}
