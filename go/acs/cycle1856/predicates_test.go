//go:build acs

package cycle1856

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reachabilityprobe"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const fixtureModule = "example.com/c1856fixture"

func TestC1856_001_ViolationErrorClosesCycleAtReferencedPackage(t *testing.T) {
	cases := []struct {
		name string
		v    reachabilityprobe.Violation
		want string
	}{
		{
			name: "two-hop cycle",
			v: reachabilityprobe.Violation{
				Site:  reachabilityprobe.CallSite{PinningPackage: "core", ReferencedPackage: "storage", Symbol: "UpdateStateMap"},
				Cycle: []string{"storage", "core"},
			},
			want: `pinning storage.UpdateStateMap( inside package "core" would create an import cycle: storage -> core -> storage`,
		},
		{
			name: "three-hop cycle",
			v: reachabilityprobe.Violation{
				Site:  reachabilityprobe.CallSite{PinningPackage: "c", ReferencedPackage: "a", Symbol: "Do"},
				Cycle: []string{"a", "b", "c"},
			},
			want: `pinning a.Do( inside package "c" would create an import cycle: a -> b -> c -> a`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.v.Error()
			if got != tc.want {
				t.Errorf("Violation.Error() =\n  %q\nwant\n  %q", got, tc.want)
			}
			pinning := tc.v.Site.PinningPackage
			if strings.Contains(got, pinning+" -> "+pinning) {
				t.Errorf("Violation.Error() = %q repeats the pinning package %q", got, pinning)
			}
		})
	}
}

func writeFixtureFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cyclicPinWorktree(t *testing.T) (string, string) {
	t.Helper()
	wt := t.TempDir()
	writeFixtureFile(t, wt, "go/go.mod", "module "+fixtureModule+"\n\ngo 1.23\n")
	writeFixtureFile(t, wt, "go/internal/core/state.go", "package core\n\ntype State struct{}\n")
	writeFixtureFile(t, wt, "go/internal/storage/storage.go",
		"package storage\n\nimport \""+fixtureModule+"/internal/core\"\n\nfunc UpdateStateMap(s *core.State) {}\n")
	frozen := "go/internal/core/frozen_pin_test.go"
	writeFixtureFile(t, wt, frozen,
		"package core\n\nimport \"testing\"\n\nfunc TestPin(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"storage.UpdateStateMap(\")\n}\n")
	return wt, frozen
}

func TestC1856_002_CheckFrozenPinsReportsClosedCycleThroughProductionPath(t *testing.T) {
	wt, frozen := cyclicPinWorktree(t)
	violations, err := reachabilityprobe.CheckFrozenPins(wt, []string{frozen})
	if err != nil {
		t.Fatalf("CheckFrozenPins: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("CheckFrozenPins returned %d violations, want 1: %+v", len(violations), violations)
	}
	storage := fixtureModule + "/internal/storage"
	core := fixtureModule + "/internal/core"
	wantTail := "import cycle: " + storage + " -> " + core + " -> " + storage
	if got := violations[0].Error(); !strings.HasSuffix(got, wantTail) {
		t.Errorf("Violation.Error() = %q, want suffix %q", got, wantTail)
	}
}

func TestC1856_003_UnreadableFrozenFileStaysFailOpen(t *testing.T) {
	wt, frozen := cyclicPinWorktree(t)
	missing := "go/internal/core/does_not_exist_test.go"

	sites, err := reachabilityprobe.ExtractFrozenPins(wt, []string{missing, frozen})
	if err != nil {
		t.Fatalf("ExtractFrozenPins with one unreadable file: err = %v, want nil (fail open)", err)
	}
	if len(sites) != 1 || sites[0].ReferencedPackage != "storage" || sites[0].Symbol != "UpdateStateMap" {
		t.Errorf("ExtractFrozenPins = %+v, want the one readable pin storage.UpdateStateMap", sites)
	}

	violations, err := reachabilityprobe.CheckFrozenPins(wt, []string{missing})
	if err != nil || len(violations) != 0 {
		t.Errorf("CheckFrozenPins(unreadable only) = %v, %v; want no violations and nil error", violations, err)
	}
}

func parseFrozenpins(t *testing.T) *ast.File {
	t.Helper()
	src := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "reachabilityprobe", "frozenpins.go")
	file, err := parser.ParseFile(token.NewFileSet(), src, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", src, err)
	}
	return file
}

func funcDecl(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func resultCount(fn *ast.FuncDecl) int {
	if fn.Type.Results == nil {
		return 0
	}
	n := 0
	for _, field := range fn.Type.Results.List {
		if len(field.Names) == 0 {
			n++
			continue
		}
		n += len(field.Names)
	}
	return n
}

func callsExtractPinsWithErrorResult(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 || len(assign.Lhs) < 2 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "extractPins" {
			found = true
		}
		return true
	})
	return found
}

func TestC1856_004_ExtractPinsHasNoDeadErrorPath(t *testing.T) {
	file := parseFrozenpins(t)
	extract := funcDecl(file, "extractPins")
	if extract == nil {
		t.Fatal("frozenpins.go: func extractPins not found")
	}
	if got := resultCount(extract); got != 1 {
		t.Errorf("extractPins returns %d results, want 1 (the error it never sets is dead)", got)
	}
	for _, caller := range []string{"ExtractFrozenPins", "CheckFrozenPins"} {
		fn := funcDecl(file, caller)
		if fn == nil {
			t.Errorf("frozenpins.go: func %s not found", caller)
			continue
		}
		if callsExtractPinsWithErrorResult(fn) {
			t.Errorf("%s still binds an error from extractPins, keeping an unreachable error branch", caller)
		}
	}
}

func packageVars(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				names[name.Name] = true
			}
		}
	}
	return names
}

func TestC1856_005_GoSourceLiteralIdentifierIsSpelledCorrectly(t *testing.T) {
	vars := packageVars(parseFrozenpins(t))
	if vars["goSourceLiteal"] {
		t.Error("frozenpins.go still declares the misspelled var goSourceLiteal")
	}
	if !vars["goSourceLiteral"] {
		t.Error("frozenpins.go declares no package var goSourceLiteral")
	}
}

func TestC1856_006_ReachabilityprobePackageTestsPass(t *testing.T) {
	goRoot := filepath.Join(acsassert.RepoRoot(t), "go")
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goRoot, "test", "-count=1", "./internal/reachabilityprobe")
	if err != nil || code != 0 {
		t.Errorf("go test ./internal/reachabilityprobe exit=%d err=%v\n%s\n%s", code, err, out, errOut)
	}
}
