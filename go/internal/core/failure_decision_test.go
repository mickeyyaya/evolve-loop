package core

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

	t.Run("absent_falls_back_nil_nil", func(t *testing.T) {
		d, err := readFailureDecision(t.TempDir())
		if d != nil || err != nil {
			t.Errorf("absent artifact = (%v, %v), want (nil, nil)", d, err)
		}
	})

	t.Run("malformed_json_falls_back_nil_nil", func(t *testing.T) {
		dir := t.TempDir()
		writeDecision(t, dir, `{ this is not valid json `)
		d, err := readFailureDecision(dir)
		if d != nil || err != nil {
			t.Errorf("malformed artifact = (%v, %v), want (nil, nil)", d, err)
		}
	})

	t.Run("unknown_action_falls_back_nil_nil", func(t *testing.T) {
		dir := t.TempDir()
		writeDecision(t, dir, `{"category":"code-audit-fail","level":"task","action":"frobnicate"}`)
		d, err := readFailureDecision(dir)
		if d != nil || err != nil {
			t.Errorf("unknown-action artifact = (%v, %v), want (nil, nil)", d, err)
		}
	})

	t.Run("unknown_level_falls_back_nil_nil", func(t *testing.T) {
		dir := t.TempDir()
		writeDecision(t, dir, `{"category":"code-audit-fail","level":"galaxy","action":"retry-with-fix"}`)
		d, err := readFailureDecision(dir)
		if d != nil || err != nil {
			t.Errorf("unknown-level artifact = (%v, %v), want (nil, nil)", d, err)
		}
	})
}
