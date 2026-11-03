//go:build acs

package emptytests

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type offender struct {
	file string
	line int
	name string
}

func (o offender) key() string { return o.file + " " + o.name }

func (o offender) String() string {
	return fmt.Sprintf("%s:%d: %s has an empty body, so it passes and proves nothing: assert the behavior, or call t.Skip with a reason", o.file, o.line, o.name)
}

func findOffenders(goDir string) ([]offender, error) {
	files, err := acsassert.WalkModule(goDir, isTestFile)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no test file under %s: the scan reads nothing, so it checks nothing", goDir)
	}
	fset := token.NewFileSet()
	var offenders []offender
	for _, rel := range files {
		file, err := parser.ParseFile(fset, filepath.Join(goDir, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Recv != nil || fn.Body == nil || len(fn.Body.List) > 0 || !isTestName(fn.Name.Name) {
				continue
			}
			offenders = append(offenders, offender{rel, fset.Position(fn.Pos()).Line, fn.Name.Name})
		}
	}
	return offenders, nil
}

func isTestFile(rel string) bool {
	if !strings.HasSuffix(rel, "_test.go") {
		return false
	}
	dirs := strings.Split(rel, "/")
	for _, dir := range dirs[:len(dirs)-1] {
		if dir == "testdata" || dir == "vendor" || strings.HasPrefix(dir, ".") || strings.HasPrefix(dir, "_") {
			return false
		}
	}
	return true
}

func isTestName(name string) bool {
	for _, prefix := range []string{"Test", "Fuzz", "Benchmark"} {
		rest, found := strings.CutPrefix(name, prefix)
		if !found {
			continue
		}
		next, _ := utf8.DecodeRuneInString(rest)
		return !unicode.IsLower(next)
	}
	return false
}

func readBaseline(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	baseline := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		if entry := strings.TrimSpace(line); entry != "" {
			baseline[entry] = true
		}
	}
	return baseline, nil
}

func violations(offenders []offender, baseline map[string]bool) []string {
	var out []string
	used := map[string]bool{}
	for _, o := range offenders {
		if baseline[o.key()] {
			used[o.key()] = true
			continue
		}
		out = append(out, o.String())
	}
	for _, entry := range slices.Sorted(maps.Keys(baseline)) {
		if !used[entry] {
			out = append(out, fmt.Sprintf("baseline entry %q matches no empty test: remove it, because the test now has a body or is gone", entry))
		}
	}
	return out
}
