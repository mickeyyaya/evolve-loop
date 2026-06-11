// Package flagregistry is the declarative SSOT for every EVOLVE_* control
// flag across all reader surfaces (Go production code, Go test seams, and
// the bash skill/agent/commit-gate surface). It exists to end the
// 252-actual-vs-93-documented drift (L2, concurrency-factory plan):
// `evolve flags generate` projects the registry into the marker region of
// docs/architecture/control-flags.md and `evolve flags check` fails on
// drift, so a flag can no longer ship undocumented.
//
// Metadata ONLY — the registry never funnels env reads through config.Load:
// subprocess-reads-env is a deliberate architecture property (the bridge
// subprocess and bash adapters read their own env).
//
// registry_table.go (the data) was seeded mechanically from the 2026-06-11
// inventory (grep over go/ + agents/ + skills/ + commit-gate/ + legacy/ +
// control-flags.md) and is maintained by hand from then on — add a row when
// you add a flag; the drift test (L2.3) catches omissions.
package flagregistry

import (
	"fmt"
	"sort"
	"strings"
)

// Status is a flag's lifecycle state.
type Status string

const (
	// StatusActive — read in production code and operator-facing; do not
	// remove without a deprecation window.
	StatusActive Status = "active"
	// StatusDeprecated — still honored (often via a bridge that maps it to
	// its replacement) but emits a WARN; scheduled for removal.
	StatusDeprecated Status = "deprecated"
	// StatusDead — no reader on any surface; safe to delete mentions.
	StatusDead Status = "dead"
	// StatusInternal — read by production code but set by the runner itself
	// (subprocess injection / plumbing); not an operator dial.
	StatusInternal Status = "internal"
	// StatusTestSeam — read only by _test.go files; never set in production.
	StatusTestSeam Status = "test-seam"
)

// Flag is one registry row. Kind/Default are optional metadata (empty =
// unspecified); Cluster mirrors the control-flags.md section the flag is
// documented under.
type Flag struct {
	Name       string
	Status     Status
	Kind       string // bool | int | string | enum | path | usd ("" = unspecified)
	Default    string
	Cluster    string
	Doc        string
	ReplacedBy string
	RemoveIn   string
}

// Lookup returns the registry row for name.
func Lookup(name string) (Flag, bool) {
	i := sort.Search(len(All), func(i int) bool { return All[i].Name >= name })
	if i < len(All) && All[i].Name == name {
		return All[i], true
	}
	return Flag{}, false
}

// RenderIndex renders the full registry as the markdown table projected
// into control-flags.md's generated marker region. Deterministic: All is
// sorted and the renderer is pure.
func RenderIndex() string {
	var b strings.Builder
	b.WriteString("Complete flag index — generated from `go/internal/flagregistry` (SSOT). Edit the registry, then run `evolve flags generate`; do not edit this table by hand.\n\n")
	b.WriteString("| Flag | Status | Kind | Default | Cluster | Purpose |\n")
	b.WriteString("|------|--------|------|---------|---------|----------|\n")
	for _, f := range All {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s |\n",
			f.Name, f.Status, cell(f.Kind), cell(f.Default), cell(f.Cluster), renderDoc(f))
	}
	return b.String()
}

// cell makes one table cell GFM-safe (pipes escaped, newlines collapsed);
// empty renders as an em-dash.
func cell(s string) string {
	if s == "" {
		return "—"
	}
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
}

// renderDoc folds ReplacedBy/RemoveIn into the purpose column and keeps the
// cell table-safe.
func renderDoc(f Flag) string {
	doc := f.Doc
	if f.ReplacedBy != "" {
		doc += " Replaced by `" + f.ReplacedBy + "`."
	}
	if f.RemoveIn != "" {
		doc += " Remove in " + f.RemoveIn + "."
	}
	return strings.TrimSpace(cell(doc))
}
