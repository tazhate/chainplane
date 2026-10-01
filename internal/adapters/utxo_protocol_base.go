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
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

// Keys of the per-node "<name>-rpc-credentials" Secret.
const (
	RPCSecretUserKey     = "rpc-user"
	RPCSecretPasswordKey = "rpc-password"
	RPCSecretSaltKey     = "rpc-auth-salt"
)

// utxoExporterImage is the Prometheus exporter used for all Bitcoin-family chains.
const utxoExporterImage = "jvstein/bitcoin-prometheus-exporter:v0.8.0"

// RPCCredentials are the JSON-RPC credentials of a single Bitcoin-family node,
// loaded by the controller from that node's Secret. They are passed explicitly
// on every call and never stored in adapter or process state.
type RPCCredentials struct {
	User     string
	Password string
	// Salt is the hex rpcauth salt. When empty, one is derived deterministically
	// from User and Password so the rendered config stays stable.
	Salt string
}

// IsZero reports whether no credentials are set.
func (c RPCCredentials) IsZero() bool {
	return c.User == "" && c.Password == ""
}

// Validate rejects values that bitcoind cannot parse in an rpcauth line or
// that would let a Secret inject extra lines into the node config.
func (c RPCCredentials) Validate() error {
	if c.User == "" || c.Password == "" {
		return errors.New("rpc user and password must both be set")
	}
	if strings.ContainsAny(c.User, ":$") || strings.ContainsFunc(c.User, isSpaceOrControl) {
		return errors.New("rpc user must not contain ':', '$', whitespace or control characters")
	}
	if c.Salt != "" {
		if _, err := hex.DecodeString(c.Salt); err != nil {
			return fmt.Errorf("rpc auth salt must be hex: %w", err)
		}
	}
	return nil
}

func isSpaceOrControl(r rune) bool {
	return r <= ' ' || r == 0x7f
}

// RPCAuth returns the value of a bitcoind rpcauth= line, in the format produced
// by Bitcoin Core share/rpcauth/rpcauth.py: "<user>:<salt>$<hmac>", where hmac
// is hex(HMAC-SHA256(key=salt, msg=password)). The config file then carries no
// password, only a salted hash.
func (c RPCCredentials) RPCAuth() string {
	salt := c.Salt
	if salt == "" {
		sum := sha256.Sum256([]byte(c.User + c.Password))
		salt = hex.EncodeToString(sum[:])[:32]
	}
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(c.Password))
	return c.User + ":" + salt + "$" + hex.EncodeToString(mac.Sum(nil))
}

// RPCCredentialed is an optional interface for adapters whose node RPC needs
// per-instance credentials (Bitcoin-family chains). The controller provisions
// the credentials Secret and uses these methods instead of the plain
// ConfigTemplate / HealthCheck / Sidecars variants.
type RPCCredentialed interface {
	// RPCEnvNames returns the env var names under which the main container
	// receives the RPC user and password from the Secret.
	RPCEnvNames() (userEnv, passwordEnv string)
	// ConfigTemplateWithCredentials renders the config with an rpcauth line.
	ConfigTemplateWithCredentials(spec chainsv1alpha2.ChainInstanceSpec, creds RPCCredentials) (filename string, content string, err error)
	// HealthCheckWithCredentials runs HealthCheck authenticated with creds.
	HealthCheckWithCredentials(ctx context.Context, rpcURL string, creds RPCCredentials) (SyncStatus, error)
	// RPCSidecars returns sidecars that read credentials from secretName.
	RPCSidecars(spec chainsv1alpha2.ChainInstanceSpec, secretName string) []corev1.Container
}

// utxoConfigData is the template input shared by all Bitcoin-family configs.
type utxoConfigData struct {
	IsTestnet bool
	Testnet   string // "0" or "1"
	RPCAuth   string // empty renders no rpcauth line (cookie auth only)
}

// utxoProtocolAdapter is the shared base for Bitcoin-family chains (Bitcoin,
// Litecoin, Dash, Dogecoin). It provides common getblockchaininfo-based health
// checking, config rendering and RPC auth handling.
type utxoProtocolAdapter struct {
	protocolAdapter
	rpcUserEnv     string // env var name for RPC user on the node container (e.g. "BTC_RPC_USER")
	rpcPasswordEnv string // env var name for RPC password on the node container
	configFile     string
	configTpl      *template.Template
	stallPolicy    string // see utxoHealthCheck
	useRetry       bool   // whether to use callRPCWithRetry
}

func (u *utxoProtocolAdapter) RPCEnvNames() (userEnv, passwordEnv string) {
	return u.rpcUserEnv, u.rpcPasswordEnv
}

// ConfigTemplate renders the config without RPC credentials. Only the
// controller knows a node's credentials, so it calls
// ConfigTemplateWithCredentials instead.
func (u *utxoProtocolAdapter) ConfigTemplate(spec chainsv1alpha2.ChainInstanceSpec) (string, string, error) {
	return u.ConfigTemplateWithCredentials(spec, RPCCredentials{})
}

func (u *utxoProtocolAdapter) ConfigTemplateWithCredentials(spec chainsv1alpha2.ChainInstanceSpec, creds RPCCredentials) (string, string, error) {
	data := utxoConfigData{
		IsTestnet: spec.Network == chainsv1alpha2.NetworkTestnet,
		Testnet:   "0",
	}
	if data.IsTestnet {
		data.Testnet = "1"
	}
	if !creds.IsZero() {
		if err := creds.Validate(); err != nil {
			return "", "", err
		}
		data.RPCAuth = creds.RPCAuth()
	}
	var buf bytes.Buffer
	if err := u.configTpl.Execute(&buf, data); err != nil {
		return "", "", err
	}
	return u.configFile, buf.String(), nil
}

// HealthCheck queries the node without credentials; see HealthCheckWithCredentials.
func (u *utxoProtocolAdapter) HealthCheck(ctx context.Context, rpcURL string) (SyncStatus, error) {
	return utxoHealthCheck(ctx, rpcURL, u)
}

func (u *utxoProtocolAdapter) HealthCheckWithCredentials(ctx context.Context, rpcURL string, creds RPCCredentials) (SyncStatus, error) {
	authURL, err := authenticatedURL(rpcURL, creds)
	if err != nil {
		return SyncStatus{}, err
	}
	return utxoHealthCheck(ctx, authURL, u)
}

// authenticatedURL injects RPC credentials into the given URL.
func authenticatedURL(rpcURL string, creds RPCCredentials) (string, error) {
	parsed, err := url.Parse(rpcURL)
	if err != nil {
		return "", fmt.Errorf("parse rpc url: %w", err)
	}
	if !creds.IsZero() {
		parsed.User = url.UserPassword(creds.User, creds.Password)
	}
	return parsed.String(), nil
}

// utxoExporterSidecar returns a bitcoin-prometheus-exporter container that
// reads the node RPC credentials from secretName via SecretKeyRef, so no
// password appears in the pod spec.
func utxoExporterSidecar(rpcPort int32, secretName string) corev1.Container {
	secretRef := func(key string) *corev1.EnvVarSource {
		return &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  key,
			},
		}
	}
	return corev1.Container{
		Name:  "metrics-exporter",
		Image: utxoExporterImage,
		Ports: []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 9332, Protocol: corev1.ProtocolTCP},
		},
		Env: []corev1.EnvVar{
			{Name: "BITCOIN_RPC_HOST", Value: "localhost"},
			{Name: "BITCOIN_RPC_PORT", Value: strconv.Itoa(int(rpcPort))},
			{Name: "BITCOIN_RPC_USER", ValueFrom: secretRef(RPCSecretUserKey)},
			{Name: "BITCOIN_RPC_PASSWORD", ValueFrom: secretRef(RPCSecretPasswordKey)},
		},
	}
}

// blockchainInfo holds the common fields from getblockchaininfo across UTXO chains.
type blockchainInfo struct {
	Blocks               int64   `json:"blocks"`
	Headers              int64   `json:"headers"`
	VerificationProgress float64 `json:"verificationprogress"`
}

// utxoHealthCheck performs a standard UTXO-chain health check via getblockchaininfo.
// authURL must already carry any required credentials.
// u.stallPolicy determines how StallExempt is set based on sync state:
//   - "synced-exempt": exempt when fully synced (BTC, DASH, DOGE)
//   - "ibd-exempt":    exempt during initial block download at <95% (LTC)
func utxoHealthCheck(ctx context.Context, authURL string, u *utxoProtocolAdapter) (SyncStatus, error) {
	var (
		result json.RawMessage
		err    error
	)
	if u.useRetry {
		result, err = callRPCWithRetry(ctx, authURL, "getblockchaininfo", nil, 2)
	} else {
		result, err = callRPC(ctx, authURL, "getblockchaininfo", nil)
	}
	if err != nil {
		if u.useRetry && isTransientRPCError(err) {
			return SyncStatus{IsSyncing: true, StallExempt: true, Progress: 0}, nil
		}
		return SyncStatus{}, fmt.Errorf("getblockchaininfo: %w", err)
	}

	var info blockchainInfo
	if err := json.Unmarshal(result, &info); err != nil {
		return SyncStatus{}, fmt.Errorf("parse getblockchaininfo: %w", err)
	}

	var peers int32
	if peerResult, err2 := callRPC(ctx, authURL, "getconnectioncount", nil); err2 == nil {
		_ = json.Unmarshal(peerResult, &peers)
	}

	isSyncing := info.VerificationProgress < 0.999

	var stallExempt bool
	switch u.stallPolicy {
	case "synced-exempt":
		// Fully-synced nodes: block intervals are long (BTC ~10min, DASH ~2.5min).
		stallExempt = !isSyncing
	case "ibd-exempt":
		// During IBD, height can freeze for >25 min while validating large batches.
		stallExempt = isSyncing && info.VerificationProgress < 0.95
	}

	return SyncStatus{
		IsSyncing:    isSyncing,
		CurrentBlock: info.Blocks,
		HighestBlock: info.Headers,
		Progress:     info.VerificationProgress * 100.0,
		Peers:        peers,
		StallExempt:  stallExempt,
	}, nil
}

var (
	_ RPCCredentialed = (*bitcoinAdapter)(nil)
	_ RPCCredentialed = (*dashAdapter)(nil)
	_ RPCCredentialed = (*litecoinAdapter)(nil)
	_ RPCCredentialed = (*dogecoinAdapter)(nil)
)
