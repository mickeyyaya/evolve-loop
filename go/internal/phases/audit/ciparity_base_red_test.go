package audit

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/ciparitygate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type baseRun struct {
	calls     int
	moduleDir string
	reds      baseattr.Reds
	budget    time.Duration
	red       func(baseattr.Reds) []baseattr.Key
}

func (b *baseRun) run(ctx context.Context, moduleDir string, r baseattr.Reds) ([]baseattr.Key, error) {
	b.calls++
	b.moduleDir, b.reds = moduleDir, r
	if deadline, ok := ctx.Deadline(); ok {
		b.budget = time.Until(deadline)
	}
	return b.red(r), nil
}

func eachRedOnTheBase(r baseattr.Reds) []baseattr.Key { return r.Keys }

func greenOnTheBase(baseattr.Reds) []baseattr.Key { return nil }

func baseRedTier(t *testing.T, red func(baseattr.Reds) []baseattr.Key) (core.PhaseRequest, *baseRun) {
	t.Helper()
	req, run, _ := baseRedTierInRepo(t, red)
	return req, run
}

func baseRedTierInRepo(t *testing.T, red func(baseattr.Reds) []baseattr.Key) (core.PhaseRequest, *baseRun, *gittest.Repo) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("go/go.mod", "module ciparitytest\n\ngo 1.23\n")
	repo.Git("add", "go/go.mod")
	repo.Git("commit", "-q", "-m", "base")
	lane := filepath.Join(t.TempDir(), "lane")
	repo.Git("worktree", "add", "-q", "-b", "lane", lane)
	runDir := filepath.Join(lane, ".evolve", "runs", "cycle-3")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"), []byte(`{"thrusts":[{"files_modified":["go/internal/widget/w.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fn, _, _ := seqRunFunc(t, []struct {
		Code int
		Out  string
	}{{1, "--- FAIL: TestNoisyFirst (0.00s)\n"}, {1, "--- FAIL: TestGenuine (0.01s)\nFAIL\tpkg\t2.0s\n"}})
	withFakeRunner(t, fn)
	run := &baseRun{red: red}
	orig := baseRedRunner
	baseRedRunner = func() baseattr.Runner { return run.run }
	t.Cleanup(func() { baseRedRunner = orig })
	return core.PhaseRequest{Cycle: 3, ProjectRoot: repo.Dir, Worktree: lane, Workspace: t.TempDir(), WorktreeBaseSHA: repo.Git("rev-parse", "HEAD")}, run, repo
}

func TestAuditOrchestration_IntegrationTier_ARedThatIsRedOnTheLaneBaseIsAWarningAndNotAFail(t *testing.T) {
	req, run := baseRedTier(t, eachRedOnTheBase)

	verdict, diags := classifyThroughProductionIntegrationTierGate(t, req)

	if verdict == core.VerdictFAIL || hasDiagContaining(diags, "skipped (could not run)") ||
		!hasDiagContaining(diags, "integration-tier gate: each red test of the integration tier is red on the lane base "+req.WorktreeBaseSHA+" too, so the audit does not blame the lane: pkg.TestGenuine") {
		t.Fatalf("verdict %q, diags %v; want no FAIL and the base-red warning of the integration-tier gate", verdict, diags)
	}
	scratch := filepath.Join(req.ProjectRoot, ".evolve", "ephemeral", "base-red")
	if resolved, err := filepath.EvalSymlinks(scratch); err == nil {
		scratch = resolved
	}
	moduleDir := run.moduleDir
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(filepath.Dir(moduleDir))); err == nil {
		moduleDir = filepath.Join(resolved, filepath.Base(filepath.Dir(moduleDir)), filepath.Base(moduleDir))
	}
	if run.calls != 1 || !strings.HasPrefix(moduleDir, scratch+string(filepath.Separator)) || filepath.Base(moduleDir) != "go" {
		t.Errorf("%d base run(s) in %q, want one run in the go module of a scratch tree under %q", run.calls, run.moduleDir, scratch)
	}
	if !slices.Equal(run.reds.Keys, []baseattr.Key{{Package: "pkg", Test: "TestGenuine"}}) || !slices.Equal(run.reds.Flags, []string{"-race", "-tags", "integration"}) {
		t.Errorf("the base run got %+v, want the red key of the retake with the flags of the tier", run.reds)
	}
	if run.budget <= 290*time.Second || run.budget > 300*time.Second {
		t.Errorf("the base run budget = %v, want the default of 300 s", run.budget)
	}
}

func TestAuditOrchestration_IntegrationTier_TheBaseRunBudgetIsThePolicyValue(t *testing.T) {
	req, run := baseRedTier(t, eachRedOnTheBase)
	if err := os.MkdirAll(filepath.Join(req.ProjectRoot, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(req.ProjectRoot, ".evolve", "policy.json"), []byte(`{"gates":{"base_red_rerun_seconds":40}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	verdict, diags := classifyThroughProductionIntegrationTierGate(t, req)

	if verdict == core.VerdictFAIL || run.budget <= 30*time.Second || run.budget > 40*time.Second {
		t.Errorf("verdict %q, base run budget %v, diags %v; want no FAIL and the 40 s of the policy", verdict, run.budget, diags)
	}
}

func TestAuditOrchestration_IntegrationTier_ARedThatIsGreenOnTheLaneBaseFails(t *testing.T) {
	req, run := baseRedTier(t, greenOnTheBase)

	verdict, diags := classifyThroughProductionIntegrationTierGate(t, req)

	if verdict != core.VerdictFAIL || !hasDiagContaining(diags, "TestGenuine") || run.calls != 1 {
		t.Errorf("verdict %q after %d base run(s), diags %v; want a FAIL that names TestGenuine after one base run", verdict, run.calls, diags)
	}
}

func TestAuditOrchestration_IntegrationTier_ALaneWithNoBaseGetsNoBaseRun(t *testing.T) {
	req, run := baseRedTier(t, eachRedOnTheBase)
	req.WorktreeBaseSHA = ""

	center := signalcenter.New()
	var codes []signalcenter.Code
	center.Subscribe(func(e signalcenter.Event) { codes = append(codes, e.Code) })

	off, err := ciParity{signals: func() *signalcenter.Center { return center }}.integrationTier(req)

	if err != nil || !strings.Contains(strings.Join(off, "\n"), "TestGenuine") || run.calls != 0 {
		t.Errorf("(%v, %v) after %d base run(s), want the offenders of the retake with no base run", off, err, run.calls)
	}
	if !slices.Equal(codes, []signalcenter.Code{ciparitygate.CodeGateFailed}) {
		t.Errorf("events = %v, want only the GATE_FAILED: a lane with no base asks for no attribution", codes)
	}
}

func TestBaseRedAttribution_ARequestWithNoProjectRootRunsInTheRepositoryOfTheWorktree(t *testing.T) {
	req, run := baseRedTier(t, eachRedOnTheBase)
	req.Worktree, req.ProjectRoot = req.ProjectRoot, ""

	v, err := baseRedAttribution(req)(context.Background(), baseattr.Reds{Keys: []baseattr.Key{{Package: "pkg", Test: "TestGenuine"}}})

	if err != nil || v.Blamed() || v.Base != req.WorktreeBaseSHA || run.calls != 1 {
		t.Errorf("verdict %+v, err %v after %d base run(s); want a base-red verdict at %s", v, err, run.calls, req.WorktreeBaseSHA)
	}
}

func TestBaseRedRunner_RunsInTheEnvironmentOfTheTier(t *testing.T) {
	t.Setenv("AUDIT_AMBIENT_PROBE", "1")
	dir := t.TempDir()
	probe := "package probe\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestAmbient(t *testing.T) {\n\tif os.Getenv(\"AUDIT_AMBIENT_PROBE\") != \"\" {\n\t\tt.Fatal(\"the ambient variable reached the base run\")\n\t}\n}\n"
	for name, body := range map[string]string{"go.mod": "module probe\n\ngo 1.23\n", "probe_test.go": probe} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	red, err := baseRedRunner()(context.Background(), dir, baseattr.Reds{Keys: []baseattr.Key{{Package: "probe", Test: "TestAmbient"}}})

	if err != nil || len(red) != 0 {
		t.Errorf("red = %v, err = %v; want a green base run, because the tier removes each ambient variable", red, err)
	}
}

func TestBaseRedAttribution_TheBaseRunIsAtTheLaneBaseWhenTheRootHasMovedOn(t *testing.T) {
	req, run, repo := baseRedTierInRepo(t, eachRedOnTheBase)
	repo.Git("commit", "-q", "--allow-empty", "-m", "main moves")

	v, err := baseRedAttribution(req)(context.Background(), baseattr.Reds{Keys: []baseattr.Key{{Package: "pkg", Test: "TestGenuine"}}})

	if err != nil || v.Base != req.WorktreeBaseSHA || run.calls == 0 {
		t.Errorf("verdict %+v, err %v after %d base run(s); want the base %s, not the head of the root", v, err, run.calls, req.WorktreeBaseSHA)
	}
}

func TestBaseRedAttribution_AMalformedPolicyWarnsOnStderr(t *testing.T) {
	req, _ := baseRedTier(t, eachRedOnTheBase)
	if err := os.MkdirAll(filepath.Join(req.ProjectRoot, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(req.ProjectRoot, ".evolve", "policy.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w

	_, attrErr := baseRedAttribution(req)(context.Background(), baseattr.Reds{Keys: []baseattr.Key{{Package: "pkg", Test: "TestGenuine"}}})

	os.Stderr = orig
	_ = w.Close()
	out, _ := io.ReadAll(r)
	if attrErr != nil || !strings.Contains(string(out), "[policy] WARN") {
		t.Errorf("stderr = %q, err = %v; want the policy warning of the malformed file", out, attrErr)
	}
}
