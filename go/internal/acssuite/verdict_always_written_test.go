package acssuite

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
)

func resultByACID(t *testing.T, v Verdict, acid string) Result {
	t.Helper()
	for _, r := range v.Results {
		if r.ACID == acid {
			return r
		}
	}
	t.Fatalf("no result %q in verdict; results=%+v", acid, v.Results)
	return Result{}
}

func assertShipGateReadsAFail(t *testing.T, v Verdict) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	read, err := ReadVerdict(raw)
	if err != nil {
		t.Fatalf("the verdict must be a well-formed artifact the ship gate reads: %v", err)
	}
	if read.Verdict != "FAIL" || read.ShipEligible || read.RedCount == 0 {
		t.Fatalf("verdict=%q ship_eligible=%v red_count=%d, want a FAIL that cannot ship", read.Verdict, read.ShipEligible, read.RedCount)
	}
}

func writeFixtureModule(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		mustMkdir(t, filepath.Dir(p))
		mustWrite(t, p, body)
	}
}

func TestRun_ANonCompilingScopeIsANamedRedCarryingTheCompilerOutput(t *testing.T) {
	raw := `{"Action":"build-output","Package":"` + acsPkgBase + `cycle9","Output":"./p_test.go:3:2: undefined: core.WithJudgeModel\n"}` + "\n"
	v, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoExec: seamGo(raw, &fakeExitErr{1})})
	if err != nil {
		t.Fatalf("Run = %v; a scope that cannot run must be a red in the verdict, never a missing verdict", err)
	}
	red := resultByACID(t, v, acsverdict.SyntheticRedPrefix+"go-lane-scope-failed/acs/cycle9")
	if red.ResultStr != "red" || red.ExitCode != 1 {
		t.Fatalf("scope failure = {result:%q exit:%d}, want {red 1}", red.ResultStr, red.ExitCode)
	}
	for _, want := range []string{"undefined: core.WithJudgeModel", "exit status 1"} {
		if !strings.Contains(red.EvidenceExcerpt, want) {
			t.Errorf("evidence must carry %q so the artifact names the real cause; got %q", want, red.EvidenceExcerpt)
		}
	}
	assertShipGateReadsAFail(t, v)
}

func TestRun_NoGoPredicateTreeIsARedVerdictNamingTheMissingTree(t *testing.T) {
	root := t.TempDir()
	v, err := Run(Options{Root: root, Cycle: 9})
	if err != nil {
		t.Fatalf("Run = %v, want a verdict", err)
	}
	red := resultByACID(t, v, acsverdict.SyntheticRedPrefix+"no-predicates")
	if !strings.Contains(red.EvidenceExcerpt, filepath.Join(root, "go")) {
		t.Errorf("evidence must name the module dir that holds no predicate tree; got %q", red.EvidenceExcerpt)
	}
	if v.PredicateSuite.Total != 1 {
		t.Errorf("total = %d, want exactly the one no-predicates red", v.PredicateSuite.Total)
	}
	assertShipGateReadsAFail(t, v)
}

func TestRun_AGoTreeWithNoActiveScopeIsARedVerdict(t *testing.T) {
	root := t.TempDir()
	writeFixtureModule(t, root, map[string]string{"go/go.mod": "module x\n\ngo 1.23\n"})
	mustMkdir(t, filepath.Join(root, "go", "acs", "cycle5"))
	v, err := Run(Options{Root: root, Cycle: 9})
	if err != nil {
		t.Fatalf("Run = %v, want a verdict", err)
	}
	red := resultByACID(t, v, acsverdict.SyntheticRedPrefix+"no-predicates")
	if !strings.Contains(red.EvidenceExcerpt, "cycle 9") {
		t.Errorf("evidence must name the cycle whose scopes are absent; got %q", red.EvidenceExcerpt)
	}
	assertShipGateReadsAFail(t, v)
}

func TestRun_ASeamWithEveryScopeInactiveIsARedVerdict(t *testing.T) {
	v, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoExec: seamGo("", nil)})
	if err != nil {
		t.Fatalf("Run = %v, want a verdict", err)
	}
	resultByACID(t, v, acsverdict.SyntheticRedPrefix+"no-predicates")
	assertShipGateReadsAFail(t, v)
}

func TestRun_AnIncompleteInventoryIsANamedRedKeepingWhatRan(t *testing.T) {
	raw := goStream(goLine(acsPkgBase+"cycle9", "TestC9_001_Ok", "pass"))
	missing := &inventoryGap{missing: "fixture/acs/cycle9/TestC9_002_Hidden"}
	v, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoExec: seamGo(raw, missing)})
	if err != nil {
		t.Fatalf("Run = %v, want a verdict", err)
	}
	if got := resultByACID(t, v, "cycle9/TestC9_001_Ok"); got.ResultStr != "green" {
		t.Errorf("the predicate that ran keeps its own result; got %q", got.ResultStr)
	}
	red := resultByACID(t, v, acsverdict.SyntheticRedPrefix+"go-lane-scope-failed/acs/cycle9")
	if !strings.Contains(red.EvidenceExcerpt, "TestC9_002_Hidden") {
		t.Errorf("evidence must name the declared test that never ran; got %q", red.EvidenceExcerpt)
	}
	assertShipGateReadsAFail(t, v)
}

type inventoryGap struct{ missing string }

func (e *inventoryGap) Error() string {
	return "predicate execution omitted declared tests " + e.missing
}
func (e *inventoryGap) Unwrap() error { return errIncompleteInventory }

func TestRun_AScopeLeftWithoutBudgetReportsTheSpentDeadline(t *testing.T) {
	started := map[string]int{}
	exec := func(ctx context.Context, _, pattern string, _ []string) (string, error) {
		started[pattern]++
		if strings.HasPrefix(pattern, "./acs/cycle") {
			<-ctx.Done()
			return "", ctx.Err()
		}
		return "", nil
	}
	v, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoTimeout: 50 * time.Millisecond, GoExec: exec})
	if err != nil {
		t.Fatalf("Run = %v, want a verdict", err)
	}
	ran := resultByACID(t, v, acsverdict.SyntheticRedPrefix+"go-lane-scope-failed/acs/cycle9")
	for _, want := range []string{"budget", "expired", "context deadline exceeded"} {
		if !strings.Contains(ran.EvidenceExcerpt, want) {
			t.Errorf("the scope the deadline killed must say so (%q); got %q", want, ran.EvidenceExcerpt)
		}
	}
	notRun := resultByACID(t, v, acsverdict.SyntheticRedPrefix+"go-lane-scope-failed/acs/redteam")
	if !strings.Contains(notRun.EvidenceExcerpt, "spent before this scope started") {
		t.Errorf("a scope left without budget must report the spent budget; got %q", notRun.EvidenceExcerpt)
	}
	if started["./acs/redteam"] != 0 {
		t.Errorf("a scope with no budget left must never be started; started %d times", started["./acs/redteam"])
	}
	assertShipGateReadsAFail(t, v)
}

func TestRun_AnExpiredLaneBudgetIsReportedAsTheDeadlineNotAsAnEmptyScope(t *testing.T) {
	root := t.TempDir()
	writeFixtureModule(t, root, map[string]string{
		"go/go.mod":                       "module fixture\n\ngo 1.23\n",
		"go/acs/cycle7/acs_test.go":       "package cycle7\n\nimport \"testing\"\n\nfunc TestC7_001_Green(t *testing.T) {}\n",
		"go/acs/regression/a/acs_test.go": "package a\n\nimport \"testing\"\n\nfunc TestGreen(t *testing.T) {}\n",
	})
	v, err := Run(Options{Root: root, Cycle: 7, GoTimeout: time.Nanosecond})
	if err != nil {
		t.Fatalf("Run = %v; an expired budget must be reported in the verdict", err)
	}
	for _, acid := range []string{"go-lane-scope-failed/acs/cycle7", "go-lane-scope-failed/acs/regression/a"} {
		red := resultByACID(t, v, acsverdict.SyntheticRedPrefix+acid)
		if !strings.Contains(red.EvidenceExcerpt, "budget") || !strings.Contains(red.EvidenceExcerpt, "spent before this scope started") {
			t.Errorf("%s: evidence must report the spent lane budget, not an empty scope; got %q", acid, red.EvidenceExcerpt)
		}
	}
	assertShipGateReadsAFail(t, v)
}

func TestRun_TheGoLaneBudgetIsOneDeadlineSharedByEveryScope(t *testing.T) {
	var deadlines []time.Time
	exec := func(ctx context.Context, _, _ string, _ []string) (string, error) {
		d, ok := ctx.Deadline()
		if !ok {
			t.Error("every scope must run under the lane deadline")
		}
		deadlines = append(deadlines, d)
		return "", nil
	}
	start := time.Now()
	if _, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoTimeout: time.Hour, GoExec: exec}); err != nil {
		t.Fatal(err)
	}
	if len(deadlines) != 3 {
		t.Fatalf("scopes run = %d, want the cycle, regression and red-team scopes", len(deadlines))
	}
	for _, d := range deadlines[1:] {
		if !d.Equal(deadlines[0]) {
			t.Fatalf("deadlines %v differ; the lane budget is one deadline shared by every scope", deadlines)
		}
	}
	if d := deadlines[0].Sub(start); d < time.Hour || d > time.Hour+time.Minute {
		t.Errorf("shared deadline is %s after the start, want the whole GoTimeout (1h)", d)
	}
}

func TestRun_ReadsTheLaneBudgetFromThePolicyUnderTheStateRoot(t *testing.T) {
	root := t.TempDir()
	writeFixtureModule(t, root, map[string]string{".evolve/policy.json": `{"acs":{"go_timeout_s":7}}`})
	var deadline time.Time
	exec := func(ctx context.Context, _, _ string, _ []string) (string, error) {
		deadline, _ = ctx.Deadline()
		return "", nil
	}
	start := time.Now()
	if _, err := Run(Options{Root: root, Cycle: 9, GoExec: exec}); err != nil {
		t.Fatal(err)
	}
	if d := deadline.Sub(start); d < 7*time.Second || d > 8*time.Second {
		t.Errorf("lane budget = %s, want the 7s acs.go_timeout_s of the policy under the state root %s", d, root)
	}
}

func TestRun_StampsAbsoluteRootsForRelativeInputs(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, plane := t.TempDir(), t.TempDir()
	relRoot, err := filepath.Rel(wd, root)
	if err != nil {
		t.Fatal(err)
	}
	relPlane, err := filepath.Rel(wd, plane)
	if err != nil {
		t.Fatal(err)
	}
	raw := goStream(goLine(acsPkgBase+"cycle9", "TestC9_001_Ok", "pass"))
	var seenEnv []string
	exec := func(_ context.Context, _, pattern string, env []string) (string, error) {
		seenEnv = env
		return seamGo(raw, nil)(context.Background(), "", pattern, env)
	}
	v, err := Run(Options{Root: relRoot, ProjectRoot: relPlane, Cycle: 9, GoExec: exec})
	if err != nil {
		t.Fatal(err)
	}
	if v.SuiteRoot != root || v.ProjectRoot != plane {
		t.Errorf("stamps suite_root=%q project_root=%q, want the absolute %q and %q", v.SuiteRoot, v.ProjectRoot, root, plane)
	}
	env := effectiveEnv(seenEnv)
	if env["EVOLVE_PROJECT_ROOT"] != plane || env["EVOLVE_WORKTREE_ROOT"] != root {
		t.Errorf("predicates must receive absolute roots; got project=%q worktree=%q", env["EVOLVE_PROJECT_ROOT"], env["EVOLVE_WORKTREE_ROOT"])
	}
}

func TestWriteVerdict_RefusesARelativeEvolveDir(t *testing.T) {
	const rel = "acs-relative-evolve-dir-must-not-exist"
	t.Cleanup(func() { _ = os.RemoveAll(rel) })
	if _, err := WriteVerdict(rel, Verdict{SchemaVersion: "1.0", Cycle: 1}); err == nil {
		t.Fatal("WriteVerdict accepted a relative evolve dir; it would land wherever the process cwd happens to be")
	}
	if _, err := os.Stat(rel); !os.IsNotExist(err) {
		t.Errorf("a refused write must create nothing under the cwd (stat err=%v)", err)
	}
}

func TestRun_AnIncompleteInventoryIsNamedEvenBesideARealRed(t *testing.T) {
	raw := goStream(
		goLine(acsPkgBase+"cycle9", "TestC9_001_Red", "output"),
		goLine(acsPkgBase+"cycle9", "TestC9_001_Red", "fail"),
	)
	missing := &inventoryGap{missing: "fixture/acs/cycle9/TestC9_002_Hidden"}
	v, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoExec: seamGo(raw, missing)})
	if err != nil {
		t.Fatal(err)
	}
	red := resultByACID(t, v, acsverdict.SyntheticRedPrefix+"go-lane-scope-failed/acs/cycle9")
	if !strings.Contains(red.EvidenceExcerpt, "TestC9_002_Hidden") {
		t.Errorf("a declared test that never ran must be named even when another predicate is red; got %q", red.EvidenceExcerpt)
	}
}

func TestRun_AScopeTheDeadlineKilledMidRunSaysSoBesideItsReds(t *testing.T) {
	exec := func(ctx context.Context, _, pattern string, _ []string) (string, error) {
		if !strings.HasPrefix(pattern, "./acs/cycle") {
			return "", nil
		}
		<-ctx.Done()
		return goStream(goLine(acsPkgBase+"cycle9", "TestC9_001_Slow", "run")), ctx.Err()
	}
	v, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoTimeout: 50 * time.Millisecond, GoExec: exec})
	if err != nil {
		t.Fatal(err)
	}
	red := resultByACID(t, v, acsverdict.SyntheticRedPrefix+"go-lane-scope-failed/acs/cycle9")
	if !strings.Contains(red.EvidenceExcerpt, "expired while this scope ran") {
		t.Errorf("a scope the lane deadline killed must say so even beside its own incomplete reds; got %q", red.EvidenceExcerpt)
	}
}

func TestRun_AScopeFailureIsCountedInItsOwnScopesBucket(t *testing.T) {
	broken := `{"Action":"build-output","Package":"x","Output":"./x.go:1: syntax error\n"}` + "\n"
	v, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoExec: seamGoByPattern(map[string]goSeamOut{
		"./acs/cycle9":         {raw: broken, err: &fakeExitErr{2}},
		"./acs/regression/...": {raw: broken, err: &fakeExitErr{2}},
		"./acs/redteam":        {raw: broken, err: &fakeExitErr{2}},
	})})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.PredicateSuite; got.ThisCycleCount != 1 || got.RegressionSuiteCount != 1 || got.RedTeamCount != 1 {
		t.Errorf("predicate_suite = %+v, want each scope's harness red in its own scope's bucket", got)
	}
	assertShipGateReadsAFail(t, v)
}

func TestNoPredicatesRed_IsAHarnessRedCarryingItsReason(t *testing.T) {
	red := noPredicatesRed("no predicate tree under /plane/go")
	if red.ACID != acsverdict.SyntheticRedPrefix+"no-predicates" || red.Predicate != red.ACID || red.ResultStr != "red" || red.ExitCode != 1 {
		t.Fatalf("red = {ac_id:%q predicate:%q result:%q exit:%d}, want the egps/no-predicates harness red", red.ACID, red.Predicate, red.ResultStr, red.ExitCode)
	}
	if !strings.Contains(red.EvidenceExcerpt, "no predicate tree under /plane/go") {
		t.Errorf("evidence = %q, want the reason", red.EvidenceExcerpt)
	}
}
