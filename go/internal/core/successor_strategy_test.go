package core

// successor_strategy_test.go — PA-BIG S2 (ADR-0058): the retro history-branch
// gate is config-driven. recordAndBranch/resume enter the retro
// failure-adapter branch when the phase's branching_strategy is "history",
// degrading to the literal phase-identity default (retro→history) when the
// catalog is unset or the field is absent — byte-identical to the pre-S2 flow.

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
)

// TestSuccessorStrategy pins the config read + degrade that the retro gate keys
// on. A wired catalog returns the descriptor's branching_strategy verbatim
// (proving the spec is consulted, not the phase name); an unset catalog OR a
// retrospective entry that omits the field degrades to the literal default
// (retro→history, every other phase→verdict/""), the byte-identity backstop.
func TestSuccessorStrategy(t *testing.T) {
	t.Parallel()

	withCat := func(specs ...phasespec.PhaseSpec) *Orchestrator {
		return NewOrchestrator(nil, nil, nil, WithCatalog(mustCatalog(t, specs...)))
	}

	cases := []struct {
		name string
		o    *Orchestrator
		p    Phase
		want string
	}{
		{
			name: "wired-history",
			o:    withCat(phasespec.PhaseSpec{Name: "retrospective", BranchingStrategy: phasespec.BranchingHistory}),
			p:    PhaseRetro,
			want: phasespec.BranchingHistory,
		},
		{
			// The catalog INVERTS retro to verdict-branching; the resolver must
			// return that (spec consulted), NOT the literal history default.
			name: "wired-verdict-override",
			o:    withCat(phasespec.PhaseSpec{Name: "retrospective", BranchingStrategy: phasespec.BranchingVerdict}),
			p:    PhaseRetro,
			want: phasespec.BranchingVerdict,
		},
		{
			// Entry present but field omitted → degrade to the literal default.
			name: "entry-without-field-degrades",
			o:    withCat(phasespec.PhaseSpec{Name: "retrospective"}),
			p:    PhaseRetro,
			want: phasespec.BranchingHistory,
		},
		{
			// No catalog at all → degrade to the literal default.
			name: "catalogless-retro-degrades",
			o:    NewOrchestrator(nil, nil, nil),
			p:    PhaseRetro,
			want: phasespec.BranchingHistory,
		},
		{
			// A non-retro phase has no history default → verdict-driven ("").
			name: "catalogless-nonretro-empty",
			o:    NewOrchestrator(nil, nil, nil),
			p:    PhaseBuild,
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.o.successorStrategy(c.p); got != c.want {
				t.Errorf("successorStrategy(%s) = %q, want %q", c.p, got, c.want)
			}
		})
	}
}

// retroGateHarness builds a minimal cycleRun positioned at the completed retro
// phase, with the supplied catalog. recordAndBranch's pre-gate steps are all
// fake-safe for retro: ledger/storage are fakes, ActiveWorktree is empty (so
// normalizeBuildWorktree no-ops), and there is no retro phase-binding case.
func retroGateHarness(t *testing.T, cat phasespec.Catalog) *cycleRun {
	t.Helper()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithCatalog(cat))
	return &cycleRun{
		o:       o,
		ctx:     context.Background(),
		req:     CycleRequest{ProjectRoot: t.TempDir()},
		cycle:   5,
		cs:      CycleState{WorkspacePath: t.TempDir()},
		current: PhaseRetro,
		envSnap: map[string]string{},
	}
}

// TestRecordAndBranch_RetroGateIsStrategyKeyed proves the gate consults
// successorStrategy, not the literal `current == PhaseRetro`. With the catalog
// overriding retro to verdict-branching, recordAndBranch must NOT take the
// failure-adapter history branch — RetroDecision stays empty, no successor is
// scheduled. RED on the name-keyed gate (which fires for any retro).
func TestRecordAndBranch_RetroGateIsStrategyKeyed(t *testing.T) {
	t.Parallel()
	cr := retroGateHarness(t, mustCatalog(t,
		phasespec.PhaseSpec{Name: "retrospective", Optional: true, BranchingStrategy: phasespec.BranchingVerdict}))

	dr := dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}, attemptCount: 1}
	if _, err := cr.recordAndBranch(PhaseRetro, dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}

	if cr.result.RetroDecision != "" {
		t.Errorf("retro branching_strategy overridden to %q must SKIP the history branch; "+
			"RetroDecision = %q (gate is name-keyed, not strategy-keyed)", phasespec.BranchingVerdict, cr.result.RetroDecision)
	}
	if cr.scheduledNext != "" {
		t.Errorf("skipped history branch must not schedule a successor; scheduledNext = %q", cr.scheduledNext)
	}
}

// TestRecordAndBranch_RetroDegradesToHistoryWhenUnconfigured is the
// byte-identity backstop: a catalog-less orchestrator keeps the literal retro
// history branch (the failure-adapter runs, RetroDecision is set). Green before
// AND after S2 — the safety net for synthetic/bare orchestrators.
func TestRecordAndBranch_RetroDegradesToHistoryWhenUnconfigured(t *testing.T) {
	t.Parallel()
	cr := retroGateHarness(t, phasespec.Catalog{}) // no retrospective entry → degrade

	dr := dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}, attemptCount: 1}
	if _, err := cr.recordAndBranch(PhaseRetro, dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}

	if cr.result.RetroDecision == "" {
		t.Error("catalog-less retro must degrade to the literal history branch " +
			"(failure-adapter consulted, RetroDecision set)")
	}
}

// resumeFromRetro builds a fake-backed orchestrator (optionally with a catalog)
// and resumes a cycle starting at retro, exercising resume.go's history-branch
// gate — the lockstep twin of recordAndBranch. ADR-0058 requires the retro
// branch to be byte-identity-covered on the resume path too, not just the live
// loop.
func resumeFromRetro(t *testing.T, opts ...Option) (CycleResult, error) {
	t.Helper()
	st := &fakeStorage{
		state:      State{LastCycleNumber: 5},
		cycleState: CycleState{CycleID: 5, WorkspacePath: "/tmp/ws"},
	}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), opts...)
	return o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(PhaseRetro), CycleID: 5})
}

// TestResume_RetroDegradesToHistoryWhenUnconfigured is the resume-path backstop:
// a catalog-less resume from retro keeps the literal history branch (retro PASS
// recovers to ship via the failure-adapter, RetroDecision set), completing
// cleanly. Green before AND after S2.
func TestResume_RetroDegradesToHistoryWhenUnconfigured(t *testing.T) {
	t.Parallel()
	res, err := resumeFromRetro(t) // no catalog → degrade to literal retro→history
	if err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if res.RetroDecision == "" {
		t.Error("catalog-less resume from retro must degrade to the literal history branch (RetroDecision set)")
	}
}

// TestResume_RetroGateIsStrategyKeyed proves resume.go's gate is config-driven,
// not name-keyed: with the catalog overriding retro to verdict-branching, resume
// must NOT take the failure-adapter history branch (RetroDecision empty). RED on
// the literal `current == PhaseRetro` gate. The cycle may stop with a downstream
// transition error after skipping the branch — expected; the contract under
// test is only that the history branch did not fire.
func TestResume_RetroGateIsStrategyKeyed(t *testing.T) {
	t.Parallel()
	res, _ := resumeFromRetro(t, WithCatalog(mustCatalog(t,
		phasespec.PhaseSpec{Name: "retrospective", Optional: true, BranchingStrategy: phasespec.BranchingVerdict})))
	if res.RetroDecision != "" {
		t.Errorf("retro branching_strategy overridden to %q must SKIP the history branch on resume; RetroDecision = %q",
			phasespec.BranchingVerdict, res.RetroDecision)
	}
}
