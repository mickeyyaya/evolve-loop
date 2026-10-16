package changedpkgs

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const importerClosureTimeout = 120 * time.Second

const moduleRootPattern = "./..."

type Closure struct {
	Patterns []string
	Testable []string
}

func ImporterClosure(repoRoot string, pkgs []string) []string {
	c, _ := ImporterClosureChecked(repoRoot, pkgs)
	return c.Patterns
}

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
			return Closure{Patterns: in, Testable: in}, true
		}
	}
	targets := targetPrefixes(in)
	if len(targets) == 0 {
		return Closure{Patterns: in}, true
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

type listedPkg struct {
	path           string
	transitiveDeps []string
	testImports    []string
	testable       bool
}

func (p listedPkg) links(hit func(string) bool) bool {
	for _, d := range p.transitiveDeps {
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

func listModule(ctx context.Context, moduleDir string) (map[string]listedPkg, error) {
	out, err := sysexec.Output(ctx, sysexec.DefaultRunner, moduleDir, "go", "list", "-e", "-f",
		`{{.ImportPath}}|{{join .Deps " "}}|{{join .TestImports " "}} {{join .XTestImports " "}}|{{len .GoFiles}} {{len .TestGoFiles}} {{len .XTestGoFiles}}`,
		"./...")
	if err != nil {
		return nil, err
	}
	listing := map[string]listedPkg{}
	for _, line := range strings.Split(out, "\n") {
		if pkg, ok := parseListedPkg(line); ok {
			listing[pkg.path] = pkg
		}
	}
	return listing, nil
}

func parseListedPkg(line string) (listedPkg, bool) {
	parts := strings.Split(line, "|")
	if len(parts) != 4 || strings.TrimSpace(parts[0]) == "" {
		return listedPkg{}, false
	}
	return listedPkg{
		path:           parts[0],
		transitiveDeps: strings.Fields(parts[1]),
		testImports:    strings.Fields(parts[2]),
		testable:       strings.TrimSpace(parts[3]) != "0 0 0",
	}, true
}

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

func matchesAny(importPath string, targets []string, modPath string) bool {
	rel := strings.TrimPrefix(importPath, modPath+"/")
	if rel == importPath {
		return false
	}
	for _, t := range targets {
		if rel == t || strings.HasPrefix(rel, t+"/") {
			return true
		}
	}
	return false
}

func patternFor(importPath, modPath string) string {
	rel := strings.TrimPrefix(importPath, modPath+"/")
	if rel == importPath || rel == "" {
		return moduleRootPattern
	}
	return "./" + rel + "/..."
}

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
