package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

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
			name: "wired-verdict-override",
			o:    withCat(phasespec.PhaseSpec{Name: "retrospective", BranchingStrategy: phasespec.BranchingVerdict}),
			p:    PhaseRetro,
			want: phasespec.BranchingVerdict,
		},
		{
			name: "entry-without-field-degrades",
			o:    withCat(phasespec.PhaseSpec{Name: "retrospective"}),
			p:    PhaseRetro,
			want: phasespec.BranchingHistory,
		},
		{
			name: "catalogless-retro-degrades",
			o:    NewOrchestrator(nil, nil, nil),
			p:    PhaseRetro,
			want: phasespec.BranchingHistory,
		},
		{
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

func TestRecordAndBranch_RetroDegradesToHistoryWhenUnconfigured(t *testing.T) {
	t.Parallel()
	cr := retroGateHarness(t, phasespec.Catalog{})

	dr := dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}, attemptCount: 1}
	if _, err := cr.recordAndBranch(PhaseRetro, dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}

	if cr.result.RetroDecision == "" {
		t.Error("catalog-less retro must degrade to the literal history branch " +
			"(failure-adapter consulted, RetroDecision set)")
	}
}

// resumeFromRetro builds a fake-backed orchestrator (optionally with a
// catalog) and resumes a cycle starting at retro, exercising resume.go's
// history-branch gate — the lockstep twin of recordAndBranch.
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

func TestResume_RetroDegradesToHistoryWhenUnconfigured(t *testing.T) {
	t.Parallel()
	res, err := resumeFromRetro(t)
	if err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if res.RetroDecision == "" {
		t.Error("catalog-less resume from retro must degrade to the literal history branch (RetroDecision set)")
	}
}

// TestResume_RetroGateIsStrategyKeyed tolerates a downstream transition error
// after the history branch is skipped; the contract under test is only that
// the branch did not fire.
func TestResume_RetroGateIsStrategyKeyed(t *testing.T) {
	t.Parallel()
	res, _ := resumeFromRetro(t, WithCatalog(mustCatalog(t,
		phasespec.PhaseSpec{Name: "retrospective", Optional: true, BranchingStrategy: phasespec.BranchingVerdict})))
	if res.RetroDecision != "" {
		t.Errorf("retro branching_strategy overridden to %q must SKIP the history branch on resume; RetroDecision = %q",
			phasespec.BranchingVerdict, res.RetroDecision)
	}
}
