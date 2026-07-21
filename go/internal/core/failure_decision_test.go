package core

// failure_decision_test.go — cycle-1002 RED contract for ADR-0072 S4 Task 2
// (failure-decision-schema-reader). The reader is the fallback boundary: a
// malformed / absent / schema-invalid artifact MUST yield (nil, nil) — the
// signal to fall back to the deterministic failureadapter — NEVER an error that
// aborts the cycle (retro_always_on_failure). Fails RED until Builder adds
// readFailureDecision + the failureDecision type in failure_decision.go.

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDecision(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "failure-decision.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadFailureDecision(t *testing.T) {
	// VALID: a well-formed decision with an in-vocabulary action + level parses
	// into a populated struct with no error.
	t.Run("valid_parses_into_struct", func(t *testing.T) {
		dir := t.TempDir()
		writeDecision(t, dir, `{
  "category": "code-audit-fail",
  "level": "task",
  "evidence": "audit reported 2 unresolved defects",
  "justification": "task-level code fault, retry with the audit findings addressed",
  "action": "retry-with-fix",
  "fix_type": "address-audit-findings",
  "schema_version": 1
}`)
		d, err := readFailureDecision(dir)
		if err != nil {
			t.Fatalf("valid decision must not error: %v", err)
		}
		if d == nil {
			t.Fatal("valid decision must return a non-nil struct")
		}
		if d.Category != "code-audit-fail" || d.Action != "retry-with-fix" || d.Level != "task" {
			t.Errorf("parsed fields wrong: category=%q action=%q level=%q", d.Category, d.Action, d.Level)
		}
		if d.FixType != "address-audit-findings" {
			t.Errorf("FixType = %q, want address-audit-findings", d.FixType)
		}
	})

	// ABSENT: no artifact → (nil, nil). This is the common fallback path (no
	// orchestrator ran, or a pre-S4 cycle) and MUST NOT be an error.
	t.Run("absent_falls_back_nil_nil", func(t *testing.T) {
		d, err := readFailureDecision(t.TempDir())
		if d != nil || err != nil {
			t.Errorf("absent artifact = (%v, %v), want (nil, nil)", d, err)
		}
	})

	// MALFORMED: unparseable JSON → (nil, nil), never a cycle-aborting error.
	t.Run("malformed_json_falls_back_nil_nil", func(t *testing.T) {
		dir := t.TempDir()
		writeDecision(t, dir, `{ this is not valid json `)
		d, err := readFailureDecision(dir)
		if d != nil || err != nil {
			t.Errorf("malformed artifact = (%v, %v), want (nil, nil)", d, err)
		}
	})

	// SCHEMA-INVALID (unknown action): well-formed JSON but the action is not in
	// the policy vocabulary → (nil, nil) fallback, not a struct that would drive
	// an unrecognized branch.
	t.Run("unknown_action_falls_back_nil_nil", func(t *testing.T) {
		dir := t.TempDir()
		writeDecision(t, dir, `{"category":"code-audit-fail","level":"task","action":"frobnicate"}`)
		d, err := readFailureDecision(dir)
		if d != nil || err != nil {
			t.Errorf("unknown-action artifact = (%v, %v), want (nil, nil)", d, err)
		}
	})

	// SCHEMA-INVALID (unknown level): level outside {system,task} → (nil, nil).
	t.Run("unknown_level_falls_back_nil_nil", func(t *testing.T) {
		dir := t.TempDir()
		writeDecision(t, dir, `{"category":"code-audit-fail","level":"galaxy","action":"retry-with-fix"}`)
		d, err := readFailureDecision(dir)
		if d != nil || err != nil {
			t.Errorf("unknown-level artifact = (%v, %v), want (nil, nil)", d, err)
		}
	})
}
