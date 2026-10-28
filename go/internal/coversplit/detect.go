package coversplit

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

type imports struct {
	test, code map[string]bool
}

func Detect(moduleDir string) ([]Split, error) {
	module, err := modulePath(moduleDir)
	if err != nil {
		return nil, err
	}
	dirs, err := packageImports(moduleDir)
	if err != nil {
		return nil, err
	}
	var found []Split
	for _, rel := range slices.Sorted(maps.Keys(dirs)) {
		for anc := path.Dir(rel); anc != "."; anc = path.Dir(anc) {
			imp := module + "/" + anc
			if _, isPkg := dirs[anc]; isPkg && dirs[rel].test[imp] && !dirs[rel].code[imp] {
				found = append(found, Split{Package: "./" + rel, Target: "./" + anc})
			}
		}
	}
	return found, nil
}

func Unlisted(moduleDir string, listed []Split) ([]string, error) {
	found, err := Detect(moduleDir)
	if err != nil {
		return nil, err
	}
	var problems []string
	for _, s := range found {
		if !slices.Contains(listed, s) {
			problems = append(problems, fmt.Sprintf("%s tests %s but %s has no `%s %s` line", s.Package, s.Target, File, s.Package, s.Target))
		}
	}
	for _, s := range listed {
		if !slices.Contains(found, s) {
			problems = append(problems, fmt.Sprintf("%s lists `%s %s`, but %s is not a test package split from %s", File, s.Package, s.Target, s.Package, s.Target))
		}
	}
	return problems, nil
}

func modulePath(moduleDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(moduleDir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("coversplit: read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if mod, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(mod), nil
		}
	}
	return "", fmt.Errorf("coversplit: go.mod in %s names no module", moduleDir)
}

func skipDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func packageImports(moduleDir string) (map[string]imports, error) {
	dirs := map[string]imports{}
	err := filepath.WalkDir(moduleDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != moduleDir && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		return addImports(dirs, relDir(moduleDir, p), p)
	})
	if err != nil {
		return nil, fmt.Errorf("coversplit: walk %s: %w", moduleDir, err)
	}
	return dirs, nil
}

func addImports(dirs map[string]imports, rel, file string) error {
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
	if err != nil {
		return err
	}
	im, ok := dirs[rel]
	if !ok {
		im = imports{test: map[string]bool{}, code: map[string]bool{}}
		dirs[rel] = im
	}
	into := im.code
	if strings.HasSuffix(file, "_test.go") {
		into = im.test
	}
	for _, spec := range f.Imports {
		into[strings.Trim(spec.Path.Value, "\"`")] = true
	}
	return nil
}

func relDir(moduleDir, file string) string {
	rel := strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(filepath.Dir(file), moduleDir)), "/")
	if rel == "" {
		return "."
	}
	return rel
}
