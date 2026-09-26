package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFailureDecisionWiring(t *testing.T) {
	sample := `{
  "category": "infra-systemic",
  "level": "system",
  "evidence": "audit self-declared a SYSTEM-class shared-state lost write; recorded FAIL",
  "justification": "the pipeline (not the task code) is the cause; the loop must halt and diagnose",
  "action": "halt-and-diagnose",
  "fix_type": "pipeline-repair",
  "schema_version": 1
}`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "failure-decision.json"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := readFailureDecision(dir)
	if err != nil {
		t.Fatalf("the instruction-shaped sample must parse without error: %v", err)
	}
	if d == nil {
		t.Fatal("the instruction-shaped sample must produce a non-nil decision (else the emitter/consumer schemas have drifted → inert API)")
	}
	if d.Action != policy.ActionHaltAndDiagnose {
		t.Errorf("Action = %q, want %q", d.Action, policy.ActionHaltAndDiagnose)
	}
	if d.Level != policy.LevelSystem {
		t.Errorf("Level = %q, want system", d.Level)
	}
	if d.Category != policy.CategoryInfraSystemic {
		t.Errorf("Category = %q, want infra-systemic", d.Category)
	}
}
