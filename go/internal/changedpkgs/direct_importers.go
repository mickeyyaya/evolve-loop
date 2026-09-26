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

// DirectImporters returns the sorted bare patterns of other module packages whose source or tests import pkgPatterns; nil when none or on unusable input.
func DirectImporters(repoRoot string, pkgPatterns []string) []string {
	if strings.TrimSpace(repoRoot) == "" || len(pkgPatterns) == 0 {
		return nil
	}
	moduleDir := filepath.Join(repoRoot, "go")
	modulePath, ok := modulePathOf(moduleDir)
	if !ok {
		return nil
	}
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
	// An unreadable subtree or unparseable file contributes no importers; the corpus fails open.
	_ = filepath.Walk(moduleDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			// acs holds the cycle predicates, not the product; none of these dirs holds a covering test.
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
			return nil
		}
		for _, spec := range file.Imports {
			if spec == nil || spec.Path == nil {
				continue
			}
			// Exact match, never prefix: ".../internal/foobar" is a different package.
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
		return nil
	}
	out := make([]string, 0, len(importers))
	for rel := range importers {
		out = append(out, "./"+rel)
	}
	sort.Strings(out)
	return out
}

// modulePathOf reads the module path from moduleDir/go.mod; false when there is no readable module.
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
