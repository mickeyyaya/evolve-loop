// Package reachabilityprobe is a deterministic compiler-probe check for the
// TDD structural-test-freeze step (inbox
// tdd-structural-test-reachability-probe, weight 0.92, root cause cycle-644).
//
// Cycle-644 froze a `doNotModifyTests:true` structural test that pinned
// `storage.UpdateStateMap(` inside a `core`-package file, while `storage`
// already imported `core` — a compiler-proven import cycle. The acceptance
// criterion was permanently unsatisfiable and burned the whole cycle before
// anyone noticed the shape was unbuildable.
//
// CheckCallSite answers, from a package import graph alone (no `go build`
// invocation required — the caller supplies the graph, typically derived from
// `go list -deps`), whether pinning a package-qualified call site would
// introduce exactly that shape: the referenced package already (transitively)
// imports the pinning package, so the pinning package importing the referenced
// package back would be an import cycle.
package reachabilityprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// ImportGraph maps a package name to the packages it directly imports. It is
// the caller's responsibility to build this from the real toolchain (e.g.
// `go list -deps`); this package only walks the graph it is given.
type ImportGraph map[string][]string

// listedPackage is the subset of `go list -json` output BuildImportGraph
// needs: the package's own import path and its direct imports.
type listedPackage struct {
	ImportPath string
	Imports    []string
}

// BuildImportGraph derives an ImportGraph from the real toolchain by shelling
// out to `go list -deps -json` for pkgs (package patterns such as
// "./internal/fleet"), scoped to the Go module rooted at repoRoot (the
// directory containing go.mod). It returns the transitive closure's direct
// import edges for every package reached, matching the shape CheckCallSite
// already consumes, or a wrapped error if the toolchain invocation fails
// (unresolvable package pattern, build errors, etc).
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

// CallSite describes a structural test's frozen pin: a call to
// ReferencedPackage.Symbol( written inside a file belonging to
// PinningPackage.
type CallSite struct {
	PinningPackage    string
	ReferencedPackage string
	Symbol            string
}

// Violation reports that pinning Site would require PinningPackage to import
// ReferencedPackage while ReferencedPackage already transitively imports
// PinningPackage — an unbuildable cycle. Cycle is the import chain from
// ReferencedPackage back to PinningPackage that proves it, in traversal order
// (e.g. ["storage", "mid", "core"] for storage -> mid -> core).
type Violation struct {
	Site  CallSite
	Cycle []string
}

// Error implements the error interface so a Violation can be surfaced
// directly as a build/test failure reason.
func (v *Violation) Error() string {
	return fmt.Sprintf("pinning %s.%s( inside package %q would create an import cycle: %s -> %s",
		v.Site.ReferencedPackage, v.Site.Symbol, v.Site.PinningPackage, pathString(v.Cycle), v.Site.PinningPackage)
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

// CheckCallSite reports whether pinning site inside a file belonging to
// site.PinningPackage would introduce an import cycle, given graph as the
// package import graph. It returns a non-nil *Violation carrying the proving
// import chain when site.ReferencedPackage transitively imports
// site.PinningPackage (directly or through intermediate packages); it returns
// nil when no such path exists, including when site.PinningPackage is absent
// from graph entirely (absence of evidence is not evidence of a cycle).
func CheckCallSite(graph ImportGraph, site CallSite) *Violation {
	if _, known := graph[site.PinningPackage]; !known {
		return nil
	}
	if chain, ok := findImportChain(graph, site.ReferencedPackage, site.PinningPackage); ok {
		return &Violation{Site: site, Cycle: chain}
	}
	return nil
}

// findImportChain performs a breadth-first search over graph for a path from
// start to target following import edges (start -> ... -> target), returning
// the path (inclusive of both ends) and true when one exists.
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
