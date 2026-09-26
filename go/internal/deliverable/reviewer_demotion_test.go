package deliverable

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

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

func TestReviewer_ApprovalIsNeverDemoted(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)
	if got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir())); got.Demoted {
		t.Errorf("a compliant deliverable must not be flagged Demoted: %+v", got)
	}
}

func TestReviewer_ShadowApprovalIsNotDemotion(t *testing.T) {
	r := newTestReviewer(config.StageShadow, filepath.Join(t.TempDir(), "breaker.json"), 3)
	if got := r.Review(context.Background(), reviewInput("build", t.TempDir(), t.TempDir())); got.Demoted {
		t.Errorf("shadow stage is observe-only by design, not a demotion: %+v", got)
	}
}
