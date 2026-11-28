//go:build acs

package cycle2685

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type goLiteralSite struct {
	file    string
	line    int
	inConst bool
}

func goToolLiteralSites(t *testing.T) []goLiteralSite {
	t.Helper()
	dir := filepath.Join(acsassert.GoModuleDir(t), "pkg", "acsassert")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	var sites []goLiteralSite
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			constLits := map[*ast.BasicLit]bool{}
			ast.Inspect(file, func(n ast.Node) bool {
				gd, ok := n.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					return true
				}
				ast.Inspect(gd, func(m ast.Node) bool {
					if lit, ok := m.(*ast.BasicLit); ok {
						constLits[lit] = true
					}
					return true
				})
				return false
			})
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if v, err := strconv.Unquote(lit.Value); err == nil && v == "go" {
					sites = append(sites, goLiteralSite{filepath.Base(path), fset.Position(lit.Pos()).Line, constLits[lit]})
				}
				return true
			})
		}
	}
	sort.Slice(sites, func(i, j int) bool {
		if sites[i].file != sites[j].file {
			return sites[i].file < sites[j].file
		}
		return sites[i].line < sites[j].line
	})
	return sites
}

func TestC2685_001_TheGoProgramNameIsOneNamedConstantInAcsassert(t *testing.T) {
	sites := goToolLiteralSites(t)
	var bare []goLiteralSite
	declared := 0
	for _, s := range sites {
		if s.inConst {
			declared++
			continue
		}
		bare = append(bare, s)
	}
	for _, s := range bare {
		t.Errorf("bare \"go\" literal at %s:%d: use the named constant", s.file, s.line)
	}
	if declared != 1 {
		t.Errorf("want exactly one const declaration of \"go\" in pkg/acsassert, found %d", declared)
	}
}

func TestC2685_002_TheBoundStillRunsTheGoToolAfterTheExtraction(t *testing.T) {
	out, _, code, err := acsassert.SubprocessOutput("go", "version")
	if err != nil || code != 0 {
		t.Fatalf("go version exit=%d err=%v", code, err)
	}
	if !strings.HasPrefix(out, "go version") {
		t.Errorf("unexpected go version output %q", out)
	}
}
