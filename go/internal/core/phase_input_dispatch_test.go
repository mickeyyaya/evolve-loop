package core

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func phaseIOCfg(phaseIO config.Stage) config.RoutingConfig {
	cfg := shadowCfg(config.StageEnforce)
	cfg.PhaseIO = phaseIO
	return cfg
}

func TestDispatch_PhaseInput_ZeroValueBelowEnforce(t *testing.T) {
	t.Parallel()

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

func TestDispatch_PhaseInput_PopulatedAtEnforce(t *testing.T) {
	t.Parallel()

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

func TestBuildPhaseInput_ErrorContext_NilUnlessShipErrorKeys(t *testing.T) {
	t.Parallel()

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

func TestPlanCycle_CarryoverSummary_NoRawWriterBelowEnforce(t *testing.T) {
	t.Parallel()

	backlog := []CarryoverTodo{
		{ID: "todo-big-1", Action: strings.Repeat("a", phaseio.MaxFieldBytes*8), Priority: "high"},
		{ID: "todo-big-2", Action: strings.Repeat("b", phaseio.MaxFieldBytes*8), Priority: "high"},
	}
	for _, tc := range []struct {
		name  string
		stage config.Stage
	}{
		{"off", config.StageOff},
		{"shadow", config.StageShadow},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			st := &fakeStorage{state: State{LastCycleNumber: 0, CarryoverTodos: backlog}}
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
			dispatched := 0
			for phase, r := range runners {
				for i, req := range r.(*fakeRunner).requests {
					dispatched++
					if v, has := req.Context["carryover_summary"]; has {
						t.Errorf("PhaseIO=%s phase %s request[%d]: Context[carryover_summary] minted (%d bytes) — no raw carryover writer may exist below enforce", tc.name, phase, i, len(v))
					}
					if !reflect.DeepEqual(req.Input, phaseio.PhaseInput{}) {
						t.Errorf("PhaseIO=%s phase %s request[%d]: PhaseInput is not the zero value", tc.name, phase, i)
					}
				}
			}
			if dispatched == 0 {
				t.Fatal("no phase was dispatched — the assertion above ran vacuously")
			}
		})
	}
}
