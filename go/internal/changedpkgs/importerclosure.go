package changedpkgs

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// importerClosureTimeout bounds `go list` on a cold module cache; past it the input set is returned rather than hang a predicate.
const importerClosureTimeout = 120 * time.Second

const moduleRootPattern = "./..."

// Closure is one reverse-dependency walk over the module's import graph.
type Closure struct {
	// Patterns is the sorted union of the inputs and every package whose build or test binary transitively links one.
	Patterns []string
	// Testable is the subset of Patterns that `go test` can run without tags; a tag-only or removed dir is left out.
	Testable []string
}

// ImporterClosure returns pkgs widened by every module package that transitively imports one; any failure returns pkgs unchanged.
func ImporterClosure(repoRoot string, pkgs []string) []string {
	c, _ := ImporterClosureChecked(repoRoot, pkgs)
	return c.Patterns
}

// ImporterClosureChecked is ImporterClosure plus Testable; ok is false when the module could not be listed.
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
			continue
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

// links reports whether pkg's transitive build deps or its tests' direct imports name a package hit accepts.
// A test import counts one hop: following its deps would pull every user of a shared fixture into every closure.
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

// listModule runs one `go list -e` over the module; -e keeps one unloadable package from failing the whole listing.
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

// testableUnder keeps the patterns under which a listed package has default-context Go files, in input order.
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

// targetPrefixes converts "./dir/..." patterns into module-relative dirs; an unparseable one adds no closure but stays in the output.
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

// sortedDedup returns a sorted, deduped copy of pkgs, so the fallbacks share the real result's shape.
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
