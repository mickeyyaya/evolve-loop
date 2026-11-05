//go:build acs

package cycle2167

import (
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
	shipPkg      = "internal/phases/ship"
	cliroutePkg  = "internal/cliroute"
	verdictPkg   = "internal/testverdict"
	shipEnforce  = "TestVerdictStore_TheShipRunsNoPackageAtEnforceOnAnUnchangedKeyAndEveryPackageAtShadow"
	shipRedAgain = "TestVerdictStore_AtEnforceAStoredFloorRedRunsAgainAndFailsTheShip"
	headTail     = "TestHeadroom_AFamilyAtTheLimitMovesToTheTailAndTheRestKeepTheirPolicyOrder"
	shorterFirst = "TestHeadroom_AShorterLaneTimeToAPassBeatsAnEarlierPolicyPlace"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

type located struct {
	source string
}

func locateTest(t *testing.T, pkg, name string) (located, bool) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(goDir(t), filepath.FromSlash(pkg), "*_test.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob %s tests: %v (%d files)", pkg, err, len(paths))
	}
	fset := token.NewFileSet()
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		f, err := parser.ParseFile(fset, p, raw, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.Name == name {
				return located{source: string(raw[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset])}, true
			}
		}
	}
	return located{}, false
}

func requirePasses(t *testing.T, pkg, name string) located {
	t.Helper()
	loc, found := locateTest(t, pkg, name)
	if !found {
		t.Fatalf("RED: %s is not declared in %s", name, pkg)
	}
	args := []string{"-C", goDir(t), "test", "-count=1", "-v", "-run", "^" + name + "$", "./" + pkg}
	out, errOut, code, err := acsassert.SubprocessOutput("go", args...)
	if err != nil || code != 0 {
		t.Fatalf("%s exit=%d err=%v\n%s\n%s", name, code, err, acsassert.OutputTail(out, 30), acsassert.OutputTail(errOut, 30))
	}
	if !strings.Contains(out, "--- PASS: "+name+" ") {
		t.Fatalf("%s did not run to a PASS:\n%s", name, acsassert.OutputTail(out, 30))
	}
	if strings.Contains(out, "--- SKIP: "+name) {
		t.Fatalf("%s was skipped, a skipped proof proves nothing", name)
	}
	return loc
}

func requireSourceMentions(t *testing.T, loc located, name string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(loc.source, n) {
			t.Errorf("%s never mentions %q: it does not exercise that case", name, n)
		}
	}
}

func requireNoNarrativeComments(t *testing.T, loc located, name string) {
	t.Helper()
	for _, line := range strings.Split(loc.source, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			t.Errorf("%s carries a comment line %q: code in this repo has no comments", name, strings.TrimSpace(line))
		}
	}
}

func TestC2167_001_TheShipRunsNoPackageAtEnforceAndEveryPackageAtShadow(t *testing.T) {
	loc := requirePasses(t, shipPkg, shipEnforce)
	requireSourceMentions(t, loc, shipEnforce, "enforceLimits", "testLimits", `"packages"`, `"skipped"`, "floorPack", "scanOnce")
	requireNoNarrativeComments(t, loc, shipEnforce)
}

func TestC2167_002_AStoredFailIsNeverReusedByTheVerdictStore(t *testing.T) {
	requirePasses(t, verdictPkg, "TestStore_AStoredFailIsNeverReused")
}

func TestC2167_003_AtEnforceAStoredFloorRedRunsAgainAndFailsTheShip(t *testing.T) {
	loc := requirePasses(t, shipPkg, shipRedAgain)
	requireSourceMentions(t, loc, shipRedAgain, "enforceLimits", "switchOn", "floorPack", "scanOnce", "userPackage", "xonlyPackage")
	requireNoNarrativeComments(t, loc, shipRedAgain)
}

func TestC2167_004_HeadroomMovesAFamilyAtTheLimitToTheTail(t *testing.T) {
	loc := requirePasses(t, cliroutePkg, headTail)
	requireSourceMentions(t, loc, headTail, "weekUsed", "laneTimes", "requireChain", `"agy-claude"`, "100")
	requireNoNarrativeComments(t, loc, headTail)
}

func TestC2167_005_AShorterLaneTimeToAPassBeatsAnEarlierPolicyPlace(t *testing.T) {
	loc := requirePasses(t, cliroutePkg, shorterFirst)
	requireSourceMentions(t, loc, shorterFirst, "laneTimes", "lane(", "requireChain", "time.Minute")
	requireNoNarrativeComments(t, loc, shorterFirst)
}

func TestC2167_006_TheExistingShorterLaneTimeProofStillPasses(t *testing.T) {
	requirePasses(t, cliroutePkg, "TestHeadroom_WithAMeasureTheShorterLaneTimeToAPassGoesFirst")
	requirePasses(t, cliroutePkg, "TestHeadroom_AFamilyAtTheLimitLeavesTheHead")
}
