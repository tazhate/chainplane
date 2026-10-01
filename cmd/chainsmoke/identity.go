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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
	"github.com/tazhate/chainplane/internal/controller"
)

// A node can stay up for the whole run window while ignoring the operator:
// an OP Stack image that never reads the mounted config starts as Ethereum
// mainnet and keeps its chain under ~/.ethereum in the container. The
// identity checks below run after the window, while the node still runs,
// and look at what it did rather than at its log.

// minDataKiB is the least a node has to write to the data volume during the
// run window. Every client that reads its datadir from the operator creates
// a database there right at start; an empty directory means it did not.
const minDataKiB = 64

// dataCheckExempt lists chains whose node legitimately writes nothing to the
// data volume in the first minutes, with the reason. Do not add a chain here
// to hide a datadir that is not applied.
var dataCheckExempt = map[chainsv1alpha2.Chain]string{}

// cometBFTRPCPort is the CometBFT RPC port, served on localhost by default.
const cometBFTRPCPort = 26657

const (
	// rpcAttempts and rpcRetryDelay give a node that is still binding its
	// RPC listener a few more seconds after the window.
	rpcAttempts   = 3
	rpcRetryDelay = 2 * time.Second
	// helperTimeout bounds one helper container run.
	helperTimeout = 30 * time.Second
)

// identityTarget is what the identity checks need to know about a run.
type identityTarget struct {
	chain    chainsv1alpha2.Chain
	main     string // main container name
	netOwner string // container owning the pod network namespace
	prefix   string // chainsmoke.prefix label value
	dataPath string // data volume mount path in the main container, "" if none

	chainID     uint64 // expected eth_chainId, 0 skips the check
	evmRPC      string // http://127.0.0.1:<port><path> of the Ethereum JSON-RPC
	cometBFTNet string // expected node_info.network, "" skips the check
}

// newIdentityTarget collects the expectations for the main container of a
// rendered pod.
func newIdentityTarget(
	spec chainsv1alpha2.ChainInstanceSpec, mc corev1.Container, main, netOwner, prefix string,
) identityTarget {
	t := identityTarget{chain: spec.Chain, main: main, netOwner: netOwner, prefix: prefix}
	if i := slices.IndexFunc(mc.VolumeMounts, func(m corev1.VolumeMount) bool {
		return m.Name == controller.DataVolumeName
	}); i >= 0 {
		t.dataPath = mc.VolumeMounts[i].MountPath
	}
	if id, ok := adapters.ExpectedEVMChainID(spec.Chain, spec.Network); ok {
		if port, ok := evmRPCPort(mc.Ports); ok {
			t.chainID = id
			t.evmRPC = fmt.Sprintf("http://127.0.0.1:%d%s", port, adapters.EVMRPCPath(spec.Chain))
		}
	}
	if net, ok := adapters.ExpectedCosmosChainID(spec.Chain, spec.Network); ok {
		t.cometBFTNet = net
	}
	return t
}

// evmRPCPort picks the Ethereum JSON-RPC port among the container ports:
// "evm-rpc" on Cosmos EVM chains, where "rpc" is CometBFT, else "rpc", else
// "http".
func evmRPCPort(ports []corev1.ContainerPort) (int32, bool) {
	for _, name := range []string{"evm-rpc", "rpc", "http"} {
		for _, p := range ports {
			if p.Name == name && p.Protocol != corev1.ProtocolUDP {
				return p.ContainerPort, true
			}
		}
	}
	return 0, false
}

// identityFindings is what the identity checks found.
type identityFindings struct {
	fails []string // the node runs, but not as the chain it was configured for
	warns []string // a check could not get an answer from the node
	facts []string // what was verified, shown with a PASS
	notes []string // checks that could not run at all
}

// checkIdentity runs the identity checks against the running pod.
func checkIdentity(ctx context.Context, t identityTarget) identityFindings {
	var f identityFindings
	switch reason, exempt := dataCheckExempt[t.chain]; {
	case t.dataPath == "":
	case exempt:
		f.notes = append(f.notes, "data check skipped: "+reason)
	default:
		kib, err := dataUsageKiB(ctx, t)
		if err != nil {
			f.notes = append(f.notes, "data check: "+firstLine(err.Error()))
		} else {
			f.add(dataFinding(t.dataPath, kib))
		}
	}

	if t.chainID != 0 {
		body, err := rpcCall(ctx, t, t.evmRPC, `{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`)
		var got uint64
		if err == nil {
			got, err = parseChainID(body)
		}
		if err != nil {
			f.warns = append(f.warns, "rpc not ready: "+firstLine(err.Error()))
		} else {
			f.add(chainIDFinding(got, t.chainID))
		}
	}

	if t.cometBFTNet != "" {
		body, err := rpcCall(ctx, t, fmt.Sprintf("http://127.0.0.1:%d/status", cometBFTRPCPort), "")
		var got string
		if err == nil {
			got, err = parseCometBFTNetwork(body)
		}
		if err != nil {
			f.warns = append(f.warns, "cometbft rpc not ready: "+firstLine(err.Error()))
		} else {
			f.add(networkFinding(got, t.cometBFTNet))
		}
	}
	return f
}

// add records the outcome of one check: a failure or a verified fact.
func (f *identityFindings) add(fail, fact string) {
	if fail != "" {
		f.fails = append(f.fails, fail)
	} else {
		f.facts = append(f.facts, fact)
	}
}

func dataFinding(dataPath string, kib int64) (fail, fact string) {
	if kib < minDataKiB {
		return fmt.Sprintf("node wrote nothing to %s (%d KiB): config/datadir not applied", dataPath, kib), ""
	}
	return "", fmt.Sprintf("%s %s", dataPath, humanKiB(kib))
}

func chainIDFinding(got, want uint64) (fail, fact string) {
	if got != want {
		return fmt.Sprintf("chain id %d, expected %d", got, want), ""
	}
	return "", fmt.Sprintf("chain id %d", got)
}

func networkFinding(got, want string) (fail, fact string) {
	if got != want {
		return fmt.Sprintf("network %s, expected %s", got, want), ""
	}
	return "", "network " + got
}

// applyIdentity folds the identity findings into the verdict of the run.
// A wrong identity is a FAIL whatever the log says; a node that does not
// answer is a WARN, since many need longer than the window to open RPC.
func applyIdentity(status, detail string, f identityFindings) (newStatus, newDetail string, notes []string) {
	notes = f.notes
	switch {
	case len(f.fails) > 0:
		detail = strings.Join(slices.Concat(f.fails, []string{detail}), "; ")
		return statusFail, detail, slices.Concat(notes, f.warns, f.facts)
	case len(f.warns) > 0 && status == statusPass:
		status, detail = statusWarn, strings.Join(slices.Concat(f.warns, []string{detail}), "; ")
	default:
		notes = append(notes, f.warns...)
	}
	if status == statusPass {
		return status, strings.Join(slices.Concat([]string{detail}, f.facts), "; "), notes
	}
	return status, detail, append(notes, f.facts...)
}

// dataUsageKiB measures the data volume through /proc/1/root of the main
// container, which works for tmpfs mounts and for images without du alike.
// SYS_PTRACE is needed when the node does not run as root.
func dataUsageKiB(ctx context.Context, t identityTarget) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, helperTimeout)
	defer cancel()
	args := slices.Concat(
		[]string{"run", "--rm", "--platform", smokePlatform, "--pid", "container:" + t.main, "--cap-add", "SYS_PTRACE"},
		labelArgs(t.prefix),
		[]string{busyboxImage, "du", "-sk", path.Join("/proc/1/root", t.dataPath)},
	)
	out, err := docker(ctx, args...)
	if err != nil {
		return 0, err
	}
	return parseDu(out)
}

// parseDu reads the size from `du -sk` output ("1234\t/path").
func parseDu(out string) (int64, error) {
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return 0, errors.New("du printed nothing")
	}
	n, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing du output %q: %w", strings.TrimSpace(out), err)
	}
	return n, nil
}

// rpcCall fetches url from inside the pod network namespace, a POST of body
// when it is not empty. Connection errors are retried a few times.
func rpcCall(ctx context.Context, t identityTarget, url, body string) (string, error) {
	args := slices.Concat(
		[]string{"run", "--rm", "--platform", smokePlatform, "--network", "container:" + t.netOwner},
		labelArgs(t.prefix),
		[]string{busyboxImage, "wget", "-q", "-O", "-", "-T", "5"},
	)
	if body != "" {
		args = append(args, "--header", "Content-Type: application/json", "--post-data", body)
	}
	args = append(args, url)

	var err error
	for attempt := range rpcAttempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(rpcRetryDelay):
			}
		}
		callCtx, cancel := context.WithTimeout(ctx, helperTimeout)
		var out string
		out, err = docker(callCtx, args...)
		cancel()
		if err == nil {
			return out, nil
		}
	}
	return "", errors.New(strings.TrimPrefix(err.Error(), "docker run: "))
}

// parseChainID decodes an eth_chainId response.
func parseChainID(body string) (uint64, error) {
	var resp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return 0, fmt.Errorf("eth_chainId: not JSON-RPC: %s", truncate(strings.TrimSpace(body), 80))
	}
	if resp.Error != nil {
		return 0, errors.New("eth_chainId: " + resp.Error.Message)
	}
	hex, ok := strings.CutPrefix(resp.Result, "0x")
	if !ok || hex == "" {
		return 0, fmt.Errorf("eth_chainId: result %q is not a hex quantity", resp.Result)
	}
	n, err := strconv.ParseUint(hex, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("eth_chainId: result %q: %w", resp.Result, err)
	}
	return n, nil
}

// parseCometBFTNetwork reads node_info.network from a CometBFT /status
// response, with or without the JSON-RPC envelope (Tendermint < 0.35 wraps
// it in "result").
func parseCometBFTNetwork(body string) (string, error) {
	type status struct {
		NodeInfo struct {
			Network string `json:"network"`
		} `json:"node_info"`
	}
	var resp struct {
		status
		Result status `json:"result"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return "", fmt.Errorf("/status: not JSON: %s", truncate(strings.TrimSpace(body), 80))
	}
	if n := cmp.Or(resp.Result.NodeInfo.Network, resp.NodeInfo.Network); n != "" {
		return n, nil
	}
	return "", errors.New("/status: no node_info.network")
}

// humanKiB renders a KiB count as KiB, MiB or GiB.
func humanKiB(kib int64) string {
	switch {
	case kib >= 1<<20:
		return fmt.Sprintf("%.1f GiB", float64(kib)/(1<<20))
	case kib >= 1<<10:
		return fmt.Sprintf("%d MiB", kib>>10)
	}
	return fmt.Sprintf("%d KiB", kib)
}
