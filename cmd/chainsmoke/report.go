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
	"slices"
	"strings"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

const (
	statusPass = "PASS"
	statusWarn = "WARN"
	statusFail = "FAIL"
	statusSkip = "SKIP"
)

// detailWidth caps the detail column so the table stays readable.
const detailWidth = 160

// result is one report row.
type result struct {
	chain  chainsv1alpha2.Chain
	client string
	image  string
	level  int
	status string
	detail string
	notes  []string
}

func anyFailed(results []result) bool {
	return slices.ContainsFunc(results, func(r result) bool { return r.status == statusFail })
}

// renderReport renders results as a markdown table sorted by chain and client,
// preceded by per-status totals.
func renderReport(results []result) string {
	rows := slices.Clone(results)
	slices.SortStableFunc(rows, func(a, b result) int {
		return cmp.Or(cmp.Compare(a.chain, b.chain), cmp.Compare(a.client, b.client))
	})

	counts := map[string]int{}
	for _, r := range rows {
		counts[r.status]++
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# chainsmoke report\n\n%d checked: %d PASS, %d WARN, %d FAIL, %d SKIP\n\n",
		len(rows), counts[statusPass], counts[statusWarn], counts[statusFail], counts[statusSkip])
	b.WriteString("| chain | client | image | level | result | detail |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, r := range rows {
		detail := r.detail
		if len(r.notes) > 0 {
			detail += " [" + strings.Join(r.notes, "; ") + "]"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %s | %s |\n",
			cell(string(r.chain)), cell(clientLabel(r.client)), cell(r.image), r.level, r.status,
			cell(truncate(detail, detailWidth)))
	}
	return b.String()
}

// clientLabel names the default client column entry.
func clientLabel(client string) string {
	if client == "" {
		return "(default)"
	}
	return client
}

// cell makes s safe for a markdown table cell.
func cell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(s, "|", `\|`)
}

// truncate shortens s to at most n runes, marking the cut.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
