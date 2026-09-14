package changedpkgs

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// importerClosureTimeout bounds the `go list` invocation. A cold module cache
// makes the graph load the slowest thing this package does; past the bound we
// fall back to the input set rather than hang a predicate.
const importerClosureTimeout = 120 * time.Second

// moduleRootPattern is the go test pattern covering every package in the
// module. Its closure is the identity — it already selects everything.
const moduleRootPattern = "./..."

// Closure is one reverse-dependency walk over the module's import graph.
type Closure struct {
	// Patterns is the sorted, deduped union of the input patterns and a
	// "./dir/..." pattern for every module package whose build OR TEST binary
	// transitively links one of them. Inputs are never dropped.
	Patterns []string
	// Testable is the subset of Patterns under which `go list` finds at least
	// one package with Go files in the default build context — what `go test`
	// can run without tags. A tag-only package (`//go:build acs`) or a removed
	// directory is in Patterns (its importers are exactly what break) but not
	// here: naming it to `go test` is "matched no packages", exit 1.
	Testable []string
}

// ImporterClosure widens a changed-package set with its REVERSE dependencies:
// the sorted, deduped union of pkgs and a "./dir/..." pattern for every module
// package that transitively imports one of them — through its build
// dependencies or through the imports of its own tests (an untouched
// `_test.go` asserting a changed package's contract is the 2026-09-14 ship-gate
// incident; a routingtest that imports router is the cycle-1250 one).
//
// Every other derivation in this package is forward-only — FileToPackage maps a
// changed file to the package it lives in, and nothing walks the import graph.
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
// would be strictly worse than not having this function at all. Callers that
// must distinguish "nothing imports it" from "go list failed" use
// ImporterClosureChecked.
func ImporterClosure(repoRoot string, pkgs []string) []string {
	c, _ := ImporterClosureChecked(repoRoot, pkgs)
	return c.Patterns
}

// ImporterClosureChecked is ImporterClosure plus the derivability signal and
// the testable projection: ok is false when the module could not be listed
// (no module, `go list` failed or timed out) — Patterns is then the input set
// and Testable is empty.
func ImporterClosureChecked(repoRoot string, pkgs []string) (Closure, bool) {
	if len(pkgs) == 0 {
		return Closure{}, true
	}
	in := sortedDedup(pkgs)
	if repoRoot == "" {
		return Closure{Patterns: in}, false
	}
	for _, p := range in {
		if p == moduleRootPattern {
			return Closure{Patterns: in, Testable: in}, true // already selects every package
		}
	}
	targets := targetPrefixes(in)
	if len(targets) == 0 {
		return Closure{Patterns: in}, true // no parseable pattern to trace importers of
	}
	moduleDir := filepath.Join(repoRoot, "go")
	modPath, ok := modulePathOf(moduleDir)
	if !ok {
		return Closure{Patterns: in}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), importerClosureTimeout)
	defer cancel()
	listing, err := listModule(ctx, moduleDir)
	if err != nil {
		return Closure{Patterns: in}, false
	}
	set := map[string]struct{}{}
	for _, p := range in {
		set[p] = struct{}{}
	}
	hit := func(dep string) bool { return matchesAny(dep, targets, modPath) }
	for _, pkg := range listing {
		if matchesAny(pkg.path, targets, modPath) {
			continue // inside an input pattern already
		}
		if pkg.links(hit) {
			set[patternFor(pkg.path, modPath)] = struct{}{}
		}
	}
	patterns := make([]string, 0, len(set))
	for p := range set {
		patterns = append(patterns, p)
	}
	sort.Strings(patterns)
	return Closure{Patterns: patterns, Testable: testableUnder(patterns, listing, modPath)}, true
}

// listedPkg is what one `go list` line says about a module package.
type listedPkg struct {
	path        string
	deps        []string // transitive build dependencies (.Deps)
	testImports []string // direct imports of its _test.go files (.TestImports + .XTestImports)
	testable    bool     // has Go files in the default build context
}

// links reports whether pkg's build deps (transitive) or its tests' DIRECT
// imports name a package hit accepts. A test's imports count one hop on
// purpose: the helper it imports is in the closure through its own deps, and
// that helper's tests are where its contract is asserted — following a test
// import's deps would pull every package whose tests use a shared fixture
// (test/fixtures links core) into every closure, and selection would be
// `./...` by another name.
func (p listedPkg) links(hit func(string) bool) bool {
	for _, d := range p.deps {
		if hit(d) {
			return true
		}
	}
	for _, t := range p.testImports {
		if hit(t) {
			return true
		}
	}
	return false
}

// listModule runs one `go list` over the module. -e keeps a single unloadable
// package from failing the whole listing; .Deps is already transitive.
func listModule(ctx context.Context, moduleDir string) (map[string]listedPkg, error) {
	out, err := sysexec.Output(ctx, sysexec.DefaultRunner, moduleDir, "go", "list", "-e", "-f",
		`{{.ImportPath}}|{{join .Deps " "}}|{{join .TestImports " "}} {{join .XTestImports " "}}|{{len .GoFiles}} {{len .TestGoFiles}} {{len .XTestGoFiles}}`,
		"./...")
	if err != nil {
		return nil, err
	}
	listing := map[string]listedPkg{}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "|")
		if len(parts) != 4 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		listing[parts[0]] = listedPkg{
			path:        parts[0],
			deps:        strings.Fields(parts[1]),
			testImports: strings.Fields(parts[2]),
			testable:    strings.TrimSpace(parts[3]) != "0 0 0",
		}
	}
	return listing, nil
}

// testableUnder keeps the patterns under which at least one listed package has
// default-context Go files. patterns is sorted, so the result is too.
func testableUnder(patterns []string, listing map[string]listedPkg, modPath string) []string {
	var out []string
	for _, pat := range patterns {
		prefix := strings.TrimSuffix(strings.TrimPrefix(pat, "./"), "/...")
		for _, pkg := range listing {
			if !pkg.testable {
				continue
			}
			rel := strings.TrimPrefix(pkg.path, modPath+"/")
			if pkg.path == modPath {
				rel = ""
			}
			if prefix == "" || rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				out = append(out, pat)
				break
			}
		}
	}
	return out
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
