// Package flagregistry is the declarative SSOT for every EVOLVE_* control
// flag across every reader surface — Go, Go tests, and the bash adapters.
// See docs/architecture/packages/internal-flagregistry.md.
package flagregistry

import (
	"fmt"
	"sort"
	"strings"
)

type Status string

const (
	StatusActive     Status = "active"
	StatusDeprecated Status = "deprecated"
	StatusDead       Status = "dead"
	StatusInternal   Status = "internal"
	StatusTestSeam   Status = "test-seam"
)

type Flag struct {
	Name       string
	Status     Status
	Kind       string
	Default    string
	Cluster    string
	Doc        string
	ReplacedBy string
	RemoveIn   string
}

const ClusterCoreInfra = "Core Infrastructure (never consolidate)"

func IsCoreInfra(f Flag) bool { return f.Cluster == ClusterCoreInfra }

func LiveFeatureFlags() []Flag {
	out := make([]Flag, 0, len(All))
	for _, f := range All {
		if f.Status == StatusActive && !IsCoreInfra(f) {
			out = append(out, f)
		}
	}
	return out
}

func Lookup(name string) (Flag, bool) {
	i := sort.Search(len(All), func(i int) bool { return All[i].Name >= name })
	if i < len(All) && All[i].Name == name {
		return All[i], true
	}
	return Flag{}, false
}

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

func cell(s string) string {
	if s == "" {
		return "—"
	}
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
}

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
