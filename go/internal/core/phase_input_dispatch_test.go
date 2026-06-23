package core

import (
	"context"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/phaseio"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// phaseIOCfg is shadowCfg with the EVOLVE_PHASE_IO dial set independently of the
// routing Stage, so a test can vary PhaseIO while the routing decision stays a
// deterministic StaticPreset (isolating the variable under test to the dial).
func phaseIOCfg(phaseIO config.Stage) config.RoutingConfig {
	cfg := shadowCfg(config.StageEnforce)
	cfg.PhaseIO = phaseIO
	return cfg
}

// TestDispatch_PhaseInput_ZeroValueBelowEnforce is the Slice-0 byte-identity
// proof (ADR-0050 Phase 3.10): at EVOLVE_PHASE_IO off/shadow/advisory the typed
// PhaseInput envelope is NEVER assembled onto the PhaseRequest, so every
// dispatched request carries the zero PhaseInput. The shadow stage still runs its
// comparison (3.4) — that is unchanged — but the dispatch *field* stays zero until
// enforce, which is what keeps the live loop byte-identical pre-cutover.
//
// The check uses reflect.DeepEqual against a struct literal (PhaseInput seals
// channels behind unexported fields, so it is not ==-comparable). This is valid
// because the guard lives in the CODE PATH, not the comparison: assemblePhaseIO
// returns early (before any NewPhaseInput call) below enforce, so the field is the
// literal zero value here — not a NewPhaseInput(empty) result that merely looks
// zero.
func TestDispatch_PhaseInput_ZeroValueBelowEnforce(t *testing.T) {
	cases := []struct {
		name  string
		stage config.Stage
	}{
		{"off", config.StageOff},
		{"shadow", config.StageShadow},
		{"advisory", config.StageAdvisory},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			st := &fakeStorage{state: State{LastCycleNumber: 0}}
			runners := buildRunners(nil)
			o := NewOrchestrator(st, &fakeLedger{}, runners,
				WithRouting(phaseIOCfg(tc.stage), router.StaticPreset{}),
				WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))
			if _, err := o.RunCycle(context.Background(), CycleRequest{
				ProjectRoot: t.TempDir(),
				GoalHash:    "s0",
				Context:     map[string]string{"goal": "g", "strategy": "profile-first"},
			}); err != nil {
				t.Fatalf("RunCycle: %v", err)
			}
			for phase, r := range runners {
				for i, req := range r.(*fakeRunner).requests {
					if !reflect.DeepEqual(req.Input, phaseio.PhaseInput{}) {
						t.Errorf("PhaseIO=%s phase %s request[%d]: PhaseInput is not the zero value — dispatch must stay byte-identical below enforce", tc.name, phase, i)
					}
				}
			}
		})
	}
}

// TestDispatch_PhaseInput_PopulatedAtEnforce: at EVOLVE_PHASE_IO=enforce the typed
// envelope becomes authoritative — every dispatched request carries a populated
// PhaseInput whose CycleInputs reproduce the legacy Context values, with identity
// fields set and (on a non-recovery cycle) no ErrorContext.
func TestDispatch_PhaseInput_PopulatedAtEnforce(t *testing.T) {
	const goal, strategy = "cut p99 latency", "profile-first"
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners,
		WithRouting(phaseIOCfg(config.StageEnforce), router.StaticPreset{}),
		WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "s0e",
		Context:     map[string]string{"goal": goal, "strategy": strategy},
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	seen := 0
	for phase, r := range runners {
		for i, req := range r.(*fakeRunner).requests {
			seen++
			in := req.Input
			if got := in.CycleInputs().Goal(); got != goal {
				t.Errorf("phase %s request[%d]: PhaseInput.CycleInputs().Goal()=%q, want %q", phase, i, got, goal)
			}
			if got := in.CycleInputs().Strategy(); got != strategy {
				t.Errorf("phase %s request[%d]: Strategy()=%q, want %q", phase, i, got, strategy)
			}
			if in.Cycle == 0 || in.Phase == "" {
				t.Errorf("phase %s request[%d]: identity not populated (Cycle=%d Phase=%q)", phase, i, in.Cycle, in.Phase)
			}
			if _, hasErr := in.ErrorContext(); hasErr {
				t.Errorf("phase %s request[%d]: ErrorContext present on a non-recovery cycle", phase, i)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no phase requests captured — harness regression")
	}
}

// TestBuildPhaseInput_ErrorContext_NilUnlessShipErrorKeys pins the critic's
// invariant directly on the assembler: ErrorContext is non-nil ONLY when the
// ship-error recovery keys are present in the phase context (the debugger
// recovery path), and nil otherwise. The integration test above proves the
// nil case across a whole cycle; this proves the non-nil mapping without having
// to drive a real ship failure.
func TestBuildPhaseInput_ErrorContext_NilUnlessShipErrorKeys(t *testing.T) {
	cr := &cycleRun{
		cycle:   7,
		cs:      CycleState{RunID: "r", WorkspacePath: "/ws"},
		req:     CycleRequest{GoalHash: "g", ProjectRoot: "/pr"},
		current: PhaseShip,
		envSnap: map[string]string{},
	}
	in := cr.buildPhaseInput(PhaseDebugger, "/wt", map[string]string{"goal": "g"}, phaseio.Handoffs{})
	if _, ok := in.ErrorContext(); ok {
		t.Fatal("ErrorContext present without ship_error_* keys (non-recovery phase)")
	}
	recoveryCtx := map[string]string{
		"ship_error_code":  "E_PUSH_NONFF",
		"ship_error_class": "transient",
		"ship_error_stage": "ship",
		"ship_error_debug": "remote moved",
	}
	in2 := cr.buildPhaseInput(PhaseDebugger, "/wt", recoveryCtx, phaseio.Handoffs{})
	ec, ok := in2.ErrorContext()
	if !ok {
		t.Fatal("ErrorContext absent on recovery path with ship_error_* keys")
	}
	if ec.Code != "E_PUSH_NONFF" || ec.Class != "transient" || ec.Stage != "ship" || ec.Debug != "remote moved" {
		t.Errorf("ErrorContext mismapped from ship_error_* keys: %+v", ec)
	}
}
