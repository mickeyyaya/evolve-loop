package structure

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Home struct {
	Dir      string
	TestOnly bool
}

func RequiredModules(goMod []byte) []string {
	var modules []string
	inBlock := false
	for _, line := range strings.Split(string(goMod), "\n") {
		fields := strings.Fields(line)
		switch {
		case len(fields) == 0 || strings.HasPrefix(fields[0], "//"):
		case line == ")":
			inBlock = false
		case strings.HasPrefix(line, "require ("):
			inBlock = true
		case strings.HasPrefix(line, "require "):
			modules = appendDirect(modules, fields[1:])
		case inBlock:
			modules = appendDirect(modules, fields)
		}
	}
	return modules
}

func appendDirect(modules, fields []string) []string {
	if len(fields) < 2 || strings.HasSuffix(strings.Join(fields, " "), "// indirect") {
		return modules
	}
	return append(modules, fields[0])
}

func ThirdPartyViolations(root string, homes map[string]Home) ([]string, error) {
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, module := range RequiredModules(goMod) {
		if _, ok := homes[module]; !ok {
			violations = append(violations, "go.mod requires "+module+" with no declared home")
		}
	}
	err = fs.WalkDir(os.DirFS(root), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return skipDir(d.Name())
		}
		if !strings.HasSuffix(rel, ".go") {
			return nil
		}
		found, err := fileViolations(filepath.Join(root, rel), rel, homes)
		violations = append(violations, found...)
		return err
	})
	sort.Strings(violations)
	return violations, err
}

func skipDir(name string) error {
	switch name {
	case "vendor", "testdata", ".git", "node_modules":
		return filepath.SkipDir
	}
	return nil
}

func fileViolations(path, rel string, homes map[string]Home) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	isTest := strings.HasSuffix(rel, "_test.go")
	var violations []string
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"`")
		module, home, ok := homeOf(importPath, homes)
		switch {
		case !ok || (isTest && !home.TestOnly):
		case home.TestOnly:
			if !isTest {
				violations = append(violations, rel+": imports "+module+", allowed in test files only")
			}
		case !strings.HasPrefix(rel, home.Dir+"/"):
			violations = append(violations, rel+": imports "+module+", whose only home is "+home.Dir)
		}
	}
	return violations, nil
}

func homeOf(importPath string, homes map[string]Home) (string, Home, bool) {
	for module, home := range homes {
		if importPath == module || strings.HasPrefix(importPath, module+"/") {
			return module, home, true
		}
	}
	return "", Home{}, false
}
