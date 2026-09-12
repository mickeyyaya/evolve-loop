package core

// resume_review_parity_test.go — ADR-0100 §4: the resume loop's review skip
// set is identical to the fresh loop's.
//
// reviewResumedDeliverable skipped review entirely when the checkpoint's
// ExplanationDocumentationVersion was 0 ("legacy checkpoints retain their
// historical behavior"). The explanation reviewer already delegates on
// version 0 (mandatoryExplanationReviewer), so that skip protected nothing
// it needed to — and it silently exempted every OTHER reviewer (the contract
// gate, the declared-deliverables gate) for any resumed legacy cycle.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCycleFromPhase_LegacyCheckpointIsStillReviewed(t *testing.T) {
	projectRoot := t.TempDir()
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	st := &fakeStorage{
		state: State{LastCycleNumber: 9},
		// ExplanationDocumentationVersion deliberately 0: a checkpoint written
		// before the explanation contract existed.
		cycleState: CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: t.TempDir()},
	}
	reviews := &phaseReviewCounter{}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil),
		WithWorktreeProvisioner(&fakeWorktree{path: st.cycleState.ActiveWorktree}),
		WithReviewer(reviews))

	if _, err := o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: projectRoot, GoalHash: "g"},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if got := reviews.byPhase[string(PhaseAudit)]; got != 1 {
		t.Fatalf("a resumed legacy (version-0) cycle's audit was reviewed %d time(s), want 1 — the fresh loop reviews every non-SKIPPED deliverable regardless of the explanation contract; the resume twin must too", got)
	}
}

// gateMarker stands in for the production contract gate (which core cannot
// import): it satisfies the declaredDeliverablesGate capability.
type gateMarker struct{ on bool }

func (g gateMarker) Review(context.Context, ReviewInput) ReviewResult {
	return ReviewResult{Approve: true}
}
func (g gateMarker) VerifiesDeclaredDeliverables() bool { return g.on }

// TestDeclaredDeliverablesGateWired pins the predicate the composition-root
// proof relies on: it must see through the mandatory-explanation wrapper and
// chain nesting, and must not be fooled by a gate that reports itself off.
func TestDeclaredDeliverablesGateWired(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []Option
		want bool
	}{
		{"no reviewer", nil, false},
		{"plain reviewer without the capability", []Option{WithReviewer(&phaseReviewCounter{})}, false},
		{"gate alone", []Option{WithReviewer(gateMarker{on: true})}, true},
		{"gate nested in a chain", []Option{WithReviewer(ChainReviewers(&phaseReviewCounter{}, gateMarker{on: true}))}, true},
		{"gate switched off", []Option{WithReviewer(ChainReviewers(gateMarker{on: false}))}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), tc.opts...)
			if got := o.DeclaredDeliverablesGateWired(); got != tc.want {
				t.Fatalf("DeclaredDeliverablesGateWired() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBuildFloorReviewer_LegacyCheckpointVersionZeroIsExempt pins the other
// version-consuming reviewer: with the resume skip gone, a legacy (version-0)
// checkpoint now reaches the production build-floor reviewer, whose
// explanation check must treat 0 as "contract not active" (explanationdocs.
// CheckBuild returns nil when the binding is inactive) rather than as a
// contract the cycle never had.
func TestBuildFloorReviewer_LegacyCheckpointVersionZeroIsExempt(t *testing.T) {
	r := NewBuildFloorReviewer(nil) // no deterministic checks: only the explanation floor could reject
	res := r.Review(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Cycle: 9, Workspace: t.TempDir(), Worktree: t.TempDir(), ProjectRoot: t.TempDir(),
		ExplanationDocumentationVersion: 0,
	})
	if !res.Approve {
		t.Fatalf("a version-0 checkpoint must not be rejected on the explanation contract it never had: %+v", res)
	}
}
