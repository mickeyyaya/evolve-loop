// quality_wiring_amplified_test.go — Test Amplifier (cycle 987).
//
// The TDD contract flags the exact blind spot this closes: gates_test.go
// exercises qualityGate{}.check() directly, never through
// NewReviewer(...).Review() — so a severed wire (qualityGate deleted from
// reviewer.go's composition slice) is invisible to the existing suite. The
// Builder's two binding tests (TestQualityGate_WiredIntoReviewer,
// TestNewReviewer_TautologyEvalBlocksAtEnforce) close that gap for the
// tautology-blocks-at-enforce case. These adversarial additions exercise the
// SAME wire from angles a stub or over-eager gate would fail even while
// passing the two canonical tests: stage-conditional gating (shadow must
// never block), the advisory/never-block contract for weak evals, a positive
// control (clean eval must not be blocked), and multi-eval aggregation.
// Written black-box against tdd-contract.md / build-report.md +
// gates_test.go's already-established qualityGate{} unit contract
// (tautology=block, weak/echo=advisory-never-block, behavioral=pass,
// missing-eval=fail-open-here) — reviewer.go/quality.go bodies and the
// Builder's new quality_wiring_test.go were deliberately not read.
package evalgate

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestNewReviewer_TautologyEvalAdvisoryAtShadow is the stage-conditional
// complement to the contracted enforce-blocks test. At StageShadow every
// violation is logged but approved (per reviewer.go's own doc comment: "at
// StageShadow every violation is logged but approved"). A gate wired to
// ALWAYS block regardless of stage — which would still pass the
// enforce-only contracted test — fails this one.
func TestNewReviewer_TautologyEvalAdvisoryAtShadow(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "taut")
	writeEval(t, root, "taut", ":")

	got := NewReviewer(config.StageShadow).Review(context.Background(), core.ReviewInput{
		Phase: "tdd", Workspace: ws, ProjectRoot: root,
	})
	if !got.Approve {
		t.Fatalf("StageShadow must log-and-approve, never block; got Approve=%v Reason=%q", got.Approve, got.Reason)
	}
}

// TestNewReviewer_WeakEvalNeverBlocksAtEnforce: a weak (echo) eval is
// advisory-only per qualityGate's own unit contract (TestQualityGate in
// gates_test.go: "weak eval must be advisory (block=false)"). Wired through
// NewReviewer at StageEnforce, it must still not block — this catches a
// wiring bug where the reviewer treats ANY non-empty gate reason as
// block-worthy instead of respecting the gate's own block=false decision.
func TestNewReviewer_WeakEvalNeverBlocksAtEnforce(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "weak")
	writeEval(t, root, "weak", "echo checking")

	got := NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase: "tdd", Workspace: ws, ProjectRoot: root,
	})
	if !got.Approve {
		t.Fatalf("weak/advisory eval must not block even at StageEnforce; got Approve=%v Reason=%q", got.Approve, got.Reason)
	}
}

// TestNewReviewer_BehavioralEvalPassesAtEnforce is the positive control: a
// legitimate behavioral eval must sail through the wired reviewer at
// StageEnforce. Without this, a "block everything" stub would falsely
// satisfy the tautology-blocks test.
func TestNewReviewer_BehavioralEvalPassesAtEnforce(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "real")
	writeEval(t, root, "real", "go test -race ./internal/real/...")

	got := NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase: "tdd", Workspace: ws, ProjectRoot: root,
	})
	if !got.Approve {
		t.Fatalf("behavioral eval must pass at StageEnforce; got Approve=%v Reason=%q", got.Approve, got.Reason)
	}
}

// TestNewReviewer_MultipleEvalsOneTautology_BlocksNamingSlug is the
// multi-eval / limit-style case: with several selected slugs and only one
// tautological, the wired reviewer must still block (aggregating across all
// composed gates and all selected evals, not just the first checked) and the
// rejection reason must name the OFFENDING slug, not the clean one.
func TestNewReviewer_MultipleEvalsOneTautology_BlocksNamingSlug(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "real", "taut")
	writeEval(t, root, "real", "go test -race ./internal/real/...")
	writeEval(t, root, "taut", ":")

	got := NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase: "tdd", Workspace: ws, ProjectRoot: root,
	})
	if got.Approve {
		t.Fatalf("one tautological eval among several must still block at StageEnforce; got Approve=%v", got.Approve)
	}
	if !strings.Contains(got.Reason, "taut") {
		t.Errorf("rejection reason should name the offending slug 'taut'; got %q", got.Reason)
	}
	if strings.Contains(got.Reason, "real,") || strings.Contains(got.Reason, ", real") {
		t.Errorf("rejection reason should not implicate the clean slug 'real'; got %q", got.Reason)
	}
}

// TestNewReviewer_MissingEvalAtEnforce_QualityGateFailsOpen exercises the
// missing-eval path through the FULL wire, not just qualityGate{}.check()
// (gates_test.go already covers the unit level: "missing eval is Gate A's
// job → fail-open here"). materializationGate applies to phase "scout" only
// (per its own appliesTo contract), so at phase "tdd" with no eval file at
// all, qualityGate must fail open and the composed reviewer must approve —
// proving the phase-scoped gate composition, not just qualityGate in
// isolation.
func TestNewReviewer_MissingEvalAtEnforce_QualityGateFailsOpen(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "gone") // no eval written for "gone"

	got := NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase: "tdd", Workspace: ws, ProjectRoot: root,
	})
	if !got.Approve {
		t.Fatalf("missing eval at phase=tdd is Gate A's (materializationGate, scout-only) job, not qualityGate's; want fail-open Approve=true, got Approve=%v Reason=%q", got.Approve, got.Reason)
	}
}
