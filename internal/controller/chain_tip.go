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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	nodesv1alpha1 "github.com/tazhate/blockchain-node-operator/api/v1alpha1"
)

// fetchPublicTip dispatches to the correct chain-specific endpoint to obtain
// the current network tip (block height or equivalent). A failure here must
// never mark a node as degraded; callers treat errors as "no opinion".
func fetchPublicTip(ctx context.Context, chain nodesv1alpha1.Chain) (int64, error) {
	switch chain {
	case nodesv1alpha1.ChainEthereum, nodesv1alpha1.ChainEthereumArchive:
		return fetchEVMTip(ctx, "https://ethereum.publicnode.com")
	case nodesv1alpha1.ChainPolygon:
		return fetchEVMTip(ctx, "https://polygon-bor.publicnode.com")
	case nodesv1alpha1.ChainBSC:
		return fetchEVMTip(ctx, "https://bsc.publicnode.com")
	case nodesv1alpha1.ChainAvalanche:
		return fetchEVMTip(ctx, "https://avalanche-c-chain.publicnode.com")
	case nodesv1alpha1.ChainBitcoin:
		return fetchPlainHeightTip(ctx, "https://blockstream.info/api/blocks/tip/height")
	case nodesv1alpha1.ChainLitecoin:
		return fetchPlainHeightTip(ctx, "https://litecoinspace.org/api/blocks/tip/height")
	case nodesv1alpha1.ChainDash:
		return fetchDashTip(ctx)
	case nodesv1alpha1.ChainTRON:
		return fetchTronTip(ctx)
	case nodesv1alpha1.ChainXRP:
		return fetchXRPTip(ctx)
	case nodesv1alpha1.ChainStellar:
		return fetchStellarTip(ctx)
	case nodesv1alpha1.ChainNear:
		return fetchNearTip(ctx)
	case nodesv1alpha1.ChainSui:
		return fetchSuiTip(ctx)
	case nodesv1alpha1.ChainTON:
		return fetchTONTip(ctx)
	case nodesv1alpha1.ChainCosmos:
		return fetchCosmosTip(ctx)
	case nodesv1alpha1.ChainAptos:
		return fetchAptosTip(ctx)
	case nodesv1alpha1.ChainCardano:
		return fetchCardanoTip(ctx)
	default:
		return 0, fmt.Errorf("no public tip endpoint for chain %s", chain)
	}
}

// ---------------------------------------------------------------------------
// EVM-compatible chains (JSON-RPC eth_blockNumber)
// ---------------------------------------------------------------------------

// fetchEVMTip calls any EVM-compatible JSON-RPC endpoint for eth_blockNumber.
func fetchEVMTip(ctx context.Context, endpoint string) (int64, error) {
	body := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`
	data, err := jsonRPCPost(ctx, endpoint, body)
	if err != nil {
		return 0, err
	}
	var resp struct{ Result string }
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing EVM tip from %s: %w", endpoint, err)
	}
	return hexToInt64(resp.Result), nil
}

// ---------------------------------------------------------------------------
// Plain-integer height APIs (Blockstream, Litecoinspace)
// ---------------------------------------------------------------------------

func fetchPlainHeightTip(ctx context.Context, url string) (int64, error) {
	data, err := httpGet(ctx, url)
	if err != nil {
		return 0, err
	}
	var height int64
	if err := json.Unmarshal(data, &height); err != nil {
		return 0, fmt.Errorf("parsing height from %s: %w", url, err)
	}
	return height, nil
}

// ---------------------------------------------------------------------------
// Chain-specific tip fetchers
// ---------------------------------------------------------------------------

func fetchDashTip(ctx context.Context) (int64, error) {
	data, err := httpGet(ctx, "https://insight.dash.org/api/status?q=getBestBlockHash")
	if err != nil {
		return 0, err
	}
	var resp struct {
		Info struct {
			Blocks int64 `json:"blocks"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing Dash tip: %w", err)
	}
	return resp.Info.Blocks, nil
}

func fetchTronTip(ctx context.Context) (int64, error) {
	data, err := httpGet(ctx, "https://api.trongrid.io/wallet/getnowblock")
	if err != nil {
		return 0, err
	}
	var resp struct {
		BlockHeader struct {
			RawData struct {
				Number int64 `json:"number"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing TRON tip: %w", err)
	}
	return resp.BlockHeader.RawData.Number, nil
}

func fetchXRPTip(ctx context.Context) (int64, error) {
	body := `{"method":"server_info","params":[{}]}`
	data, err := jsonRPCPost(ctx, "https://xrplcluster.com/", body)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Result struct {
			Info struct {
				ValidatedLedger struct {
					Seq int64 `json:"seq"`
				} `json:"validated_ledger"`
			} `json:"info"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing XRP tip: %w", err)
	}
	return resp.Result.Info.ValidatedLedger.Seq, nil
}

func fetchStellarTip(ctx context.Context) (int64, error) {
	data, err := httpGet(ctx, "https://horizon.stellar.org/ledgers?order=desc&limit=1")
	if err != nil {
		return 0, err
	}
	var resp struct {
		Embedded struct {
			Records []struct {
				Sequence int64 `json:"sequence"`
			} `json:"records"`
		} `json:"_embedded"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing Stellar tip: %w", err)
	}
	if len(resp.Embedded.Records) == 0 {
		return 0, fmt.Errorf("no ledger records from Horizon")
	}
	return resp.Embedded.Records[0].Sequence, nil
}

func fetchNearTip(ctx context.Context) (int64, error) {
	body := `{"jsonrpc":"2.0","method":"block","params":{"finality":"final"},"id":1}`
	data, err := jsonRPCPost(ctx, "https://rpc.mainnet.near.org", body)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Result struct {
			Header struct {
				Height int64 `json:"height"`
			} `json:"header"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing NEAR tip: %w", err)
	}
	return resp.Result.Header.Height, nil
}

func fetchSuiTip(ctx context.Context) (int64, error) {
	body := `{"jsonrpc":"2.0","method":"sui_getLatestCheckpointSequenceNumber","params":[],"id":1}`
	data, err := jsonRPCPost(ctx, "https://sui-rpc.publicnode.com", body)
	if err != nil {
		return 0, err
	}
	var resp struct{ Result string }
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing SUI tip: %w", err)
	}
	return strconv.ParseInt(resp.Result, 10, 64)
}

func fetchTONTip(ctx context.Context) (int64, error) {
	data, err := httpGet(ctx, "https://toncenter.com/api/v2/getMasterchainInfo")
	if err != nil {
		return 0, err
	}
	var resp struct {
		Result struct {
			Last struct {
				Seqno int64 `json:"seqno"`
			} `json:"last"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing TON tip: %w", err)
	}
	return resp.Result.Last.Seqno, nil
}

func fetchCosmosTip(ctx context.Context) (int64, error) {
	data, err := httpGet(ctx, "https://cosmos-rest.publicnode.com/cosmos/base/tendermint/v1beta1/blocks/latest")
	if err != nil {
		return 0, err
	}
	var resp struct {
		Block struct {
			Header struct {
				Height string `json:"height"`
			} `json:"header"`
		} `json:"block"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing Cosmos tip: %w", err)
	}
	return strconv.ParseInt(resp.Block.Header.Height, 10, 64)
}

func fetchAptosTip(ctx context.Context) (int64, error) {
	data, err := httpGet(ctx, "https://fullnode.mainnet.aptoslabs.com/v1")
	if err != nil {
		return 0, err
	}
	var resp struct {
		BlockHeight string `json:"block_height"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing Aptos tip: %w", err)
	}
	return strconv.ParseInt(resp.BlockHeight, 10, 64)
}

func fetchCardanoTip(ctx context.Context) (int64, error) {
	data, err := httpGet(ctx, "https://api.koios.rest/api/v1/tip")
	if err != nil {
		return 0, err
	}
	var resp []struct {
		AbsSlot int64 `json:"abs_slot"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing Cardano tip: %w", err)
	}
	if len(resp) == 0 {
		return 0, fmt.Errorf("no tip records from Koios")
	}
	return resp[0].AbsSlot, nil
}

// ---------------------------------------------------------------------------
// HTTP transport helpers
// ---------------------------------------------------------------------------

// jsonRPCPost performs a JSON-RPC POST and returns the raw response body.
func jsonRPCPost(ctx context.Context, url, body string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tipHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// httpGet performs a plain GET and returns the response body.
func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := tipHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
