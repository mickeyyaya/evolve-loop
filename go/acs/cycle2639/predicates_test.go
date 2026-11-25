//go:build acs

package cycle2639

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	stageEvidencePkg = "./internal/stageevidence"
	sealScorerTest   = "TestSealOutcomeRoutedReachesConflictClass"
	realGuardTest    = "TestStageEvidenceRoutedSourceGuard"
)

var registryProbeTests = []string{
	"TestRoutedRegistry_UnregisteredEmitSiteFails",
	"TestRoutedRegistry_MissingScorerTestFails",
	"TestRoutedRegistry_RegisteredSiteWithExistingTestPasses",
	"TestRoutedRegistry_RegistrySiteAbsentFromTreeIsAccepted",
	"TestRoutedRegistry_SameFileOtherKindIsUnregistered",
	"TestRoutedRegistry_ScorerTestNamesReadsTestFunctions",
}

var sealScorerTests = []string{
	sealScorerTest,
	"TestSealOutcomeWithoutRoutedStaysClear",
	"TestSealRoutedSurvivesEncodeParseAndVerifyStored",
	"TestRoutedRegistry_RealRegistryNamesExistingScorerTests",
}

func goTest(t *testing.T, extra ...string) (output string, code int) {
	t.Helper()
	mod := acsassert.GoModuleDir(t)
	args := append([]string{"test", "-count=1", "-v"}, extra...)
	stdout, stderr, code, _ := acsassert.SubprocessOutputIn(mod, "go", args...)
	return stdout + stderr, code
}

func exactRun(names ...string) string {
	return "^(" + strings.Join(names, "|") + ")$"
}

func requirePassed(t *testing.T, output string, code int, names ...string) {
	t.Helper()
	if code != 0 {
		t.Fatalf("RED: go test exited %d:\n%s", code, acsassert.OutputTail(output, 25))
	}
	for _, name := range names {
		if !strings.Contains(output, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
}

func writeFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "emit.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const emitSiteFixture = `package fixture

import "github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"

func emit() stageevidence.Outcome {
	return stageevidence.Outcome{ID: 1, Routed: []string{"item-1"}}
}
`

const cleanFixture = `package fixture

import "github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"

func clean() stageevidence.Outcome {
	return stageevidence.Outcome{ID: 1, Shipped: true}
}
`

func TestC2639_001_RegistryProbesFailUnregisteredSiteAndMissingTestName(t *testing.T) {
	out, code := goTest(t, "-run", exactRun(registryProbeTests...), stageEvidencePkg)
	requirePassed(t, out, code, registryProbeTests...)
}

func TestC2639_002_GuardFailsAnUnregisteredEmitSiteInAFixtureTree(t *testing.T) {
	root := writeFixture(t, emitSiteFixture)
	out, code := goTest(t, "-run", exactRun(realGuardTest), stageEvidencePkg, "-args", "-routed-guard-root="+root)
	if code == 0 {
		t.Fatalf("RED: the guard passed over a fixture with an unregistered emit site:\n%s", acsassert.OutputTail(out, 20))
	}
	if !strings.Contains(out, "--- FAIL: "+realGuardTest) {
		t.Fatalf("RED: the guard did not run and fail:\n%s", acsassert.OutputTail(out, 20))
	}
	if !strings.Contains(out, "emit.go") || !strings.Contains(out, "unregistered") {
		t.Errorf("RED: the failure must name emit.go as unregistered:\n%s", acsassert.OutputTail(out, 20))
	}
}

func TestC2639_003_GuardPassesACleanFixtureTree(t *testing.T) {
	root := writeFixture(t, cleanFixture)
	out, code := goTest(t, "-run", exactRun(realGuardTest), stageEvidencePkg, "-args", "-routed-guard-root="+root)
	requirePassed(t, out, code, realGuardTest)
}

func TestC2639_004_SealScorerTestsReachConflictThroughTheEmitSiteAndGuardPassesOnTree(t *testing.T) {
	names := append([]string{realGuardTest}, sealScorerTests...)
	out, code := goTest(t, "-run", exactRun(names...), stageEvidencePkg)
	requirePassed(t, out, code, names...)
}

func legacyOutcomeBody(fn *ast.FuncDecl) string {
	recv := fn.Recv.List[0].Names[0].Name
	scope := fn.Type.Params.List[0].Names[0].Name
	return "{ return Outcome{Scope: " + scope + ".Kind, ID: " + scope + ".ID, Shipped: " + recv + ".Shipped, Incomplete: " + recv + ".Incomplete} }"
}

func droppedRoutedSource(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Outcome" || fn.Recv == nil || fn.Body == nil {
			continue
		}
		start, end := fset.Position(fn.Body.Lbrace).Offset, fset.Position(fn.Body.Rbrace).Offset+1
		return string(raw[:start]) + legacyOutcomeBody(fn) + string(raw[end:])
	}
	t.Fatalf("RED: %s has no Seal.Outcome method", path)
	return ""
}

func TestC2639_005_MutantDroppingRoutedFromSealOutcomeFailsTheSealScorerTest(t *testing.T) {
	mod := acsassert.GoModuleDir(t)
	sealPath := filepath.Join(mod, "internal", "stageevidence", "seal.go")
	mutant := filepath.Join(t.TempDir(), "seal_mutant.go")
	if err := os.WriteFile(mutant, []byte(droppedRoutedSource(t, sealPath)), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(t.TempDir(), "overlay.json")
	raw, _ := json.Marshal(map[string]map[string]string{"Replace": {sealPath: mutant}})
	if err := os.WriteFile(overlay, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	clean, cleanCode := goTest(t, "-run", exactRun(sealScorerTest), stageEvidencePkg)
	requirePassed(t, clean, cleanCode, sealScorerTest)
	out, code := goTest(t, "-overlay="+overlay, "-run", exactRun(sealScorerTest), stageEvidencePkg)
	if code == 0 {
		t.Fatalf("RED: the mutant that drops the routed items from Seal.Outcome survived:\n%s", acsassert.OutputTail(out, 20))
	}
	if !strings.Contains(out, "--- FAIL: "+sealScorerTest) {
		t.Errorf("RED: the mutant must fail %s by assertion, not by build error:\n%s", sealScorerTest, acsassert.OutputTail(out, 20))
	}
}
