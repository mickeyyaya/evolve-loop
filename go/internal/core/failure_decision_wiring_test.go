package core

// failure_decision_wiring_test.go — cycle-1002 RED contract for ADR-0072 S4
// Task 4 (orchestrator-emits-failure-decision), Go half. The inert-API guard:
// the EXACT failure-decision.json shape the retrospective agent's instructions
// document must round-trip cleanly through the Task-2 reader — proving the
// emitter and consumer share one schema, so the consumption path is not
// permanently fallback-only. The instruction-file wiring proof (the emit
// directive + write-allowlist widening) is asserted by the ACS predicate
// C1002_005 against agents/evolve-retrospective.md. Fails RED until Builder
// adds readFailureDecision.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFailureDecisionWiring(t *testing.T) {
	// The canonical sample the retrospective instructions embed as their schema
	// exemplar — a full {category,level,evidence,justification,action,fix_type}
	// object over the policy vocabulary.
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
