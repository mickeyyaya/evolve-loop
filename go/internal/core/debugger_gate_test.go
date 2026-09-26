package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestSuccessorStrategy_Debugger(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		o    *Orchestrator
		want string
	}{
		{
			name: "seam-supplies-signal",
			o:    NewOrchestrator(nil, nil, nil),
			want: phasespec.BranchingSignal,
		},
		{
			name: "registry-overrides-seam",
			o: NewOrchestrator(nil, nil, nil, WithCatalog(mustCatalog(t,
				phasespec.PhaseSpec{Name: "debugger", BranchingStrategy: phasespec.BranchingVerdict}))),
			want: phasespec.BranchingVerdict,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.o.successorStrategy(PhaseDebugger); got != c.want {
				t.Errorf("successorStrategy(debugger) = %q, want %q", got, c.want)
			}
		})
	}
}

func TestBuiltinControlSpec(t *testing.T) {
	t.Parallel()
	spec, ok := builtinControlSpec(PhaseDebugger)
	if !ok {
		t.Fatal("builtinControlSpec(debugger) must return a control spec")
	}
	if spec.BranchingStrategy != phasespec.BranchingSignal {
		t.Errorf("debugger control spec branching = %q, want %q", spec.BranchingStrategy, phasespec.BranchingSignal)
	}
	if _, ok := builtinControlSpec(PhaseBuild); ok {
		t.Error("builtinControlSpec(build) must miss — build is a registry phase, not a control phase")
	}
}

// debuggerGateHarness leaves ActiveWorktree empty so normalizeBuildWorktree
// no-ops during recordAndBranch's pre-gate steps.
func debuggerGateHarness(t *testing.T, cat phasespec.Catalog) *cycleRun {
	t.Helper()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithCatalog(cat))
	return &cycleRun{
		o:       o,
		ctx:     context.Background(),
		req:     CycleRequest{ProjectRoot: t.TempDir()},
		cycle:   5,
		cs:      CycleState{WorkspacePath: t.TempDir()},
		current: PhaseDebugger,
		envSnap: map[string]string{},
	}
}

func TestRecordAndBranch_DebuggerGateIsStrategyKeyed(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"RESHIP", "RERUN_PHASE", "BLOCK"} {
		t.Run(action, func(t *testing.T) {
			cr := debuggerGateHarness(t, mustCatalog(t,
				phasespec.PhaseSpec{Name: "debugger", BranchingStrategy: phasespec.BranchingVerdict}))
			dr := dispatchResult{resp: PhaseResponse{Signals: map[string]any{"debugger.action": action}}, attemptCount: 1}
			act, err := cr.recordAndBranch(PhaseDebugger, dr)
			if err != nil {
				t.Fatalf("recordAndBranch: %v", err)
			}
			if cr.scheduledNext != "" {
				t.Errorf("strategy overridden to verdict must SKIP the signal branch for %q; "+
					"scheduledNext = %q (gate is name-keyed, not strategy-keyed)", action, cr.scheduledNext)
			}
			if act == loopBreak {
				t.Errorf("strategy overridden must skip the whole block; %q must not loopBreak", action)
			}
		})
	}
}

func TestRecordAndBranch_DebuggerDegradesToSignalViaSeam(t *testing.T) {
	t.Parallel()
	cases := []struct {
		action     string
		rerunPhase string
		wantNext   Phase
		wantBreak  bool
	}{
		{action: "RESHIP", wantNext: PhaseShip},
		{action: "RERUN_PHASE", rerunPhase: "audit", wantNext: PhaseAudit},
		{action: "BLOCK", wantBreak: true}, // → end → loopBreak, no successor scheduled
	}
	for _, c := range cases {
		t.Run(c.action, func(t *testing.T) {
			cr := debuggerGateHarness(t, phasespec.Catalog{}) // no entry → seam supplies signal
			sig := map[string]any{"debugger.action": c.action}
			if c.rerunPhase != "" {
				sig["debugger.rerun_phase"] = c.rerunPhase
			}
			dr := dispatchResult{resp: PhaseResponse{Signals: sig}, attemptCount: 1}
			act, err := cr.recordAndBranch(PhaseDebugger, dr)
			if err != nil {
				t.Fatalf("recordAndBranch: %v", err)
			}
			if c.wantBreak {
				if act != loopBreak {
					t.Errorf("%q via seam must loopBreak (decision→end); got action %v, scheduledNext %q",
						c.action, act, cr.scheduledNext)
				}
				return
			}
			if cr.scheduledNext != c.wantNext {
				t.Errorf("%q via seam must schedule %q through the signal branch; scheduledNext = %q",
					c.action, c.wantNext, cr.scheduledNext)
			}
		})
	}
}

func TestNext_DebuggerSeamDoesNotLeakIntoVerdictBranch(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(nil, nil, nil, WithCatalog(phasespec.Catalog{}))
	for _, v := range []string{"", VerdictPASS, VerdictWARN, VerdictFAIL} {
		got, err := o.sm.Next(PhaseDebugger, v)
		if got != PhaseEnd || err != nil {
			t.Errorf("Next(debugger,%q) = (%q,%v), want (end,nil) — control seam leaked into verdict branch", v, got, err)
		}
	}
}
