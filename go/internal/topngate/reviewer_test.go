package topngate

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestNewReviewer_EnforceApprovesLabelDrift(t *testing.T) {
	// POLICY CHANGE 2026-07-22 (cycles 916 + 1012): label drift is advisory —
	// even at enforce, a drifted label WARNs and passes; the committed set is
	// the binding authority. See gate_test.go's advisory case for rationale.
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
	r := NewReviewer(config.StageEnforce)
	res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
	if !res.Approve {
		t.Fatalf("label drift must approve (advisory); got reason=%q", res.Reason)
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
	// HISTORICAL REPLAY, updated 2026-07-22: cycle-640's wrong-lane build now
	// passes with a loud WARN instead of a fatal block — the 916/1012
	// evidence showed the fatal form discarded CORRECT work over label drift
	// between two LLM strings, while the cycle-640 fraud class is covered by
	// the queued scope-verification (deliverable files vs committed item
	// scope), which catches REAL wrong-work regardless of its label.
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
	r := NewReviewer(config.StageEnforce)
	res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
	if !res.Approve {
		t.Fatalf("label drift is advisory since 2026-07-22; got reason=%q", res.Reason)
	}
}
