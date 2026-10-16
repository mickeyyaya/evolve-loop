// Package reachabilityprobe flags a frozen structural-test pin that could only
// be satisfied by closing an import cycle.
// See docs/architecture/packages/internal-reachabilityprobe.md.
package reachabilityprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type ImportGraph map[string][]string

type listedPackage struct {
	ImportPath string
	Imports    []string
}

func BuildImportGraph(repoRoot string, pkgs ...string) (ImportGraph, error) {
	args := append([]string{"list", "-deps", "-json"}, pkgs...)
	out, err := sysexec.Output(context.Background(), sysexec.DefaultRunner, repoRoot, "go", args...)
	if err != nil {
		return nil, fmt.Errorf("reachabilityprobe: go list -deps -json %s: %w", strings.Join(pkgs, " "), err)
	}

	graph := ImportGraph{}
	dec := json.NewDecoder(strings.NewReader(out))
	for dec.More() {
		var pkg listedPackage
		if err := dec.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("reachabilityprobe: decoding go list -deps -json output: %w", err)
		}
		graph[pkg.ImportPath] = pkg.Imports
	}
	return graph, nil
}

type CallSite struct {
	PinningPackage    string
	ReferencedPackage string
	Symbol            string
}

type Violation struct {
	Site  CallSite
	Cycle []string
}

func (v *Violation) Error() string {
	return fmt.Sprintf("pinning %s.%s( inside package %q would create an import cycle: %s -> %s",
		v.Site.ReferencedPackage, v.Site.Symbol, v.Site.PinningPackage, pathString(v.Cycle), v.Site.ReferencedPackage)
}

func pathString(chain []string) string {
	out := ""
	for i, pkg := range chain {
		if i > 0 {
			out += " -> "
		}
		out += pkg
	}
	return out
}

func CheckCallSite(graph ImportGraph, site CallSite) *Violation {
	if _, known := graph[site.PinningPackage]; !known {
		return nil
	}
	if chain, ok := findImportChain(graph, site.ReferencedPackage, site.PinningPackage); ok {
		return &Violation{Site: site, Cycle: chain}
	}
	return nil
}

func findImportChain(graph ImportGraph, start, target string) ([]string, bool) {
	if start == target {
		return []string{start}, true
	}

	type frame struct {
		pkg  string
		path []string
	}
	visited := map[string]bool{start: true}
	queue := []frame{{pkg: start, path: []string{start}}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, next := range graph[cur.pkg] {
			if next == target {
				return append(append([]string{}, cur.path...), next), true
			}
			if visited[next] {
				continue
			}
			visited[next] = true
			nextPath := append(append([]string{}, cur.path...), next)
			queue = append(queue, frame{pkg: next, path: nextPath})
		}
	}
	return nil, false
}
