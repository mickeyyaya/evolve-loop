package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Layer 4 (ADR-0034): the host-side contract gate. Same verifier as the agent
// self-check, wired behind core.DeliverableReviewer. Fail-open on ambiguity,
// fail-closed on confirmed violation at enforce, with a circuit breaker that
// demotes enforce→advisory after N consecutive blocks so a miscalibrated gate
// cannot brick the loop.

func reviewInput(phase, workspace, projectRoot string) core.ReviewInput {
	return core.ReviewInput{Phase: phase, Workspace: workspace, ProjectRoot: projectRoot}
}

func newTestReviewer(stage config.Stage, breakerPath string, threshold int) *Reviewer {
	r := NewReviewer(stage).(*Reviewer)
	r.breakerPath = breakerPath
	r.threshold = threshold
	r.logf = func(string, ...any) {}
	return r
}

func newTestReviewerPhaseIO(stage, phaseIO config.Stage, breakerPath string, threshold int) *Reviewer {
	r := newTestReviewer(stage, breakerPath, threshold)
	r.phaseIO = phaseIO
	return r
}

// Phase 3.8 (ADR-0050): the generalized failure-context check blocks at the
// gate ONLY when BOTH ContractGate==enforce AND PhaseIO==enforce. A build report
// that self-reports FAIL without a structured failure block is blocked there,
// and approved (dormant) at every lower PhaseIO stage even while ContractGate
// enforces — so the rollout cannot false-block before the cutover.
func TestReviewer_FailureContextPhaseIO_BlocksOnlyAtBothEnforce(t *testing.T) {
	report := failReport("build", "## Changes", false)
	for _, tc := range []struct {
		phaseIO   config.Stage
		wantBlock bool
	}{
		{config.StageOff, false},
		{config.StageShadow, false},
		{config.StageAdvisory, false},
		{config.StageEnforce, true},
	} {
		t.Run(tc.phaseIO.String(), func(t *testing.T) {
			ws := t.TempDir()
			writeFile(t, ws, "build-report.md", report)
			r := newTestReviewerPhaseIO(config.StageEnforce, tc.phaseIO, filepath.Join(t.TempDir(), "b.json"), 3)
			got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir()))
			if tc.wantBlock && got.Approve {
				t.Fatalf("PhaseIO=%s, ContractGate=enforce: want BLOCK, got approve", tc.phaseIO)
			}
			if !tc.wantBlock && !got.Approve {
				t.Fatalf("PhaseIO=%s: failure-context check must be dormant, want approve, got block (%s)", tc.phaseIO, got.Reason)
			}
		})
	}
}

func TestReviewer_Off_ApprovesEverything(t *testing.T) {
	// Even a missing artifact is approved when the gate is off.
	r := newTestReviewer(config.StageOff, filepath.Join(t.TempDir(), "b.json"), 3)
	got := r.Review(context.Background(), reviewInput("build", t.TempDir(), t.TempDir()))
	if !got.Approve {
		t.Errorf("StageOff must approve; got %+v", got)
	}
}

func TestReviewer_Enforce_BlocksMissingArtifact(t *testing.T) {
	ws := t.TempDir()
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "b.json"), 3)
	got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir()))
	if got.Approve {
		t.Fatal("enforce must block a missing deliverable")
	}
	if got.Reason == "" {
		t.Error("rejection must carry a reason")
	}
}

func TestReviewer_Shadow_LogsButApproves(t *testing.T) {
	ws := t.TempDir()
	r := newTestReviewer(config.StageShadow, filepath.Join(t.TempDir(), "b.json"), 3)
	got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir()))
	if !got.Approve {
		t.Errorf("shadow must approve (log-only); got %+v", got)
	}
}

func TestReviewer_Enforce_ValidArtifact_Approves(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "b.json"), 3)
	if got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir())); !got.Approve {
		t.Errorf("valid deliverable must pass; got %+v", got)
	}
}

func TestReviewer_Ambiguity_FailsOpen(t *testing.T) {
	// Unknown phase → Verify returns error → gate fails OPEN even at enforce.
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "b.json"), 3)
	if got := r.Review(context.Background(), reviewInput("not-a-phase", t.TempDir(), t.TempDir())); !got.Approve {
		t.Errorf("ambiguity must fail open; got %+v", got)
	}
}

func TestReviewer_CircuitBreaker_DemotesAfterN(t *testing.T) {
	ws := t.TempDir() // empty → always violates
	pr := t.TempDir() // project root
	bp := filepath.Join(t.TempDir(), "breaker.json")
	r := newTestReviewer(config.StageEnforce, bp, 3)
	// First (threshold-1) violations BLOCK.
	for i := 1; i < 3; i++ {
		if got := r.Review(context.Background(), reviewInput("build", ws, pr)); got.Approve {
			t.Fatalf("block %d: enforce should still reject before the breaker opens", i)
		}
	}
	// The Nth consecutive violation OPENS the breaker → demote to approve.
	if got := r.Review(context.Background(), reviewInput("build", ws, pr)); !got.Approve {
		t.Errorf("circuit breaker should demote enforce→advisory at threshold; got %+v", got)
	}
}

func TestReviewer_CircuitBreaker_ResetsOnSuccess(t *testing.T) {
	wsBad := t.TempDir()
	wsGood := t.TempDir()
	writeFile(t, wsGood, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	pr := t.TempDir()
	bp := filepath.Join(t.TempDir(), "breaker.json")
	r := newTestReviewer(config.StageEnforce, bp, 3)
	r.Review(context.Background(), reviewInput("build", wsBad, pr))  // block, n=1
	r.Review(context.Background(), reviewInput("build", wsGood, pr)) // success → reset
	if n := readBreaker(bp); n != 0 {
		t.Errorf("breaker should reset to 0 after success; got %d", n)
	}
}

func TestReviewer_UsesEvolveDirForOrchestrator(t *testing.T) {
	pr := t.TempDir()
	ev := filepath.Join(pr, ".evolve")
	if err := os.MkdirAll(ev, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, ev, "cycle-state.json", `{"cycle_id":1,"phase":"tdd"}`)
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "b.json"), 3)
	if got := r.Review(context.Background(), reviewInput("orchestrator", "", pr)); !got.Approve {
		t.Errorf("orchestrator cycle-state.json under .evolve should validate; got %+v", got)
	}
}
