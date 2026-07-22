package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// size_budget_wiring_test.go — ADR-0076 slice A consumption: the cycle's size
// signal (triage-report.md via router.Digest — the LIVE artifact path, not the
// extinct handoff JSON) drives the build phase's budget scale and correction
// limit. Absent/unknown size is pinned to the exact legacy behavior.

func sizeBudgetRun(t *testing.T, reportBody string, completed []string) *cycleRun {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if reportBody != "" {
		if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(reportBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	return &cycleRun{
		o:     o,
		req:   CycleRequest{ProjectRoot: root},
		cycle: 42,
		cs:    CycleState{CycleID: 42, WorkspacePath: ws, CompletedPhases: completed},
	}
}

func TestBuildBudgetScale_FromTriageReport(t *testing.T) {
	cases := []struct {
		name   string
		report string
		want   float64
	}{
		{"large scales 1.5", "cycle_size_estimate: large\n", 1.5},
		{"medium scales 1.25", "cycle_size_estimate: medium\n", 1.25},
		{"small stays 1.0", "cycle_size_estimate: small\n", 1.0},
		{"unknown size stays 1.0", "cycle_size_estimate: gigantic\n", 1.0},
		{"no size line stays 1.0", "# Triage Decision\n", 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr := sizeBudgetRun(t, tc.report, []string{"triage"})
			if got := cr.buildBudgetScale(); got != tc.want {
				t.Errorf("buildBudgetScale() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildBudgetScale_AbsentReportIsUnscaled(t *testing.T) {
	cr := sizeBudgetRun(t, "", []string{"triage"})
	if got := cr.buildBudgetScale(); got != 1.0 {
		t.Errorf("no report → 1.0, got %v", got)
	}
	// Triage not completed yet: size unknown → unscaled (dispatch before triage
	// must never scale).
	cr2 := sizeBudgetRun(t, "cycle_size_estimate: large\n", nil)
	if got := cr2.buildBudgetScale(); got != 1.0 {
		t.Errorf("triage not completed → 1.0, got %v", got)
	}
}

// reportWriterRunner wraps a phase's fakeRunner to write an artifact into the
// LIVE workspace during the cycle — how a real triage phase delivers its
// report, so the composed proof exercises the true artifact path.
type reportWriterRunner struct {
	*fakeRunner
	fileName string
	body     string
}

func (r *reportWriterRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := os.WriteFile(filepath.Join(req.Workspace, r.fileName), []byte(r.body), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return r.fakeRunner.Run(ctx, req)
}

// TestSizeBudget_ComposedLargeCycleExtendsCorrectionLadder is the I2 wiring
// proof: a triage phase that DELIVERS `cycle_size_estimate: large` through the
// real artifact path buys the build phase one extra correction round (base 2 →
// 3) and stamps the scaled BudgetScale on every build dispatch. Contrast pin:
// TestLadder_BudgetsExhaust_CycleAbortsAsToday holds the unsized cycle at
// exactly 3 dispatches.
func TestSizeBudget_ComposedLargeCycleExtendsCorrectionLadder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rev := &recordingReviewer{
		default_: ReviewResult{Approve: true},
		decide:   map[string]ReviewResult{"build": {Approve: false, Reason: "still malformed"}},
	}
	fv := &fakeVerifier{destSub: filepath.Join("deliverables", "build-report.md")}
	o, runners := ladderOrchestrator(rev, fv, config.StageEnforce)
	runners[PhaseTriage] = &reportWriterRunner{
		fakeRunner: runners[PhaseTriage].(*fakeRunner),
		fileName:   "triage-report.md",
		body:       "# Triage Decision — Cycle X\n\ncycle_size_estimate: large\nphase_skip: []\n",
	}
	buildR := runners[PhaseBuild].(*fakeRunner)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err == nil {
		t.Fatal("expected abort after the ladder exhausts")
	}
	if want := "after 3 correction(s)"; !strings.Contains(err.Error(), want) {
		t.Errorf("large cycle must scale the ladder to 3: %v", err)
	}
	if buildR.calls != 4 {
		t.Errorf("build dispatched %d times, want 4 (initial + 3 scaled corrections)", buildR.calls)
	}
	for i, req := range buildR.requests {
		if req.BudgetScale != 1.5 {
			t.Errorf("build dispatch %d BudgetScale = %v, want 1.5", i, req.BudgetScale)
		}
	}
}

func TestScaledCorrectionLimit_BuildPhaseOnly(t *testing.T) {
	cr := sizeBudgetRun(t, "cycle_size_estimate: large\n", []string{"triage"})
	// Base default is 2; large (×1.5) → 3.
	if got := cr.correctionLimitFor(PhaseBuild, 2); got != 3 {
		t.Errorf("build large: correctionLimitFor = %d, want 3", got)
	}
	// Non-build phases never scale (A2 amendment: build-only scope).
	if got := cr.correctionLimitFor(PhaseAudit, 2); got != 2 {
		t.Errorf("audit must not scale: got %d, want 2", got)
	}
	// Small cycle: identity.
	crSmall := sizeBudgetRun(t, "cycle_size_estimate: small\n", []string{"triage"})
	if got := crSmall.correctionLimitFor(PhaseBuild, 2); got != 2 {
		t.Errorf("build small: got %d, want 2", got)
	}
}
