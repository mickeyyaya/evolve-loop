package phaseio

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestPhaseInput_SealedContext_NoMutation is the named RED anchor for 3.1: the
// Env channel is sealed — mutating the init map after construction, or mutating
// a map returned by EnvCopy, must not change the input a phase observes. This
// is the "no shared mutable state between filters" guarantee (P4/P5).
func TestPhaseInput_SealedContext_NoMutation(t *testing.T) {
	env := map[string]string{"EVOLVE_PHASE_IO": "shadow"}
	in := NewPhaseInput(PhaseInputInit{
		Phase: "build",
		Env:   env,
	})

	// (1) Mutating the source map after construction must not leak in.
	env["EVOLVE_PHASE_IO"] = "enforce"
	env["INJECTED"] = "x"
	if v, ok := in.Env("EVOLVE_PHASE_IO"); !ok || v != "shadow" {
		t.Fatalf("Env leaked source mutation: (%q, %v)", v, ok)
	}
	if _, ok := in.Env("INJECTED"); ok {
		t.Fatalf("Env leaked source insertion")
	}

	// (2) Mutating the EnvCopy result must not affect subsequent reads.
	cp := in.EnvCopy()
	cp["EVOLVE_PHASE_IO"] = "enforce"
	delete(cp, "EVOLVE_PHASE_IO")
	if v, ok := in.Env("EVOLVE_PHASE_IO"); !ok || v != "shadow" {
		t.Fatalf("Env leaked EnvCopy mutation: (%q, %v)", v, ok)
	}
}

// TestPhaseInput_ZeroValue_Safe pins the documented contract that the zero
// PhaseInput is safe to read (every accessor is nil-tolerant).
func TestPhaseInput_ZeroValue_Safe(t *testing.T) {
	var in PhaseInput
	if _, ok := in.Env("x"); ok {
		t.Errorf("zero PhaseInput Env ok=true")
	}
	if in.Spec() != nil {
		t.Errorf("zero PhaseInput Spec non-nil")
	}
	if _, ok := in.ErrorContext(); ok {
		t.Errorf("zero PhaseInput ErrorContext ok=true")
	}
	if _, ok := in.Correction(); ok {
		t.Errorf("zero PhaseInput Correction ok=true")
	}
	if _, ok := in.Upstream().Scout(); ok {
		t.Errorf("zero PhaseInput Upstream has scout")
	}
	if in.CycleInputs().Goal() != "" {
		t.Errorf("zero PhaseInput CycleInputs non-empty")
	}
}

func TestPhaseInput_Identity_Fields(t *testing.T) {
	in := NewPhaseInput(PhaseInputInit{
		Cycle:            42,
		RunID:            "r-abc",
		GoalHash:         "deadbeef",
		ProjectRoot:      "/repo",
		Workspace:        "/repo/.evolve/runs/cycle-42",
		Worktree:         "/wt/cycle-42",
		Phase:            "audit",
		PreviousPhase:    "build",
		WorktreeWritable: true,
	})
	if in.Cycle != 42 || in.RunID != "r-abc" || in.GoalHash != "deadbeef" ||
		in.ProjectRoot != "/repo" || in.Workspace != "/repo/.evolve/runs/cycle-42" ||
		in.Worktree != "/wt/cycle-42" || in.Phase != "audit" || in.PreviousPhase != "build" ||
		!in.WorktreeWritable {
		t.Fatalf("identity round-trip mismatch: %+v", in)
	}
}

func TestPhaseInput_Env_AbsentKey(t *testing.T) {
	in := NewPhaseInput(PhaseInputInit{})
	if v, ok := in.Env("NOPE"); ok || v != "" {
		t.Fatalf("absent env key: (%q, %v)", v, ok)
	}
	cp := in.EnvCopy()
	if cp == nil {
		t.Fatalf("EnvCopy must return a non-nil (empty) map even when Env is nil")
	}
	if len(cp) != 0 {
		t.Fatalf("EnvCopy of empty: %v", cp)
	}
}

// TestPhaseInput_Sealed_NoMutationViaInit is the missing half of the seal proof:
// the typed Error/Correction channels must be deep-copied at construction so a
// caller that retains and mutates the init pointer cannot change what a phase
// observes. (Mirrors TestHandoffs_Sealed_NoMutationViaInitOrAccessor.)
func TestPhaseInput_Sealed_NoMutationViaInit(t *testing.T) {
	ec := &ErrorContext{Code: "E1", Class: "transient", Stage: "ship", Debug: "d"}
	cs := &CorrectionState{Directive: "redispatch", Attempt: 1}
	in := NewPhaseInput(PhaseInputInit{Error: ec, Correction: cs})

	// Mutate the init pointers AFTER construction.
	ec.Code = "MUTATED"
	ec.Stage = "MUTATED"
	cs.Directive = "MUTATED"
	cs.Attempt = 99

	gotEC, _ := in.ErrorContext()
	if gotEC.Code != "E1" || gotEC.Stage != "ship" {
		t.Fatalf("ErrorContext leaked init-pointer mutation: %+v", gotEC)
	}
	gotCS, _ := in.Correction()
	if gotCS.Directive != "redispatch" || gotCS.Attempt != 1 {
		t.Fatalf("Correction leaked init-pointer mutation: %+v", gotCS)
	}
}

func TestPhaseInput_Spec_PassthroughPointer(t *testing.T) {
	spec := &phasespec.PhaseSpec{Name: "build", Kind: "llm"}
	in := NewPhaseInput(PhaseInputInit{Spec: spec})
	got := in.Spec()
	if got == nil || got.Name != "build" {
		t.Fatalf("Spec passthrough: %+v", got)
	}
	// Absent spec is nil.
	if NewPhaseInput(PhaseInputInit{}).Spec() != nil {
		t.Fatalf("absent Spec should be nil")
	}
}

func TestPhaseInput_Upstream_And_CycleInputs_Passthrough(t *testing.T) {
	up := NewHandoffs(HandoffsInit{Scout: &ScoutView{CycleSizeEstimate: "small"}})
	ci := NewCycleInputs(CycleInputsInit{Goal: "g"})
	in := NewPhaseInput(PhaseInputInit{Upstream: up, CycleInputs: ci})

	if s, ok := in.Upstream().Scout(); !ok || s.CycleSizeEstimate != "small" {
		t.Fatalf("Upstream passthrough: (%+v, %v)", s, ok)
	}
	if in.CycleInputs().Goal() != "g" {
		t.Fatalf("CycleInputs passthrough: %q", in.CycleInputs().Goal())
	}
}

func TestPhaseInput_ErrorContext_PresentAndAbsent(t *testing.T) {
	in := NewPhaseInput(PhaseInputInit{
		Error: &ErrorContext{Code: "E_PUSH", Class: "transient", Stage: "ship", Debug: "non-ff"},
	})
	ec, ok := in.ErrorContext()
	if !ok || ec.Code != "E_PUSH" || ec.Class != "transient" || ec.Stage != "ship" || ec.Debug != "non-ff" {
		t.Fatalf("ErrorContext present: (%+v, %v)", ec, ok)
	}
	if _, ok := NewPhaseInput(PhaseInputInit{}).ErrorContext(); ok {
		t.Fatalf("absent ErrorContext: ok=true, want false")
	}
}

func TestPhaseInput_Correction_PresentAndAbsent(t *testing.T) {
	in := NewPhaseInput(PhaseInputInit{
		Correction: &CorrectionState{Directive: "re-emit deliverable", Attempt: 2},
	})
	cs, ok := in.Correction()
	if !ok || cs.Directive != "re-emit deliverable" || cs.Attempt != 2 {
		t.Fatalf("Correction present: (%+v, %v)", cs, ok)
	}
	if _, ok := NewPhaseInput(PhaseInputInit{}).Correction(); ok {
		t.Fatalf("absent Correction: ok=true, want false")
	}
}

// TestPhaseInput_ErrorContext_SealedByValue proves the typed channels are
// returned by value: a caller mutating the returned struct cannot reach back
// into the sealed input.
func TestPhaseInput_ErrorContext_SealedByValue(t *testing.T) {
	in := NewPhaseInput(PhaseInputInit{Error: &ErrorContext{Code: "E1"}})
	ec, _ := in.ErrorContext()
	ec.Code = "MUTATED"
	again, _ := in.ErrorContext()
	if again.Code != "E1" {
		t.Fatalf("ErrorContext not sealed by value: %q", again.Code)
	}
}
