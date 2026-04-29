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
// versioncheck checks upstream registries for newer blockchain node images and
// optionally updates internal/adapters/versions_gen.go with the latest tags.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
	"github.com/tazhate/chainplane/internal/adapters"
	_ "github.com/tazhate/chainplane/internal/adapters" // register all adapters via init()
	"github.com/tazhate/chainplane/internal/registry"
)

func main() {
	var (
		update      = flag.Bool("update", false, "write updated versions to versions_gen.go")
		filterChain = flag.String("chain", "", "check only this chain (e.g. bitcoin)")
		concurrency = flag.Int("concurrency", 10, "parallel registry requests")
		timeout     = flag.Duration("timeout", 30*time.Second, "per-request timeout")
	)
	flag.Parse()

	ctx := context.Background()

	results := checkVersions(ctx, *filterChain, *concurrency, *timeout)

	printReport(results)

	if !*update {
		return
	}

	newer := filterNewer(results)
	if len(newer) == 0 {
		fmt.Println("\nAll versions up to date.")
		return
	}

	if err := applyUpdates(newer); err != nil {
		log.Fatalf("failed to update versions_gen.go: %v", err)
	}
	fmt.Printf("\nUpdated %d image(s) in versions_gen.go\n", len(newer))
}

// versionResult holds the check result for one chain+client pair.
type versionResult struct {
	Chain      chainsv1alpha2.Chain
	Client     string // "" means chain default
	CurrentRef string // full image ref
	CurrentTag string // tag portion of CurrentRef
	LatestTag  string // latest from registry (empty on error)
	IsNewer    bool
	Err        error
}

func checkVersions(
	ctx context.Context, filterChain string, concurrency int, timeout time.Duration,
) []versionResult {
	all := adapters.All()

	type workItem struct {
		chain  chainsv1alpha2.Chain
		policy adapters.ChainVersionPolicy
	}

	var items []workItem
	for chain, adapter := range all {
		vp, ok := adapter.(adapters.VersionProvider)
		if !ok {
			continue
		}
		if filterChain != "" && string(chain) != filterChain {
			continue
		}
		items = append(items, workItem{chain: chain, policy: vp.VersionPolicy()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].chain < items[j].chain })

	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var results []versionResult
	var wg sync.WaitGroup

	for _, item := range items {
		wg.Add(1)
		go func(it workItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			reqCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			res := versionResult{Chain: it.chain}

			currentRef := adapters.DefaultImageFor(it.chain, "")
			res.CurrentRef = currentRef
			res.CurrentTag = parseTag(currentRef)

			client, err := registry.NewClient(it.policy.Registry)
			if err != nil {
				res.Err = err
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
				return
			}

			tags, err := client.LatestTags(reqCtx, it.policy, 25)
			if err != nil {
				res.Err = err
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
				return
			}

			// Pick the latest stable tag (skip pre-releases, nightly, latest, etc.)
			for _, t := range tags {
				if isStableTag(t.Tag, it.policy.TagPrefix) {
					res.LatestTag = t.Tag
					res.IsNewer = registry.IsNewer(res.LatestTag, res.CurrentTag, it.policy.TagPrefix)
					break
				}
			}

			mu.Lock()
			results = append(results, res)
			mu.Unlock()
		}(item)
	}

	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].Chain < results[j].Chain })
	return results
}

func filterNewer(results []versionResult) []versionResult {
	var out []versionResult
	for _, r := range results {
		if r.IsNewer {
			out = append(out, r)
		}
	}
	return out
}

func printReport(results []versionResult) {
	fmt.Printf("%-30s %-30s %-20s %s\n", "CHAIN", "CURRENT TAG", "LATEST TAG", "STATUS")
	fmt.Println(strings.Repeat("-", 100))
	for _, r := range results {
		status := "up to date"
		if r.Err != nil {
			status = fmt.Sprintf("ERROR: %v", r.Err)
		} else if r.IsNewer {
			status = "UPDATE AVAILABLE"
		}
		fmt.Printf("%-30s %-30s %-20s %s\n", r.Chain, r.CurrentTag, r.LatestTag, status)
	}
}

// applyUpdates patches the chainDefaultImages entries and rewrites versions_gen.go.
func applyUpdates(updates []versionResult) error {
	genPath, err := versionsGenPath()
	if err != nil {
		return err
	}

	images, err := parseVersionsGen(genPath)
	if err != nil {
		return fmt.Errorf("parsing versions_gen.go: %w", err)
	}

	for _, u := range updates {
		clients, ok := images[u.Chain]
		if !ok {
			continue
		}
		if oldRef, ok2 := clients[u.Client]; ok2 {
			clients[u.Client] = replaceTag(oldRef, u.LatestTag)
		}
	}

	return writeVersionsGen(genPath, images)
}

// isStableTag reports whether tag is a stable release — not a pre-release, nightly, or floating tag.
// prefix is stripped before the check (e.g. "GreatVoyage-" for TRON).
func isStableTag(tag, prefix string) bool {
	stripped := strings.TrimPrefix(tag, prefix)
	lower := strings.ToLower(stripped)
	// Reject floating / non-versioned tags
	floating := []string{"latest", "nightly", "canary", "edge", "develop", "main", "master", "unstable"}
	for _, f := range floating {
		if lower == f {
			return false
		}
	}
	// Reject pre-release suffix keywords that appear after a "-"
	preRelease := []string{"-alpha", "-beta", "-rc", "-dev", "-pre", "-snapshot", "-test", "-experimental"}
	for _, p := range preRelease {
		if strings.Contains(lower, p) {
			return false
		}
	}
	return true
}

// parseTag extracts the tag from an image reference (everything after the last ":").
func parseTag(imageRef string) string {
	if i := strings.LastIndex(imageRef, ":"); i >= 0 {
		return imageRef[i+1:]
	}
	return imageRef
}

// replaceTag swaps the tag portion of an image reference.
func replaceTag(imageRef, newTag string) string {
	if i := strings.LastIndex(imageRef, ":"); i >= 0 {
		return imageRef[:i+1] + newTag
	}
	return imageRef + ":" + newTag
}

// versionsGenPath resolves the path to versions_gen.go by walking up to find go.mod.
func versionsGenPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "internal", "adapters", "versions_gen.go"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not locate go.mod (run from repo root)")
}

// mapEntryRe matches Go map string entries in both normal and aligned formats:
//
//	"key": "value",
//	"key":    "value",
var mapEntryRe = regexp.MustCompile(`^"([^"]*)"\s*:\s*"([^"]*)"\s*,?\s*$`)

// parseVersionsGen reads versions_gen.go and extracts the chainDefaultImages map.
// Relies on the machine-generated file having a stable, well-known format.
func parseVersionsGen(path string) (map[chainsv1alpha2.Chain]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := make(map[chainsv1alpha2.Chain]map[string]string)
	lines := strings.Split(string(data), "\n")

	var currentChain chainsv1alpha2.Chain
	inMap := false
	inChain := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "chainDefaultImages") && strings.Contains(trimmed, "{") {
			inMap = true
			continue
		}
		if !inMap {
			continue
		}

		// Chain entry: chainsv1alpha2.ChainBitcoin: {
		if strings.HasPrefix(trimmed, "chainsv1alpha2.Chain") && strings.HasSuffix(trimmed, ": {") {
			constName := strings.TrimPrefix(strings.TrimSuffix(trimmed, ": {"), "chainsv1alpha2.")
			currentChain = lookupChainByConst(constName)
			result[currentChain] = make(map[string]string)
			inChain = true
			continue
		}

		if inChain {
			// Client entry: "key": "value",  or aligned: "key":   "value",
			if m := mapEntryRe.FindStringSubmatch(trimmed); m != nil {
				result[currentChain][m[1]] = m[2]
				continue
			}
			if trimmed == "}," || trimmed == "}" {
				inChain = false
				currentChain = ""
			}
		}

		if trimmed == "}" && !inChain {
			break
		}
	}

	return result, nil
}

// lookupChainByConst finds the Chain value for a given Go constant name like "ChainBitcoin".
// It builds the lookup map lazily from the adapter registry.
var (
	constToChain     map[string]chainsv1alpha2.Chain
	constToChainOnce sync.Once
)

func lookupChainByConst(constName string) chainsv1alpha2.Chain {
	constToChainOnce.Do(func() {
		constToChain = make(map[string]chainsv1alpha2.Chain)
		for chain := range adapters.All() {
			constToChain[chainValueToConstName(string(chain))] = chain
		}
	})
	if chain, ok := constToChain[constName]; ok {
		return chain
	}
	// Fallback: return the const name as-is (shouldn't happen for well-formed files)
	return chainsv1alpha2.Chain(constName)
}

// chainValueToConstName maps chain string values to their exact Go constant names.
// Generated from api/v1alpha2/chaininstance_types.go — update when new chains are added.
func chainValueToConstName(value string) string {
	return chainConstNames[value]
}

// chainConstNames is the authoritative map from chain runtime value to Go constant name.
var chainConstNames = map[string]string{
	"abstract":         "ChainAbstract",
	"aptos":            "ChainAptos",
	"arbitrum":         "ChainArbitrum",
	"aurora":           "ChainAurora",
	"avalanche":        "ChainAvalanche",
	"axelar":           "ChainAxelar",
	"base":             "ChainBase",
	"berachain":        "ChainBerachain",
	"bitcoin":          "ChainBitcoin",
	"bittorrent":       "ChainBitTorrent",
	"blast":            "ChainBlast",
	"bob":              "ChainBob",
	"boba-eth":         "ChainBobaEth",
	"bsc":              "ChainBSC",
	"cardano":          "ChainCardano",
	"celo":             "ChainCelo",
	"core":             "ChainCore",
	"cosmos":           "ChainCosmos",
	"cronos":           "ChainCronos",
	"cronos-zkevm":     "ChainCronosZkEVM",
	"dash":             "ChainDash",
	"dogecoin":         "ChainDogecoin",
	"doma":             "ChainDoma",
	"dymension":        "ChainDymension",
	"ethereum":         "ChainEthereum",
	"ethereum-archive": "ChainEthereumArchive",
	"ethereum-beacon":  "ChainEthereumBeacon",
	"ethereum-classic": "ChainEthereumClassic",
	"everclear":        "ChainEverclear",
	"evmos":            "ChainEvmos",
	"fantom":           "ChainFantom",
	"filecoin":         "ChainFilecoin",
	"fraxtal":          "ChainFraxtal",
	"fuse":             "ChainFuse",
	"gnosis":           "ChainGnosis",
	"gnosis-beacon":    "ChainGnosisBeacon",
	"goat":             "ChainGoat",
	"gravity-alpha":    "ChainGravityAlpha",
	"haqq":             "ChainHaqq",
	"harmony":          "ChainHarmony",
	"hashkey":          "ChainHashKey",
	"hemi":             "ChainHemi",
	"hyperliquid":      "ChainHyperliquid",
	"immutable-zkevm":  "ChainImmutableZkEVM",
	"ink":              "ChainInk",
	"katana":           "ChainKatana",
	"kava":             "ChainKava",
	"klaytn":           "ChainKlaytn",
	"kroma":            "ChainKroma",
	"kusama":           "ChainKusama",
	"lens":             "ChainLens",
	"linea":            "ChainLinea",
	"lisk":             "ChainLisk",
	"litecoin":         "ChainLitecoin",
	"manta-pacific":    "ChainMantaPacific",
	"mantle":           "ChainMantle",
	"megaeth":          "ChainMegaETH",
	"metis":            "ChainMetis",
	"mezo":             "ChainMezo",
	"moca":             "ChainMoca",
	"mode":             "ChainMode",
	"monad":            "ChainMonad",
	"moonbeam":         "ChainMoonbeam",
	"moonriver":        "ChainMoonriver",
	"morph":            "ChainMorph",
	"near":             "ChainNear",
	"opbnb":            "ChainOpBNB",
	"optimism":         "ChainOptimism",
	"osmosis":          "ChainOsmosis",
	"plasma":           "ChainPlasma",
	"playnance":        "ChainPlaynance",
	"plume":            "ChainPlume",
	"polkadot":         "ChainPolkadot",
	"polygon":          "ChainPolygon",
	"polygon-zkevm":    "ChainPolygonZkEVM",
	"ronin":            "ChainRonin",
	"rootstock":        "ChainRootstock",
	"scroll":           "ChainScroll",
	"sei":              "ChainSei",
	"shibarium":        "ChainShibarium",
	"solana":           "ChainSolana",
	"soneium":          "ChainSoneium",
	"sonic":            "ChainSonic",
	"starknet":         "ChainStarknet",
	"stellar":          "ChainStellar",
	"sui":              "ChainSui",
	"superseed":        "ChainSuperseed",
	"swell":            "ChainSwell",
	"taiko":            "ChainTaiko",
	"telos":            "ChainTelos",
	"thundercore":      "ChainThundercore",
	"ton":              "ChainTON",
	"tron":             "ChainTRON",
	"unichain":         "ChainUnichain",
	"viction":          "ChainViction",
	"wemix":            "ChainWemix",
	"worldchain":       "ChainWorldchain",
	"xrp":              "ChainXRP",
	"zero-network":     "ChainZeroNetwork",
	"zircuit":          "ChainZircuit",
	"zksync":           "ChainZkSync",
	"zora":             "ChainZora",
}

// versionsGenTemplate is the template for regenerating versions_gen.go.
const versionsGenTemplate = `// Code generated by cmd/versioncheck on {{.Date}}. DO NOT EDIT.

package adapters

import (
	"strings"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

//go:generate go run ../../cmd/versioncheck --update

// chainDefaultImages maps each chain and optional client to its default container image.
// Inner key: lowercase client name, or "" for the chain default.
var chainDefaultImages = map[chainsv1alpha2.Chain]map[string]string{
{{- range .Entries}}
	chainsv1alpha2.{{.ConstName}}: {
	{{- range .Clients}}
		{{printf "%q" .Key}}: {{printf "%q" .Image}},
	{{- end}}
	},
{{- end}}
}

// DefaultImageFor returns the default container image for a chain and client.
// Falls back to the chain default (key "") when the client has no dedicated entry.
func DefaultImageFor(chain chainsv1alpha2.Chain, client string) string {
	clients, ok := chainDefaultImages[chain]
	if !ok {
		return ""
	}
	if img, ok := clients[strings.ToLower(client)]; ok {
		return img
	}
	return clients[""]
}
`

type clientEntry struct {
	Key   string
	Image string
}

type chainEntry struct {
	ConstName string
	Clients   []clientEntry
}

type templateData struct {
	Date    string
	Entries []chainEntry
}

func writeVersionsGen(path string, images map[chainsv1alpha2.Chain]map[string]string) error {
	chains := make([]chainsv1alpha2.Chain, 0, len(images))
	for chain := range images {
		chains = append(chains, chain)
	}
	sort.Slice(chains, func(i, j int) bool { return chains[i] < chains[j] })

	// Ensure lookup is populated
	lookupChainByConst("_init")

	reverseMap := make(map[chainsv1alpha2.Chain]string)
	for constName, chain := range constToChain {
		reverseMap[chain] = constName
	}

	entries := make([]chainEntry, 0, len(chains))
	for _, chain := range chains {
		constName, ok := reverseMap[chain]
		if !ok {
			constName = chainValueToConstName(string(chain))
		}

		clients := images[chain]
		keys := make([]string, 0, len(clients))
		for k := range clients {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i] == "" {
				return true
			}
			if keys[j] == "" {
				return false
			}
			return keys[i] < keys[j]
		})

		clientEntries := make([]clientEntry, 0, len(keys))
		for _, k := range keys {
			clientEntries = append(clientEntries, clientEntry{Key: k, Image: clients[k]})
		}
		entries = append(entries, chainEntry{ConstName: constName, Clients: clientEntries})
	}

	data := templateData{
		Date:    time.Now().UTC().Format("2006-01-02"),
		Entries: entries,
	}

	tmpl, err := template.New("versions").Parse(versionsGenTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		_ = os.WriteFile(path+".debug", buf.Bytes(), 0644)
		return fmt.Errorf("formatting generated code: %w (debug written to %s.debug)", err, path)
	}

	return os.WriteFile(path, formatted, 0644)
}
