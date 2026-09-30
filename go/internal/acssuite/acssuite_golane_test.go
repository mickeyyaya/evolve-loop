package acssuite

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func goLine(pkg, test, action string) string {
	// Elapsed only matters on terminal actions; a fixed value is fine for tests.
	switch action {
	case "pass", "fail", "skip":
		return `{"Action":"` + action + `","Package":"` + pkg + `","Test":"` + test + `","Elapsed":0.01}`
	case "output":
		return `{"Action":"output","Package":"` + pkg + `","Test":"` + test + `","Output":"boom\n"}`
	default:
		return `{"Action":"` + action + `","Package":"` + pkg + `","Test":"` + test + `"}`
	}
}

func goStream(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

const acsPkgBase = "github.com/mickeyyaya/evolve-loop/go/acs/"

// seamGo returns a GoExec seam that yields the canned NDJSON + err for the
// CURRENT-cycle scope (`./acs/cycle<N>`) and empty output for the regression /
// redteam scopes — so single-scope tests behave as before the lane gained
// regression + redteam scopes.
func seamGo(raw string, err error) func(context.Context, string, string, []string) (string, error) {
	return func(_ context.Context, _ string, pattern string, _ []string) (string, error) {
		if strings.HasPrefix(pattern, "./acs/cycle") {
			return raw, err
		}
		return "", nil
	}
}

// seamGoByPattern returns a GoExec seam that yields canned (raw, err) keyed by
// the exact package pattern, empty for any other scope.
func seamGoByPattern(byPat map[string]goSeamOut) func(context.Context, string, string, []string) (string, error) {
	return func(_ context.Context, _ string, pattern string, _ []string) (string, error) {
		if v, ok := byPat[pattern]; ok {
			return v.raw, v.err
		}
		return "", nil
	}
}

type goSeamOut struct {
	raw string
	err error
}

func TestGoLane_GoGreenGoFail_Verdict(t *testing.T) {
	root := t.TempDir()
	raw := goStream(
		goLine(acsPkgBase+"cycle9", "TestC9_001_Ok", "pass"),
		goLine(acsPkgBase+"cycle9", "TestC9_002_Bar", "run"),
		goLine(acsPkgBase+"cycle9", "TestC9_002_Bar", "output"),
		goLine(acsPkgBase+"cycle9", "TestC9_002_Bar", "fail"),
	)
	v, err := Run(Options{Root: root, Cycle: 9, GoExec: seamGo(raw, &fakeExitErr{1})})
	if err != nil {
		t.Fatal(err)
	}
	if v.GreenCount != 1 || v.RedCount != 1 {
		t.Errorf("green=%d red=%d, want 1/1", v.GreenCount, v.RedCount)
	}
	if v.Verdict != "FAIL" || v.ShipEligible {
		t.Errorf("verdict=%q ship=%v, want FAIL/false", v.Verdict, v.ShipEligible)
	}
}

func TestGoLane_GoSkip_NeitherGreenNorRed(t *testing.T) {
	root := t.TempDir()
	raw := goStream(
		goLine(acsPkgBase+"cycle9", "TestC9_001_Skip", "skip"),
	)
	v, err := Run(Options{Root: root, Cycle: 9, GoExec: seamGo(raw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	if v.SkipCount != 1 || v.RedCount != 0 || v.GreenCount != 0 {
		t.Errorf("skip=%d red=%d green=%d, want 1/0/0", v.SkipCount, v.RedCount, v.GreenCount)
	}
	if v.Verdict != "PASS" || !v.ShipEligible {
		t.Errorf("verdict=%q ship=%v, want PASS/true (skip-only ships)", v.Verdict, v.ShipEligible)
	}
}

func TestGoLane_Classification(t *testing.T) {
	root := t.TempDir()
	raw := goStream(
		goLine(acsPkgBase+"cycle9", "TestC9_001_Now", "pass"),
		goLine(acsPkgBase+"cycle8", "TestC8_001_Old", "pass"),
	)
	v, err := Run(Options{Root: root, Cycle: 9, GoExec: seamGo(raw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	if v.PredicateSuite.ThisCycleCount != 1 || v.PredicateSuite.RegressionSuiteCount != 1 {
		t.Errorf("this=%d regression=%d, want 1/1", v.PredicateSuite.ThisCycleCount, v.PredicateSuite.RegressionSuiteCount)
	}
}

func TestGoLane_RedTeamClassification(t *testing.T) {
	root := t.TempDir()
	raw := goStream(goLine(acsPkgBase+"redteam", "TestRT001_Foo", "pass"))
	v, err := Run(Options{Root: root, Cycle: 9, GoExec: seamGo(raw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	if v.PredicateSuite.RedTeamCount != 1 {
		t.Errorf("RedTeamCount=%d, want 1", v.PredicateSuite.RedTeamCount)
	}
	if len(v.Results) != 1 || !v.Results[0].IsRedTeam {
		t.Errorf("result IsRedTeam=%v, want true", v.Results)
	}
}

func TestGoLane_PackageQualifiedDedup(t *testing.T) {
	root := t.TempDir()
	raw := goStream(
		goLine(acsPkgBase+"cycle9", "TestShared", "pass"),
		goLine(acsPkgBase+"cycle10", "TestShared", "pass"),
	)
	v, err := Run(Options{Root: root, Cycle: 9, GoExec: seamGo(raw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	if v.GreenCount != 2 {
		t.Fatalf("green=%d, want 2 (same test name in 2 packages must not collide)", v.GreenCount)
	}
	if v.Results[0].ACID == v.Results[1].ACID {
		t.Errorf("ACIDs collide: both %q", v.Results[0].ACID)
	}
}

func TestGoLane_NoScopePresent_EmptyPass(t *testing.T) {
	root := t.TempDir()
	v, err := Run(Options{Root: root, Cycle: 1}) // no GoExec, no go/ subtree
	if err != nil {
		t.Fatal(err)
	}
	if v.PredicateSuite.Total != 0 || v.Verdict != "PASS" || !v.ShipEligible {
		t.Errorf("total=%d verdict=%q ship=%v, want 0/PASS/true (no scope → empty PASS)", v.PredicateSuite.Total, v.Verdict, v.ShipEligible)
	}
}

func TestGoLane_CompileError_HardError(t *testing.T) {
	root := t.TempDir()
	// Build failure: go emits package-level build-output (no Test field) + nonzero exit.
	raw := `{"Action":"build-output","Package":"` + acsPkgBase + `cycle9","Output":"./x.go:1: syntax error\n"}` + "\n"
	_, err := Run(Options{Root: root, Cycle: 9, GoExec: seamGo(raw, &fakeExitErr{2})})
	if err == nil {
		t.Fatal("Run must surface a hard error when a Go predicate scope fails to compile (zero test events + nonzero exit), got nil")
	}
}

func TestGoLane_GreenInvariantPreserved(t *testing.T) {
	root := t.TempDir()
	raw := goStream(goLine(acsPkgBase+"cycle9", "TestC9_002_Ok", "pass"))
	v, err := Run(Options{Root: root, Cycle: 9, GoExec: seamGo(raw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	if v.Verdict != "PASS" || v.RedCount != 0 {
		t.Fatalf("verdict=%q red=%d, want PASS/0", v.Verdict, v.RedCount)
	}
	for _, r := range v.Results {
		if r.ResultStr == "green" && r.EvidenceExcerpt != "" {
			t.Errorf("green predicate %s carries evidence %q, want none", r.ACID, r.EvidenceExcerpt)
		}
	}
}

func TestParseGoTestJSON_PackageQualifiedKeys(t *testing.T) {
	raw := goStream(
		goLine(acsPkgBase+"cycle9", "TestX", "pass"),
		goLine(acsPkgBase+"cycle10", "TestX", "output"),
		goLine(acsPkgBase+"cycle10", "TestX", "fail"),
	)
	results := parseGoTestJSON(strings.NewReader(raw), 9)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (package-qualified keys)", len(results))
	}
	byACID := map[string]Result{}
	for _, r := range results {
		byACID[r.ACID] = r
	}
	pass, okP := byACID["cycle9/TestX"]
	fail, okF := byACID["cycle10/TestX"]
	if !okP || !okF {
		t.Fatalf("ACIDs=%v, want cycle9/TestX and cycle10/TestX", byACID)
	}
	if pass.ResultStr != "green" || pass.EvidenceExcerpt != "" {
		t.Errorf("pass: result=%q evidence=%q, want green/empty", pass.ResultStr, pass.EvidenceExcerpt)
	}
	if fail.ResultStr != "red" || fail.EvidenceExcerpt == "" {
		t.Errorf("fail: result=%q evidence=%q, want red/non-empty", fail.ResultStr, fail.EvidenceExcerpt)
	}
}

func TestGoLane_RegressionScope(t *testing.T) {
	root := t.TempDir()
	regRaw := goStream(goLine(acsPkgBase+"regression/cycle84", "TestC84_002_CarryoverTodosCleared", "fail"))
	v, err := Run(Options{Root: root, Cycle: 256, GoExec: seamGoByPattern(map[string]goSeamOut{
		"./acs/regression/...": {raw: regRaw, err: &fakeExitErr{1}},
	})})
	if err != nil {
		t.Fatal(err)
	}
	if v.RedCount != 1 || v.Verdict != "FAIL" {
		t.Errorf("red=%d verdict=%q, want 1/FAIL (regression scope runs every cycle)", v.RedCount, v.Verdict)
	}
	if v.PredicateSuite.RegressionSuiteCount != 1 {
		t.Errorf("RegressionSuiteCount=%d, want 1", v.PredicateSuite.RegressionSuiteCount)
	}
}

func TestGoLane_RedteamScope(t *testing.T) {
	root := t.TempDir()
	rtRaw := goStream(goLine(acsPkgBase+"redteam", "TestRT001_LedgerRoleCompleteness", "pass"))
	v, err := Run(Options{Root: root, Cycle: 256, GoExec: seamGoByPattern(map[string]goSeamOut{
		"./acs/redteam": {raw: rtRaw},
	})})
	if err != nil {
		t.Fatal(err)
	}
	if v.PredicateSuite.RedTeamCount != 1 || len(v.Results) != 1 || !v.Results[0].IsRedTeam {
		t.Errorf("RedTeamCount=%d results=%v, want 1 red-team result", v.PredicateSuite.RedTeamCount, v.Results)
	}
}

func TestGoLane_PerScopeCompileError(t *testing.T) {
	root := t.TempDir()
	v := map[string]goSeamOut{
		"./acs/cycle256":       {raw: goStream(goLine(acsPkgBase+"cycle256", "TestC256_001_Ok", "pass"))},
		"./acs/regression/...": {raw: `{"Action":"build-output","Package":"x/acs/regression/cycle9","Output":"./x.go:1: syntax error\n"}` + "\n", err: &fakeExitErr{2}},
	}
	_, err := Run(Options{Root: root, Cycle: 256, GoExec: seamGoByPattern(v)})
	if err == nil {
		t.Fatal("a regression-scope compile error must be a HARD error even when the current-cycle scope is clean, got nil")
	}
}

func TestParseGoTestJSON_ScanErrorFailsLoud(t *testing.T) {
	// One JSON line whose Output exceeds the 1MB max-token buffer → scan error.
	huge := `{"Action":"output","Package":"` + acsPkgBase + `cycle9","Test":"TestC9_001","Output":"` +
		strings.Repeat("x", 2*1024*1024) + `"}`
	results := parseGoTestJSON(strings.NewReader(huge+"\n"), 9)
	var red bool
	for _, r := range results {
		if r.ResultStr == "red" && strings.Contains(r.ACID, "parse-error") {
			red = true
		}
	}
	if !red {
		t.Errorf("scan error must synthesize a RED (fail loud), got results=%v", results)
	}
}

func TestGoLane_CurrentCycleScope(t *testing.T) {
	root := t.TempDir()
	goDir := filepath.Join(root, "go")
	// A Go module with an acs/ tree and a cycle5 package — but the run is cycle 9.
	mustMkdir(t, filepath.Join(goDir, "acs", "cycle5"))
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module x\n\ngo 1.21\n")

	if currentCycleGoPkgExists(goDir, 9) {
		t.Fatal("precondition: go/acs/cycle9 must be absent")
	}
	if !currentCycleGoPkgExists(goDir, 5) {
		t.Fatal("precondition: go/acs/cycle5 must exist")
	}

	v, err := Run(Options{Root: root, Cycle: 9}) // no go/acs/cycle9, no regression/redteam → no-op
	if err != nil {
		t.Fatal(err)
	}
	if v.PredicateSuite.Total != 0 || v.Verdict != "PASS" {
		t.Errorf("total=%d verdict=%q, want 0/PASS (no scope present → empty PASS)",
			v.PredicateSuite.Total, v.Verdict)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeExitErr mimics a *exec.ExitError carrying a nonzero code for the GoExec
// seam (the production lane returns the real *exec.ExitError; tests only need a
// non-nil error to signal "go test exited nonzero").
type fakeExitErr struct{ code int }

func (e *fakeExitErr) Error() string { return "exit status " + strconv.Itoa(e.code) }
