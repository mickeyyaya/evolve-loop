package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedState writes a state.json into dir/.evolve with the given failedApproaches.
func seedState(t *testing.T, dir string, failedApproaches []map[string]any) {
	t.Helper()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	st := map[string]any{
		"lastUpdated":      "2026-05-26T00:00:00Z",
		"lastCycleNumber":  107,
		"version":          1,
		"failedApproaches": failedApproaches,
	}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestRunGuard_ListAuditFails_TableOutput(t *testing.T) {
	dir := t.TempDir()
	future := "2030-01-01T00:00:00Z"
	seedState(t, dir, []map[string]any{
		{"cycle": 42, "classification": "code-audit-fail", "verdict": "FAIL", "recordedAt": "2026-05-14T03:00:00Z", "expiresAt": future, "summary": "ABC defect"},
		{"cycle": 99, "classification": "code-build-fail", "verdict": "FAIL", "recordedAt": "2026-05-15T00:00:00Z", "expiresAt": future, "summary": "build broke (should be skipped)"},
		{"cycle": 87, "classification": "code-audit-fail", "verdict": "FAIL", "recordedAt": "2026-05-13T05:00:00Z", "expiresAt": future, "summary": "XYZ defect"},
	})
	var stdout, stderr bytes.Buffer
	rc := runGuard([]string{"--evolve-dir", filepath.Join(dir, ".evolve"), "list-audit-fails"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d, stderr=%s", rc, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "42") || !strings.Contains(out, "87") {
		t.Errorf("expected cycles 42 and 87 in output, got %s", out)
	}
	if strings.Contains(out, "build broke") {
		t.Errorf("code-build-fail must be filtered out, got %s", out)
	}
	if !strings.Contains(out, "ABC defect") {
		t.Errorf("expected ABC defect summary in table, got %s", out)
	}
	if !strings.Contains(out, "2 pending code-audit-fail") {
		t.Errorf("expected count footer of 2, got %s", out)
	}
}

func TestRunGuard_ListAuditFails_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	future := "2030-01-01T00:00:00Z"
	seedState(t, dir, []map[string]any{
		{"cycle": 5, "classification": "code-audit-fail", "verdict": "FAIL", "recordedAt": "2026-05-10T00:00:00Z", "expiresAt": future, "summary": "alpha"},
	})
	var stdout, stderr bytes.Buffer
	rc := runGuard([]string{"--evolve-dir", filepath.Join(dir, ".evolve"), "list-audit-fails", "--json"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d, stderr=%s", rc, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal JSON: %v\noutput=%s", err, stdout.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0]["cycle"].(float64) != 5 {
		t.Errorf("expected cycle 5, got %v", got[0]["cycle"])
	}
}

func TestRunGuard_ListAuditFails_NoStateFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No state.json on purpose.
	var stdout, stderr bytes.Buffer
	rc := runGuard([]string{"--evolve-dir", filepath.Join(dir, ".evolve"), "list-audit-fails"}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("expected rc=1 (read error) when state.json missing, got rc=%d", rc)
	}
	if !strings.Contains(stderr.String(), "read") {
		t.Errorf("expected 'read' in stderr, got %s", stderr.String())
	}
}

func TestRunGuard_ListAuditFails_EmptyState(t *testing.T) {
	dir := t.TempDir()
	seedState(t, dir, nil)
	var stdout, stderr bytes.Buffer
	rc := runGuard([]string{"--evolve-dir", filepath.Join(dir, ".evolve"), "list-audit-fails"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("empty state must exit 0, got rc=%d stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "0 pending") {
		t.Errorf("expected '0 pending' footer, got %s", stdout.String())
	}
}

func TestRunGuard_ListAuditFails_FlagAfterSubcommand(t *testing.T) {
	// Operator may put --evolve-dir AFTER list-audit-fails. Both orders
	// must work.
	dir := t.TempDir()
	seedState(t, dir, nil)
	var stdout, stderr bytes.Buffer
	rc := runGuard([]string{"list-audit-fails", "--evolve-dir", filepath.Join(dir, ".evolve")}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d, stderr=%s", rc, stderr.String())
	}
}
