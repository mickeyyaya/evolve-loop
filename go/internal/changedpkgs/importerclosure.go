package changedpkgs

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// importerClosureTimeout bounds the two `go list` invocations. A cold module
// cache makes the dep-graph load the slowest thing this package does; past the
// bound we fall back to the input set rather than hang a predicate.
const importerClosureTimeout = 120 * time.Second

// moduleRootPattern is the go test pattern covering every package in the
// module. Its closure is the identity — it already selects everything.
const moduleRootPattern = "./..."

// ImporterClosure widens a changed-package set with its REVERSE dependencies:
// the sorted, deduped union of pkgs and a "./dir/..." pattern for every module
// package that transitively imports one of them.
//
// Every other derivation in this package is forward-only — FileToPackage maps a
// changed file to the package it lives in, and nothing walks the import graph.
// So a change confined to internal/router never selects internal/routingtest,
// even though routingtest imports router and holds the keystone parity
// invariant: exactly the cycle-1250 miss that kept main red for 5 commits.
// Test-impact selection built on a forward-only set silently hides that whole
// regression class.
//
// repoRoot is the REPOSITORY root (the dir containing the go/ module dir), the
// same parameter meaning as FromGit/FromGitChecked. pkgs are "./dir/..."
// patterns as emitted by FileToPackage.
//
// Best-effort, like the rest of this package: an empty or nonexistent repoRoot,
// a junk pattern, or any `go list` failure yields the input set unchanged (an
// EMPTY added closure) — never an error, never a panic, never a lost input
// entry. Closure only ever widens; narrowing below the forward-only baseline
// would be strictly worse than not having this function at all.
func ImporterClosure(repoRoot string, pkgs []string) []string {
	if len(pkgs) == 0 {
		return nil
	}
	in := sortedDedup(pkgs)
	if repoRoot == "" {
		return in
	}
	for _, p := range in {
		if p == moduleRootPattern {
			return in // already selects every package; nothing to widen
		}
	}
	targets := targetPrefixes(in)
	if len(targets) == 0 {
		return in // no parseable pattern to trace importers of
	}

	ctx, cancel := context.WithTimeout(context.Background(), importerClosureTimeout)
	defer cancel()
	goDir := repoRoot + "/go"

	modPath, err := sysexec.Output(ctx, sysexec.DefaultRunner, goDir, "go", "list", "-m")
	if err != nil || modPath == "" {
		return in
	}
	// -e keeps a single unloadable package from failing the whole listing; .Deps
	// is already transitive, so one pass yields the transitive closure.
	out, err := sysexec.Output(ctx, sysexec.DefaultRunner, goDir, "go", "list", "-e",
		"-f", "{{.ImportPath}} {{join .Deps \" \"}}", "./...")
	if err != nil {
		return in
	}

	set := map[string]struct{}{}
	for _, p := range in {
		set[p] = struct{}{}
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		self := fields[0]
		if matchesAny(self, targets, modPath) {
			continue // inside an input pattern already
		}
		for _, dep := range fields[1:] {
			if matchesAny(dep, targets, modPath) {
				set[patternFor(self, modPath)] = struct{}{}
				break
			}
		}
	}
	res := make([]string, 0, len(set))
	for p := range set {
		res = append(res, p)
	}
	sort.Strings(res)
	return res
}

// targetPrefixes converts "./dir/..." patterns into module-relative directory
// prefixes. Unparseable patterns are skipped: they contribute no closure but
// still survive in the output, per the never-drop-an-input contract.
func targetPrefixes(pkgs []string) []string {
	var out []string
	for _, p := range pkgs {
		d := strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/...")
		if d == "" || d == p || strings.HasPrefix(d, ".") {
			continue
		}
		out = append(out, d)
	}
	return out
}

// matchesAny reports whether importPath (a full module import path) lies inside
// any of the module-relative target dirs.
func matchesAny(importPath string, targets []string, modPath string) bool {
	rel := strings.TrimPrefix(importPath, modPath+"/")
	if rel == importPath {
		return false // outside the module (stdlib or an external dep)
	}
	for _, t := range targets {
		if rel == t || strings.HasPrefix(rel, t+"/") {
			return true
		}
	}
	return false
}

// patternFor maps a module import path back to its go test pattern.
func patternFor(importPath, modPath string) string {
	rel := strings.TrimPrefix(importPath, modPath+"/")
	if rel == importPath || rel == "" {
		return moduleRootPattern
	}
	return "./" + rel + "/..."
}

// sortedDedup returns pkgs sorted and deduped without mutating the input, so
// the degenerate-input fallbacks share the shape contract of the real result.
func sortedDedup(pkgs []string) []string {
	set := map[string]struct{}{}
	for _, p := range pkgs {
		set[p] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
