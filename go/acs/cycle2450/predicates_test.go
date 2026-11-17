//go:build acs

package cycle2450

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	scopedsetPkg     = "./internal/scopedset"
	coreTestPkg      = "./internal/core"
	cmdEvolveTestPkg = "./cmd/evolve"
)

func goTestNamed(t *testing.T, pkg string, names ...string) {
	t.Helper()
	moduleDir := acsassert.GoModuleDir(t)
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "-run", pattern, pkg)
	if err != nil || code != 0 {
		t.Fatalf("go test -run %s %s exited %d (%v):\n%s", pattern, pkg, code, err, acsassert.OutputTail(stdout+stderr, 40))
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass in %s; a -run that matches nothing exits 0:\n%s", name, pkg, acsassert.OutputTail(stdout, 20))
		}
	}
}

func TestC2450_001_ScopedSetIsTheIntersectionOfBothReverseClosures(t *testing.T) {
	goTestNamed(t, scopedsetPkg, "TestScopedSet_IsTheIntersectionOfBothReverseClosures")
}

func TestC2450_002_AGlobalInputSelectsAll(t *testing.T) {
	goTestNamed(t, scopedsetPkg, "TestScopedSet_AGlobalInputSelectsAll")
}

func TestC2450_003_ATreeReadingTestJoinsWhenItsInputChanged(t *testing.T) {
	goTestNamed(t, scopedsetPkg, "TestScopedSet_ATreeReadingTestJoinsWhenItsInputChanged")
}

func TestC2450_004_AFailureOutsideTheSetIsAnEscape(t *testing.T) {
	goTestNamed(t, scopedsetPkg, "TestShadow_AFailureOutsideTheSetIsAnEscape")
}

func TestC2450_005_TheShadowRecordAppendsOneLinePerRecordAndRefusesAnEmptyPlane(t *testing.T) {
	goTestNamed(t, scopedsetPkg,
		"TestShadow_AppendWritesOneLinePerRecordUnderThePlane",
		"TestShadow_AppendRefusesAnEmptyPlane")
}

func TestC2450_006_TheCompositionGateRunnerRecordsTheShadowAndKeepsTheFullRecheck(t *testing.T) {
	goTestNamed(t, cmdEvolveTestPkg,
		"TestComposedGatesTo_ShadowRecordsAnEscapeForARedOutsideTheScopedSet",
		"TestComposedGatesTo_ShadowRecordsAGreenRecheckWithNoEscape",
		"TestComposedGatesTo_AnAuditedBaseThatCannotBeReadSelectsTheFullSet",
		"TestComposedGatesTo_WithoutARecheckSiteNoShadowRecordIsWritten",
		"TestComposedGatesTo_AShadowRecordThatCannotBeWrittenLeavesTheGatesUntouched")
}

func TestC2450_007_BothProductionRecheckCallersNameTheirSiteToTheGateRunner(t *testing.T) {
	goTestNamed(t, coreTestPkg,
		"TestRecheckSite_RoundTripsThroughTheContext",
		"TestRecheckSite_IdentityCarryHandsTheGateRunnerTheAuditedBaseAndTheProjectRoot",
		"TestRecheckSite_ScopedMergeHandsTheGateRunnerTheAuditedBaseAndTheProjectRoot")
}

func exportedNames(t *testing.T, pkgDir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(pkgDir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]struct{}{}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			collectExported(decl, set)
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func collectExported(decl ast.Decl, set map[string]struct{}) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Name.IsExported() {
			set[d.Name.Name] = struct{}{}
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if s.Name.IsExported() {
					set[s.Name.Name] = struct{}{}
				}
			case *ast.ValueSpec:
				for _, n := range s.Names {
					if n.IsExported() {
						set[n.Name] = struct{}{}
					}
				}
			}
		}
	}
}

func identifiersIn(t *testing.T, file string) map[string]struct{} {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	idents := map[string]struct{}{}
	ast.Inspect(parsed, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			idents[id.Name] = struct{}{}
		}
		return true
	})
	return idents
}

func TestC2450_008_TheNewPackageGraduatesIntoTheApicoverGateAndHasItsNote(t *testing.T) {
	root := acsassert.RepoRoot(t)
	moduleDir := acsassert.GoModuleDir(t)
	pkgDir := filepath.Join(moduleDir, "internal", "scopedset")

	// acs-predicate: config-check
	if !acsassert.FileMatchesRegex(t, filepath.Join(moduleDir, ".apicover-enforce"), `(?m)^\./internal/scopedset$`) {
		t.Errorf("go/.apicover-enforce does not enroll ./internal/scopedset; the repo-wide apicover gate would abort the build phase")
	}

	exports := exportedNames(t, pkgDir)
	if len(exports) == 0 {
		t.Fatalf("no non-test Go file of %s exports a symbol yet", pkgDir)
	}
	named := filepath.Join(pkgDir, "apicover_named_test.go")
	if _, err := os.Stat(named); err != nil {
		t.Fatalf("apicover_named_test.go is missing: %v", err)
	}
	idents := identifiersIn(t, named)
	for _, name := range exports {
		if _, ok := idents[name]; !ok {
			t.Errorf("apicover_named_test.go does not name exported symbol %s in a real assertion", name)
		}
	}

	for _, rel := range []string{
		"docs/architecture/packages/internal-scopedset.md",
		"go/.apicover-enforce",
		"go/internal/scopedset/apicover_named_test.go",
	} {
		if !acsassert.FileExists(t, filepath.Join(root, rel)) {
			t.Errorf("%s is missing", rel)
			continue
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code == 0 {
			t.Errorf("%s is gitignored and would be dropped at ship", rel)
		}
	}
}
