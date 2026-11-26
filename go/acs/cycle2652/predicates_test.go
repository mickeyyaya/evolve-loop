//go:build acs

package cycle2652

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdPackage = "./cmd/evolve"

func runNamedTests(t *testing.T, pattern string, extra ...string) string {
	t.Helper()
	args := append([]string{"test", "-count=1", "-v", "-run", pattern}, extra...)
	args = append(args, cmdPackage)
	out, errOut, code, err := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go", args...)
	if err != nil || code != 0 {
		t.Fatalf("go test -run %s exit=%d err=%v\n%s", pattern, code, err, acsassert.OutputTail(out+errOut, 40))
	}
	return out
}

func requirePassed(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass", name)
		}
	}
	if strings.Contains(out, "--- FAIL") {
		t.Errorf("a test failed:\n%s", acsassert.OutputTail(out, 30))
	}
}

func declName(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Name.Name
	case *ast.GenDecl:
		var names []string
		for _, spec := range d.Specs {
			if v, ok := spec.(*ast.ValueSpec); ok {
				for _, n := range v.Names {
					names = append(names, n.Name)
				}
			}
		}
		return strings.Join(names, " ")
	}
	return ""
}

func TestC2652_001_ARedBatchMemberEjectsThroughTheBatchSplitAndAGreenBatchMerges(t *testing.T) {
	pattern := "^(TestGateJourney_AGreenBatchMergesEveryMemberAndRunsTheGateOnEach|TestGateJourney_ARedBatchMemberEjectsThroughTheBatchSplitAndTheGreenMemberMerges|TestL2Gate_AGreenTreeReturnsNilAndRunsTheStepsOfTheTierInTheGoDirectory|TestL2Gate_ARedStepReturnsAGateRedNamingEachRedStepAndStillReleasesTheCheckout)$"
	out := runNamedTests(t, pattern)
	requirePassed(t, out,
		"TestGateJourney_AGreenBatchMergesEveryMemberAndRunsTheGateOnEach",
		"TestGateJourney_ARedBatchMemberEjectsThroughTheBatchSplitAndTheGreenMemberMerges",
		"TestL2Gate_AGreenTreeReturnsNilAndRunsTheStepsOfTheTierInTheGoDirectory",
		"TestL2Gate_ARedStepReturnsAGateRedNamingEachRedStepAndStillReleasesTheCheckout",
	)
}

func TestC2652_002_AGateThatCannotRunIsNeverARedVerdict(t *testing.T) {
	pattern := "^(TestL2Gate_ACommandThatCannotRunIsAHostFailureAndNotAGateRed|TestL2Gate_AFailedCheckoutIsAPlainErrorAndRunsNoCommand|TestL2Gate_ARedStepOfALowTierNamesOnlyThatStep)$"
	out := runNamedTests(t, pattern)
	requirePassed(t, out,
		"TestL2Gate_ACommandThatCannotRunIsAHostFailureAndNotAGateRed",
		"TestL2Gate_AFailedCheckoutIsAPlainErrorAndRunsNoCommand",
		"TestL2Gate_ARedStepOfALowTierNamesOnlyThatStep",
	)
}

func TestC2652_003_TheProductionBatcherNamesNoMissingGateAndTheTrainGateIsReal(t *testing.T) {
	pattern := "^(TestProductionDriver_TheProductionBatcherNamesNoMissingPort|TestProductionDriver_CheckRefusesEnforceOnTheProductionCompositionWithoutTheLedgerPorts|TestIntegrationTrain_TheL2ComposerGateIsTheRealGate)$"
	out := runNamedTests(t, pattern)
	requirePassed(t, out,
		"TestProductionDriver_TheProductionBatcherNamesNoMissingPort",
		"TestProductionDriver_CheckRefusesEnforceOnTheProductionCompositionWithoutTheLedgerPorts",
		"TestIntegrationTrain_TheL2ComposerGateIsTheRealGate",
	)
}

func TestC2652_004_TheNoOpGateIsGoneFromTheCommandPackage(t *testing.T) {
	dir := filepath.Join(acsassert.GoModuleDir(t), "cmd", "evolve")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		for _, decl := range file.Decls {
			if strings.Contains(declName(decl), "l2NoGate") {
				t.Errorf("%s still declares l2NoGate", e.Name())
			}
		}
		for _, ref := range file.Unresolved {
			if ref.Name == "l2NoGate" {
				t.Errorf("%s still references l2NoGate", e.Name())
			}
		}
	}
}

func TestC2652_005_TheFixtureSoakPassesAndTheBatchTestCostIsInThePackageNote(t *testing.T) {
	out := runNamedTests(t, "^TestGateSoak_ABatchOfMembersCostsTwoGateCommandsPerMemberAndMergesAll$")
	requirePassed(t, out, "TestGateSoak_ABatchOfMembersCostsTwoGateCommandsPerMemberAndMergesAll")
	note := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "packages", "internal-integration.md")
	// acs-predicate: config-check
	if !acsassert.FileMatchesRegex(t, note, `BatchTest cost per batch[^\n]*[0-9]+(\.[0-9]+)? ?(ms|s)\b`) {
		t.Errorf("%s does not state the BatchTest cost per batch with a measured number and unit", note)
	}
}

func TestC2652_006_EachChangedFunctionIsFullyCoveredAndNoCommentIsAdded(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	pattern := "^(TestL2Gate_|TestGateJourney_|TestGateSoak_|TestIntegrationTrain_TheL2ComposerGateIsTheRealGate|TestIntegrationTrainWiresTheBatchTestAtTheStageOfThePolicy|TestProductionDriver_)"
	runNamedTests(t, pattern, "-coverprofile="+profile)
	funcs, errOut, code, err := acsassert.SubprocessOutputIn(goDir, "go", "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover exit=%d err=%v %s", code, err, errOut)
	}
	for _, fn := range []string{"l2Gate", "batchTest", "productionEnforceActor"} {
		pct, err := acsassert.FuncCoverage(funcs, fn)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		if pct < 100 {
			t.Errorf("%s coverage = %.1f%%, want 100%%", fn, pct)
		}
	}
	for _, name := range []string{"cmd_integration_l2.go", "cmd_loop_driver.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(goDir, "cmd", "evolve", name), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, group := range file.Comments {
			for _, c := range group.List {
				if !regexp.MustCompile(`^//(go:|line )`).MatchString(c.Text) {
					t.Errorf("%s holds the comment %q", name, c.Text)
				}
			}
		}
	}
}
