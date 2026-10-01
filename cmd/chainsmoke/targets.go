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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

const (
	versionsGenPath = "internal/adapters/versions_gen.go"
	chainTypesPath  = "api/v1alpha2/chaininstance_types.go"
)

// imageOverride replaces the image of one chain (every client) or of one
// chain/client pair, so a major bump can be smoke-tested before it lands in
// versions_gen.go.
type imageOverride struct {
	chain  chainsv1alpha2.Chain
	client string // lowercase; "" applies to every client of the chain
	ref    string
}

// parseOverride parses "chain[/client]=ref".
func parseOverride(s string) (imageOverride, error) {
	key, ref, ok := strings.Cut(s, "=")
	key, ref = strings.TrimSpace(key), strings.TrimSpace(ref)
	if !ok || key == "" || ref == "" {
		return imageOverride{}, fmt.Errorf("invalid --image %q: want chain[/client]=ref", s)
	}
	chain, client, _ := strings.Cut(key, "/")
	if chain == "" {
		return imageOverride{}, fmt.Errorf("invalid --image %q: empty chain", s)
	}
	return imageOverride{
		chain:  chainsv1alpha2.Chain(chain),
		client: strings.ToLower(client),
		ref:    ref,
	}, nil
}

// matches reports whether the override applies to a chain/client pair.
func (o imageOverride) matches(chain chainsv1alpha2.Chain, client string) bool {
	return o.chain == chain && (o.client == "" || o.client == strings.ToLower(client))
}

// overrideList implements flag.Value for the repeatable --image flag.
type overrideList []imageOverride

func (l *overrideList) String() string {
	parts := make([]string, 0, len(*l))
	for _, o := range *l {
		key := string(o.chain)
		if o.client != "" {
			key += "/" + o.client
		}
		parts = append(parts, key+"="+o.ref)
	}
	return strings.Join(parts, ",")
}

func (l *overrideList) Set(s string) error {
	o, err := parseOverride(s)
	if err != nil {
		return err
	}
	*l = append(*l, o)
	return nil
}

// lookup returns the last override matching chain/client.
func (l overrideList) lookup(chain chainsv1alpha2.Chain, client string) (string, bool) {
	for _, o := range slices.Backward(l) {
		if o.matches(chain, client) {
			return o.ref, true
		}
	}
	return "", false
}

// target is one chain/client/image row checked by level 0.
type target struct {
	chain  chainsv1alpha2.Chain
	client string
	image  string
}

// level0Targets flattens the default image table into sorted targets, applies
// overrides (an override naming a client absent from the table adds a row)
// and keeps only chains accepted by selected.
func level0Targets(
	images map[chainsv1alpha2.Chain]map[string]string,
	overrides overrideList,
	selected func(chainsv1alpha2.Chain) bool,
) []target {
	for _, o := range overrides {
		if o.client == "" {
			continue
		}
		if images[o.chain] == nil {
			images[o.chain] = map[string]string{}
		}
		if _, ok := images[o.chain][o.client]; !ok {
			images[o.chain][o.client] = o.ref
		}
	}

	var out []target
	for chain, clients := range images {
		if !selected(chain) {
			continue
		}
		for client, image := range clients {
			if ref, ok := overrides.lookup(chain, client); ok {
				image = ref
			}
			out = append(out, target{chain: chain, client: client, image: image})
		}
	}
	slices.SortFunc(out, func(a, b target) int {
		return cmp.Or(cmp.Compare(a.chain, b.chain), cmp.Compare(a.client, b.client))
	})
	return out
}

// changedSince returns the chains whose default image table entries differ
// between versions_gen.go at the given git ref and the compiled-in table.
func changedSince(
	repoRoot, ref string,
	current map[chainsv1alpha2.Chain]map[string]string,
) (map[chainsv1alpha2.Chain]bool, error) {
	oldSrc, err := exec.Command("git", "-C", repoRoot, "show", ref+":"+versionsGenPath).Output()
	if err != nil {
		return nil, fmt.Errorf("--changed-since: reading %s at %s: %w", versionsGenPath, ref, err)
	}
	oldByConst, err := parseImageTable(oldSrc)
	if err != nil {
		return nil, fmt.Errorf("--changed-since: parsing %s at %s: %w", versionsGenPath, ref, err)
	}
	typesSrc, err := os.ReadFile(filepath.Join(repoRoot, chainTypesPath))
	if err != nil {
		return nil, fmt.Errorf("--changed-since: %w", err)
	}
	consts, err := parseChainConsts(typesSrc)
	if err != nil {
		return nil, fmt.Errorf("--changed-since: parsing %s: %w", chainTypesPath, err)
	}

	old := make(map[chainsv1alpha2.Chain]map[string]string, len(oldByConst))
	for name, clients := range oldByConst {
		chain, ok := consts[name]
		if !ok {
			continue // chain removed from the API since ref
		}
		old[chain] = clients
	}
	return diffImageTables(old, current), nil
}

// diffImageTables returns the chains with at least one client whose image is
// new or different in cur compared to old.
func diffImageTables(old, cur map[chainsv1alpha2.Chain]map[string]string) map[chainsv1alpha2.Chain]bool {
	changed := map[chainsv1alpha2.Chain]bool{}
	for chain, clients := range cur {
		for client, image := range clients {
			if old[chain][client] != image {
				changed[chain] = true
			}
		}
	}
	return changed
}

// parseImageTable extracts the chainDefaultImages literal from versions_gen.go
// source, keyed by the Go constant name of the chain (e.g. "ChainBitcoin").
func parseImageTable(src []byte) (map[string]map[string]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), versionsGenPath, src, 0)
	if err != nil {
		return nil, err
	}

	var table *ast.CompositeLit
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || spec.Names[0].Name != "chainDefaultImages" || len(spec.Values) != 1 {
			return table == nil
		}
		table, _ = spec.Values[0].(*ast.CompositeLit)
		return false
	})
	if table == nil {
		return nil, fmt.Errorf("chainDefaultImages literal not found")
	}

	out := make(map[string]map[string]string, len(table.Elts))
	for _, elt := range table.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		sel, ok := kv.Key.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		inner, ok := kv.Value.(*ast.CompositeLit)
		if !ok {
			continue
		}
		clients := map[string]string{}
		for _, e := range inner.Elts {
			ckv, ok := e.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			client, err1 := stringLit(ckv.Key)
			image, err2 := stringLit(ckv.Value)
			if err1 != nil || err2 != nil {
				continue
			}
			clients[client] = image
		}
		out[sel.Sel.Name] = clients
	}
	return out, nil
}

// parseChainConsts maps Chain constant names to their string values, e.g.
// "ChainBitTorrent" -> "bittorrent".
func parseChainConsts(src []byte) (map[string]chainsv1alpha2.Chain, error) {
	file, err := parser.ParseFile(token.NewFileSet(), chainTypesPath, src, 0)
	if err != nil {
		return nil, err
	}
	out := map[string]chainsv1alpha2.Chain{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, s := range gen.Specs {
			spec := s.(*ast.ValueSpec)
			if len(spec.Names) != 1 || len(spec.Values) != 1 || !strings.HasPrefix(spec.Names[0].Name, "Chain") {
				continue
			}
			v, err := stringLit(spec.Values[0])
			if err != nil {
				continue
			}
			out[spec.Names[0].Name] = chainsv1alpha2.Chain(v)
		}
	}
	return out, nil
}

func stringLit(e ast.Expr) (string, error) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", fmt.Errorf("not a string literal")
	}
	return strconv.Unquote(lit.Value)
}
