package changedpkgs

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func DirectImporters(repoRoot string, pkgPatterns []string) []string {
	if strings.TrimSpace(repoRoot) == "" || len(pkgPatterns) == 0 {
		return nil
	}
	moduleDir := filepath.Join(repoRoot, "go")
	modulePath, ok := modulePathOf(moduleDir)
	if !ok {
		return nil
	}
	dirByImportPath := map[string]string{}
	inputs := map[string]struct{}{}
	for _, pat := range pkgPatterns {
		rel, ok := patternDir(pat)
		if !ok {
			continue
		}
		dirByImportPath[modulePath+"/"+rel] = rel
		inputs[rel] = struct{}{}
	}
	if len(dirByImportPath) == 0 {
		return nil
	}
	importers := importersOf(moduleDir, dirByImportPath, inputs)
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

var importersRel = filepath.Rel

func importersOf(moduleDir string, dirByImportPath map[string]string, inputs map[string]struct{}) map[string]struct{} {
	fset := token.NewFileSet()
	importers := map[string]struct{}{}
	_ = filepath.Walk(moduleDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if holdsNoCoveringTest(info.Name()) {
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
		rel, rerr := importersRel(moduleDir, filepath.Dir(p))
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if _, isInput := inputs[rel]; isInput {
			return nil
		}
		if importsTarget(file, dirByImportPath) {
			importers[rel] = struct{}{}
		}
		return nil
	})
	return importers
}

func holdsNoCoveringTest(dirName string) bool {
	switch dirName {
	case "acs", "testdata", "vendor", ".git", "node_modules":
		return true
	}
	return false
}

func importsTarget(file *ast.File, dirByImportPath map[string]string) bool {
	for _, spec := range file.Imports {
		if spec == nil || spec.Path == nil {
			continue
		}
		path, uerr := strconv.Unquote(spec.Path.Value)
		if uerr != nil {
			continue
		}
		if _, hit := dirByImportPath[path]; hit {
			return true
		}
	}
	return false
}

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
