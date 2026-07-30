package deliverable

// reviewer_demotion_test.go — the contract gate must ANNOUNCE its own
// demotion (inbox contract-block-cli-escalation, P1 0.95).
//
// Before this, a circuit-open returned ReviewResult{Approve:true} — structurally
// indistinguishable from a deliverable that actually satisfied its contract. The
// orchestrator therefore could not tell "the gate passed you" from "the gate
// gave up on you", which is why the batch-19 and batch-21 demotions were
// invisible outside one stderr line.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// TestReviewer_CircuitBreaker_MarksDemoted pins the Demoted signal on the REAL
// production reviewer: blocks below the threshold are plain rejections; the
// threshold block approves AND flags Demoted, carrying the violation reason so
// the orchestrator's WARN can name what the gate stopped enforcing.
func TestReviewer_CircuitBreaker_MarksDemoted(t *testing.T) {
	ws := t.TempDir() // empty → every Verify violates
	pr := t.TempDir()
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)

	for i := 1; i < 3; i++ {
		got := r.Review(context.Background(), reviewInput("build", ws, pr))
		if got.Approve {
			t.Fatalf("block %d: enforce must still reject before the breaker opens", i)
		}
		if got.Demoted {
			t.Errorf("block %d: Demoted must stay false while the gate is still enforcing (%+v)", i, got)
		}
	}

	got := r.Review(context.Background(), reviewInput("build", ws, pr))
	if !got.Approve {
		t.Fatalf("threshold block must demote enforce→advisory; got %+v", got)
	}
	if !got.Demoted {
		t.Error("circuit open returned Approve without Demoted — the demotion is indistinguishable from a compliant deliverable")
	}
	if got.Reason == "" {
		t.Error("a demoting result must carry the violation reason so the operator WARN can name what stopped being enforced")
	}
}

// TestReviewer_ApprovalIsNeverDemoted is the anti-false-positive guard: a
// deliverable that genuinely satisfies its contract must never be reported as a
// demotion, or every clean cycle would file a bogus escalation.
func TestReviewer_ApprovalIsNeverDemoted(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)
	if got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir())); got.Demoted {
		t.Errorf("a compliant deliverable must not be flagged Demoted: %+v", got)
	}
}

// TestReviewer_ShadowApprovalIsNotDemotion pins the stage axis: below enforce
// the gate is observe-only by DESIGN, not demoted-under-duress. Reporting a
// shadow approval as a demotion would file an escalation on every cycle of a
// deliberate shadow rollout.
func TestReviewer_ShadowApprovalIsNotDemotion(t *testing.T) {
	r := newTestReviewer(config.StageShadow, filepath.Join(t.TempDir(), "breaker.json"), 3)
	if got := r.Review(context.Background(), reviewInput("build", t.TempDir(), t.TempDir())); got.Demoted {
		t.Errorf("shadow stage is observe-only by design, not a demotion: %+v", got)
	}
}
