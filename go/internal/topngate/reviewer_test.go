package topngate

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestNewReviewer_EnforceBlocksOutOfLaneBuild(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
	r := NewReviewer(config.StageEnforce)
	res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
	if res.Approve {
		t.Fatal("enforce must block a build report whose slug is outside triage top_n")
	}
	if res.Reason == "" {
		t.Error("a blocked review must record a non-empty abort_reason (ADR-0044 C1)")
	}
}

func TestNewReviewer_EnforceApprovesInLaneBuild(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "statefile-rmw-flock-single-source")
	r := NewReviewer(config.StageEnforce)
	res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
	if !res.Approve {
		t.Errorf("enforce must approve an in-lane build; got reason=%q", res.Reason)
	}
}

func TestNewReviewer_ShadowLogsButApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
	r := NewReviewer(config.StageShadow)
	res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
	if !res.Approve {
		t.Fatalf("shadow must approve even on violation; got Approve=false (%s)", res.Reason)
	}
}

func TestNewReviewer_NonBuildPhaseApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	r := NewReviewer(config.StageEnforce)
	// audit phase: the gate only applies to build's deliverable → approve.
	if res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseAudit), Workspace: ws}); !res.Approve {
		t.Errorf("gate must not apply to phase audit; want approve, got reason=%q", res.Reason)
	}
}

// TestReplayCycle640Shape is a direct regression test for the 7th (and
// intended-final) recurrence of this defect: cycle 640 triage committed
// exactly "statefile-rmw-flock-single-source", TDD authored predicates for
// it, but Builder instead implemented "fix-token-resolver-transcript-source"
// (the OTHER fleet lane's goal). Audit graded the delivered diff PASS 0.93
// while the ACS suite bound to the committed task returned FAIL, red_count=9,
// ship_eligible=false (.evolve/runs/cycle-640/retrospective-report.md +
// stage-lesson-1.yaml). This test replays that exact shape and asserts the
// gate now blocks BEFORE audit ever ran, instead of consuming a full
// audit+ship phase pair on a cycle doomed from the build->audit transition.
func TestReplayCycle640Shape(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
	r := NewReviewer(config.StageEnforce)
	res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
	if res.Approve {
		t.Fatal("cycle-640 replay must be blocked at the build->audit transition, not allowed through to audit")
	}
	if !strings.Contains(res.Reason, "fix-token-resolver-transcript-source") {
		t.Errorf("abort_reason must name the wrong-task slug so operators can diagnose without re-deriving it; got %q", res.Reason)
	}
}
