package changedpkgs

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// DirectImporters returns the module packages that DIRECTLY import any of
// pkgPatterns, as sorted, deduped bare-form patterns ("./internal/bar") — the
// "+ direct reverse-import test packages" half of the covering-test corpus that
// CoveringTests alone cannot see.
//
// Why it exists: CoveringTests walks only the changed packages' own
// directories, so a package whose _test.go imports the changed package — the
// literal definition of a covering test — stays invisible to test-amplification,
// which then falls back to the whole-repo Grep the corpus exists to remove.
// Imports from _test.go files therefore COUNT here; that is the case that
// matters most.
//
// DIRECT means one hop. Transitive importers are excluded on purpose: an
// unbounded closure would re-inflate the very context this corpus shrinks. The
// input packages are excluded too — they are already in the corpus.
//
// Fail-open like every other deriver here: any unusable input (empty root, no
// go/go.mod, the module-wide "./..." sweep, a pattern escaping the module dir,
// a package nothing imports) yields nil — never an error, never a panic — and
// the corpus degrades to exactly today's changed-packages-only set.
func DirectImporters(repoRoot string, pkgPatterns []string) []string {
	if strings.TrimSpace(repoRoot) == "" || len(pkgPatterns) == 0 {
		return nil
	}
	moduleDir := filepath.Join(repoRoot, "go")
	modulePath, ok := modulePathOf(moduleDir)
	if !ok {
		return nil
	}
	// Target import paths, keyed by the module-relative dir they were derived
	// from, so an input package can be excluded from its own widening.
	targets := map[string]string{} // import path -> module-relative dir
	inputs := map[string]struct{}{}
	for _, pat := range pkgPatterns {
		rel, ok := patternDir(pat)
		if !ok {
			continue
		}
		targets[modulePath+"/"+rel] = rel
		inputs[rel] = struct{}{}
	}
	if len(targets) == 0 {
		return nil
	}

	fset := token.NewFileSet()
	importers := map[string]struct{}{}
	// Best-effort walk: an unreadable subtree or an unparseable file yields no
	// importers rather than an error, so a permissions blip degrades to today's
	// behavior.
	_ = filepath.Walk(moduleDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			// acs holds the cycle predicate gates, not the product; the rest
			// are never covering tests.
			case "acs", "testdata", "vendor", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		rel, rerr := filepath.Rel(moduleDir, filepath.Dir(p))
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if _, isInput := inputs[rel]; isInput {
			return nil // already in the corpus
		}
		for _, spec := range file.Imports {
			if spec == nil || spec.Path == nil {
				continue
			}
			// Exact match, never prefix: "example.com/other/internal/foo" and
			// ".../internal/foobar" are different packages, and matching them
			// would over-inject.
			path, uerr := strconv.Unquote(spec.Path.Value)
			if uerr != nil {
				continue
			}
			if _, hit := targets[path]; hit {
				importers[rel] = struct{}{}
				break
			}
		}
		return nil
	})
	if len(importers) == 0 {
		return nil // a leaf package nothing imports is a normal cycle
	}
	out := make([]string, 0, len(importers))
	for rel := range importers {
		out = append(out, "./"+rel)
	}
	sort.Strings(out)
	return out
}

// modulePathOf reads the `module` line of moduleDir/go.mod, or reports false
// when there is no readable module there (the fail-open signal: without the
// module path no import can be resolved to a package in this tree).
func modulePathOf(moduleDir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(moduleDir, "go.mod"))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module") {
			continue
		}
		if p := strings.TrimSpace(strings.TrimPrefix(line, "module")); p != "" {
			return p, true
		}
	}
	return "", false
}
