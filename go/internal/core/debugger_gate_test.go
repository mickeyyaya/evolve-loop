package core

// debugger_gate_test.go — PA-BIG S3 (ADR-0058): the debugger decision-branch
// gate is config-driven, mirroring the retro history gate (S2). The debugger is
// a CONTROL phase with no registry home, so its branch metadata
// (branching_strategy: signal) comes from the builtinControlSpec seam
// (ADR-0058 §5), overlaid by Orchestrator.specFor with registry precedence and
// degrading to the literal phase-identity default (debugger→signal) backstop.

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestSuccessorStrategy_Debugger pins the debugger resolution: the control seam
// supplies "signal" even with no registry (proving the seam is consulted, not
// the phase name); a registry "debugger" entry OVERRIDES the seam (registry
// precedence). RED before S3 — phasespec.BranchingSignal does not yet exist.
func TestSuccessorStrategy_Debugger(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		o    *Orchestrator
		want string
	}{
		{
			// No registry at all → the control seam supplies signal.
			name: "seam-supplies-signal",
			o:    NewOrchestrator(nil, nil, nil),
			want: phasespec.BranchingSignal,
		},
		{
			// Registry "debugger" entry inverts to verdict → registry wins over
			// the seam (proving specFor consults config, not the phase name).
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

// TestBuiltinControlSpec asserts the control-phase seam: debugger gets a spec
// declaring signal branching; a non-control (registry) phase has no seam entry.
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

// debuggerGateHarness builds a minimal cycleRun positioned at the completed
// debugger phase, with the supplied catalog. Mirrors retroGateHarness:
// recordAndBranch's pre-gate steps are fake-safe (ledger/storage fakes, empty
// ActiveWorktree so normalizeBuildWorktree no-ops).
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

// TestRecordAndBranch_DebuggerGateIsStrategyKeyed proves the debugger gate
// consults successorStrategy, not the literal `current == PhaseDebugger`. A
// registry "debugger" entry overriding to verdict makes the gate SKIP the ENTIRE
// debugger block — so for EVERY decision signal no successor is scheduled and
// even BLOCK (which would loopBreak inside the block) does not. RED on the
// name-keyed gate (which fires for any debugger regardless of catalog).
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

// TestRecordAndBranch_DebuggerDegradesToSignalViaSeam is the byte-identity
// backstop for the seam-supplied default: with no registry "debugger" entry, the
// control seam supplies signal, the gate fires, and decideAfterDebugger routes
// each decision signal to its successor — identical to the pre-S3 name-keyed
// gate. (Cannot compile pre-S3, so "green" is asserted only post-S3.)
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

// TestNext_DebuggerSeamDoesNotLeakIntoVerdictBranch guards the subtle risk that
// overlaying the control seam in specFor (shared by sm.Next via WithCatalog)
// activates Next's verdict branch for debugger. The seam spec declares only
// branching_strategy (no on_pass/on_fail), so Next must stay on its literal
// debugger sentinel (→ end) for every verdict — byte-identical to the oracle.
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
