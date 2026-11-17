// Package scopedset computes the packages a rebased lane can break and judges each red outside them.
// See docs/architecture/packages/internal-scopedset.md.
package scopedset

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
)

const (
	moduleRootPattern = "./..."
	enforceListPath   = "go/.apicover-enforce"
	treeReadPrefix    = "../"
)

type Set struct {
	All      bool
	Reason   string
	Packages []string
	module   string
}

type Escape struct {
	Gate    string `json:"gate"`
	Package string `json:"package,omitempty"`
	Test    string `json:"test,omitempty"`
}

func Compute(repoRoot string, lane, main []string) Set {
	if repoRoot == "" {
		return everything("the repository root is unknown")
	}
	for _, file := range append(append([]string{}, lane...), main...) {
		if file == enforceListPath {
			continue
		}
		if pattern, ok := changedpkgs.PatternOf(file); ok && pattern == moduleRootPattern {
			return everything("a module-root file changed: " + file)
		}
	}
	module, ok := modulePath(filepath.Join(repoRoot, "go"))
	if !ok {
		return everything("the module path cannot be read")
	}
	var closures [2][]string
	for i, files := range [2][]string{lane, main} {
		closure, ok := closureOf(repoRoot, files)
		if !ok {
			return everything("the import graph of the module cannot be read")
		}
		closures[i] = closure
	}
	packages := overlap(closures[0], closures[1])
	packages = append(packages, treeReaders(repoRoot, append(append([]string{}, lane...), main...))...)
	return Set{Packages: sortedUnique(packages), module: module}
}

func Escapes(set Set, gate string, reds baseattr.Reds) []Escape {
	if set.All {
		return nil
	}
	var out []Escape
	for _, key := range reds.Keys {
		if !set.holds(key.Package) {
			out = append(out, Escape{Gate: gate, Package: key.Package, Test: key.Test})
		}
	}
	if reds.Unnamed {
		out = append(out, Escape{Gate: gate})
	}
	return out
}

func everything(reason string) Set {
	return Set{All: true, Reason: reason}
}

func (s Set) holds(importPath string) bool {
	rel := strings.TrimPrefix(importPath, s.module+"/")
	if rel == importPath {
		return false
	}
	for _, pattern := range s.Packages {
		if covers(pattern, "./"+rel+"/...") {
			return true
		}
	}
	return false
}

func closureOf(repoRoot string, files []string) ([]string, bool) {
	var patterns []string
	for _, file := range files {
		if file == enforceListPath {
			continue
		}
		if pattern, ok := changedpkgs.PatternOf(file); ok {
			patterns = append(patterns, pattern)
		}
	}
	closure, ok := changedpkgs.ImporterClosureChecked(repoRoot, patterns)
	return closure.Patterns, ok
}

func overlap(a, b []string) []string {
	var out []string
	for _, p := range a {
		for _, q := range b {
			switch {
			case covers(p, q):
				out = append(out, q)
			case covers(q, p):
				out = append(out, p)
			}
		}
	}
	return out
}

func covers(outer, inner string) bool {
	dir := strings.TrimSuffix(outer, "/...")
	target := strings.TrimSuffix(inner, "/...")
	return dir == "." || target == dir || strings.HasPrefix(target, dir+"/")
}

func treeReaders(repoRoot string, changed []string) []string {
	if len(changed) == 0 {
		return nil
	}
	var packages []string
	fset := token.NewFileSet()
	walkRoot := filepath.Join(repoRoot, "go")
	_ = filepath.WalkDir(walkRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if name := d.Name(); p != walkRoot && (name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, p)
		dir := path.Dir(filepath.ToSlash(rel))
		if readsAny(fset, p, dir, changed) {
			if pattern, ok := changedpkgs.PatternOf(filepath.ToSlash(rel)); ok {
				packages = append(packages, pattern)
			}
		}
		return nil
	})
	return packages
}

func readsAny(fset *token.FileSet, file, dir string, changed []string) bool {
	parsed, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
	if err != nil {
		return false
	}
	found := false
	ast.Inspect(parsed, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if found || !ok || lit.Kind != token.STRING {
			return !found
		}
		value, _ := strconv.Unquote(lit.Value)
		if !strings.HasPrefix(value, treeReadPrefix) {
			return true
		}
		resolved := path.Join(dir, value)
		for _, c := range changed {
			if resolved == "." || c == resolved || strings.HasPrefix(c, resolved+"/") {
				found = true
				break
			}
		}
		return !found
	})
	return found
}

func modulePath(moduleDir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(moduleDir, "go.mod"))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module"); ok && strings.TrimSpace(rest) != "" {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

func sortedUnique(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if _, dup := seen[item]; !dup {
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}
