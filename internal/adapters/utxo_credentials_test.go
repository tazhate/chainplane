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
package adapters_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
)

var utxoChains = []chainsv1alpha2.Chain{
	chainsv1alpha2.ChainBitcoin,
	chainsv1alpha2.ChainDash,
	chainsv1alpha2.ChainLitecoin,
	chainsv1alpha2.ChainDogecoin,
}

func rpcCredentialed(t *testing.T, chain chainsv1alpha2.Chain) adapters.RPCCredentialed {
	t.Helper()
	adapter, ok := adapters.Get(chain)
	if !ok {
		t.Fatalf("adapter not registered for chain: %s", chain)
	}
	rc, ok := adapter.(adapters.RPCCredentialed)
	if !ok {
		t.Fatalf("%s adapter does not implement RPCCredentialed", chain)
	}
	return rc
}

// Expected values computed with Python, matching Bitcoin Core share/rpcauth:
//
//	hmac.new(salt.encode(), password.encode(), 'SHA256').hexdigest()
//	hashlib.sha256(b'chainplanehunter2').hexdigest()[:32]  # derived salt
func TestRPCAuthKnownVectors(t *testing.T) {
	tests := []struct {
		name  string
		creds adapters.RPCCredentials
		want  string
	}{
		{
			name: "stored salt",
			creds: adapters.RPCCredentials{
				User:     "alice",
				Password: "correct horse battery staple",
				Salt:     "0123456789abcdef0123456789abcdef",
			},
			want: "alice:0123456789abcdef0123456789abcdef$90ff3ce205bece1c447b777562931b92721a1956552484506bf368ee0278bcd3",
		},
		{
			name:  "derived salt",
			creds: adapters.RPCCredentials{User: "chainplane", Password: "hunter2"},
			want:  "chainplane:9998d2f7f0cdc3b6b66bcda7dd6d5a70$738666f90557aef72e26323f593bb32a9d41f3f08049367742939aff8357401d",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.creds.RPCAuth(); got != tt.want {
				t.Errorf("RPCAuth() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRPCCredentialsValidate(t *testing.T) {
	tests := []struct {
		name    string
		creds   adapters.RPCCredentials
		wantErr bool
	}{
		{name: "valid", creds: adapters.RPCCredentials{User: "u", Password: "p"}},
		{name: "valid with salt", creds: adapters.RPCCredentials{User: "u", Password: "p", Salt: "abcd"}},
		{name: "empty password", creds: adapters.RPCCredentials{User: "u"}, wantErr: true},
		{name: "empty user", creds: adapters.RPCCredentials{Password: "p"}, wantErr: true},
		{name: "colon in user", creds: adapters.RPCCredentials{User: "a:b", Password: "p"}, wantErr: true},
		{name: "dollar in user", creds: adapters.RPCCredentials{User: "a$b", Password: "p"}, wantErr: true},
		{name: "newline in user", creds: adapters.RPCCredentials{User: "u\nrpcallowip=1.2.3.4", Password: "p"}, wantErr: true},
		{name: "non-hex salt", creds: adapters.RPCCredentials{User: "u", Password: "p", Salt: "zz\nx=1"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.creds.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUTXOConfigUsesRPCAuth(t *testing.T) {
	creds := adapters.RPCCredentials{
		User:     "chainplane",
		Password: "s3cret-Password-value",
		Salt:     "0123456789abcdef0123456789abcdef",
	}
	for _, chain := range utxoChains {
		for _, network := range []chainsv1alpha2.Network{chainsv1alpha2.NetworkMainnet, chainsv1alpha2.NetworkTestnet} {
			t.Run(string(chain)+"/"+string(network), func(t *testing.T) {
				rc := rpcCredentialed(t, chain)
				spec := chainsv1alpha2.ChainInstanceSpec{Chain: chain, Network: network}
				_, content, err := rc.ConfigTemplateWithCredentials(spec, creds)
				if err != nil {
					t.Fatal(err)
				}
				if want := "\nrpcauth=" + creds.RPCAuth() + "\n"; !strings.Contains(content, want) {
					t.Errorf("config missing %q:\n%s", strings.TrimSpace(want), content)
				}
				for _, forbidden := range []string{"rpcpassword", "rpcuser", creds.Password} {
					if strings.Contains(content, forbidden) {
						t.Errorf("config must not contain %q:\n%s", forbidden, content)
					}
				}
			})
		}
	}
}

func TestUTXOConfigWithoutCredentialsHasNoAuth(t *testing.T) {
	for _, chain := range utxoChains {
		t.Run(string(chain), func(t *testing.T) {
			adapter, _ := adapters.Get(chain)
			_, content, err := adapter.ConfigTemplate(chainsv1alpha2.ChainInstanceSpec{Chain: chain})
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"rpcauth", "rpcuser", "rpcpassword"} {
				if strings.Contains(content, forbidden) {
					t.Errorf("config without credentials must not contain %q:\n%s", forbidden, content)
				}
			}
		})
	}
}

func TestUTXOConfigRejectsInjectedUser(t *testing.T) {
	rc := rpcCredentialed(t, chainsv1alpha2.ChainBitcoin)
	creds := adapters.RPCCredentials{User: "x\nrpcallowip=0.0.0.0/0", Password: "p"}
	if _, _, err := rc.ConfigTemplateWithCredentials(chainsv1alpha2.ChainInstanceSpec{}, creds); err == nil {
		t.Fatal("expected error for user with newline")
	}
}

func TestUTXORPCSidecarsUseSecretKeyRef(t *testing.T) {
	const secretName = "btc-0-rpc-credentials"
	for _, chain := range utxoChains {
		t.Run(string(chain), func(t *testing.T) {
			rc := rpcCredentialed(t, chain)
			sidecars := rc.RPCSidecars(chainsv1alpha2.ChainInstanceSpec{Chain: chain}, secretName)
			if len(sidecars) == 0 {
				t.Fatal("expected an exporter sidecar")
			}
			wantKeys := map[string]string{
				"BITCOIN_RPC_USER":     adapters.RPCSecretUserKey,
				"BITCOIN_RPC_PASSWORD": adapters.RPCSecretPasswordKey,
			}
			for _, env := range sidecars[0].Env {
				key, ok := wantKeys[env.Name]
				if !ok {
					continue
				}
				delete(wantKeys, env.Name)
				if env.Value != "" {
					t.Errorf("%s must not have a plain value", env.Name)
				}
				ref := env.ValueFrom
				if ref == nil || ref.SecretKeyRef == nil {
					t.Fatalf("%s must use SecretKeyRef", env.Name)
				}
				if ref.SecretKeyRef.Name != secretName || ref.SecretKeyRef.Key != key {
					t.Errorf("%s refs %s/%s, want %s/%s", env.Name, ref.SecretKeyRef.Name, ref.SecretKeyRef.Key, secretName, key)
				}
			}
			if len(wantKeys) != 0 {
				t.Errorf("missing env vars: %v", wantKeys)
			}
		})
	}
}

func TestUTXORPCEnvNames(t *testing.T) {
	want := map[chainsv1alpha2.Chain][2]string{
		chainsv1alpha2.ChainBitcoin:  {"BTC_RPC_USER", "BTC_RPC_PASSWORD"},
		chainsv1alpha2.ChainDash:     {"DASH_RPC_USER", "DASH_RPC_PASSWORD"},
		chainsv1alpha2.ChainLitecoin: {"LTC_RPC_USER", "LTC_RPC_PASSWORD"},
		chainsv1alpha2.ChainDogecoin: {"DOGE_RPC_USER", "DOGE_RPC_PASSWORD"},
	}
	for chain, names := range want {
		user, pass := rpcCredentialed(t, chain).RPCEnvNames()
		if user != names[0] || pass != names[1] {
			t.Errorf("%s: RPCEnvNames() = %s, %s; want %s, %s", chain, user, pass, names[0], names[1])
		}
	}
}

func TestUTXOHealthCheckWithCredentialsSendsBasicAuth(t *testing.T) {
	creds := adapters.RPCCredentials{User: "chainplane", Password: "per-node-secret"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != creds.User || pass != creds.Password {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &req)
		result := `5`
		if req.Method == "getblockchaininfo" {
			result = `{"blocks":100,"headers":100,"verificationprogress":1}`
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":`+result+`}`)
	}))
	defer srv.Close()

	rc := rpcCredentialed(t, chainsv1alpha2.ChainDash)
	status, err := rc.HealthCheckWithCredentials(t.Context(), srv.URL, creds)
	if err != nil {
		t.Fatalf("HealthCheckWithCredentials: %v", err)
	}
	if status.CurrentBlock != 100 || status.Peers != 5 {
		t.Errorf("unexpected status: %+v", status)
	}

	if _, err := rc.HealthCheckWithCredentials(t.Context(), srv.URL, adapters.RPCCredentials{User: "rpc", Password: "rpc"}); err == nil {
		t.Error("expected failure with wrong credentials")
	}
}
