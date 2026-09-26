package phasespec

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestApplyArchetypeDefaults_EvaluateFillsDefaults(t *testing.T) {
	s := &PhaseSpec{Name: "widget-scan", Role: "evaluate"}
	ApplyArchetypeDefaults(s)

	if !s.Optional {
		t.Error("evaluate overlay: Optional should be defaulted to true")
	}
	if len(s.PromptContext) != 1 || s.PromptContext[0] != "goal" {
		t.Errorf("PromptContext = %v, want [goal]", s.PromptContext)
	}
	if s.Classify == nil {
		t.Fatal("Classify should be allocated for an evaluate overlay")
	}
	if !s.Classify.FailIfEmpty {
		t.Error("Classify.FailIfEmpty should default to true")
	}
	if s.Classify.VerdictOnPass != "PASS" {
		t.Errorf("Classify.VerdictOnPass = %q, want PASS", s.Classify.VerdictOnPass)
	}
}

func TestApplyArchetypeDefaults_NonEvaluateNoOp(t *testing.T) {
	plan := &PhaseSpec{Name: "scout", Role: "plan"}
	ApplyArchetypeDefaults(plan)
	if plan.Optional {
		t.Error("plan archetype must not have Optional forced true")
	}
	if plan.Classify != nil {
		t.Error("plan archetype must not get an implicit Classify block")
	}
	if len(plan.PromptContext) != 0 {
		t.Errorf("plan archetype PromptContext = %v, want empty", plan.PromptContext)
	}

	// Also covers an evaluate spec: its explicit values are preserved, not overwritten.
	preset := &PhaseSpec{
		Name:          "audit",
		Role:          "evaluate",
		Optional:      true,
		PromptContext: []string{"build-report"},
		Classify:      &ClassifyRules{FailIfEmpty: true, VerdictOnPass: "PASS"},
	}
	ApplyArchetypeDefaults(preset)
	if len(preset.PromptContext) != 1 || preset.PromptContext[0] != "build-report" {
		t.Errorf("preset PromptContext = %v, want preserved [build-report]", preset.PromptContext)
	}
}

func TestRoots_DefaultAndOverride(t *testing.T) {
	root := t.TempDir()

	got := RootsWithPolicy(root, policy.PathsConfig{})
	wantDefault := filepath.Join(root, defaultRoot)
	if len(got) != 1 || got[0] != wantDefault {
		t.Fatalf("RootsWithPolicy(default) = %v, want [%s]", got, wantDefault)
	}

	// The "::" leaves an empty segment, which must be dropped.
	abs := filepath.Join(root, "abs-phases")
	got = RootsWithPolicy(root, policy.PathsConfig{PhaseRoots: "custom/phases::" + abs})
	wantRel := filepath.Join(root, "custom/phases")
	if len(got) != 2 {
		t.Fatalf("RootsWithPolicy(override) = %v, want 2 entries", got)
	}
	if got[0] != wantRel {
		t.Errorf("RootsWithPolicy(override)[0] = %q, want %q", got[0], wantRel)
	}
	if got[1] != abs {
		t.Errorf("RootsWithPolicy(override)[1] = %q, want %q (absolute kept verbatim)", got[1], abs)
	}
}

func TestIO_StructAndRoundTrip(t *testing.T) {
	want := IO{
		Files:   []string{"scout-report.md"},
		Signals: []string{"scout.cycle_size", "scout.item_count"},
	}

	spec := PhaseSpec{Name: "scout", Inputs: IO{Files: []string{"intent.md"}}, Outputs: want}

	if !reflect.DeepEqual(spec.Outputs, want) {
		t.Errorf("PhaseSpec.Outputs = %+v, want %+v", spec.Outputs, want)
	}
	if len(spec.Inputs.Files) != 1 || spec.Inputs.Files[0] != "intent.md" {
		t.Errorf("PhaseSpec.Inputs.Files = %v, want [intent.md]", spec.Inputs.Files)
	}
	if len(spec.Outputs.Signals) != 2 || spec.Outputs.Signals[1] != "scout.item_count" {
		t.Errorf("PhaseSpec.Outputs.Signals = %v, want [scout.cycle_size scout.item_count]", spec.Outputs.Signals)
	}
}
