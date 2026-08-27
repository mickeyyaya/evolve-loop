package cyclesimulator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runid_stamp_test.go — go-review CRITICAL #2. The simulator writes into the
// SAME ledger as the real loop, so its agent_subprocess entries are subject to
// the same run-scoped binding contract. Setting entry["run_id"] is NOT enough:
// jsonCompact emits only the keys in its own hardcoded allowlist, so a key that
// is not on that list is computed and then silently discarded.
//
// This is the exact unit-green/live-dark shape the rest of this PR exists to
// close, and the source-scanning class guard is structurally blind to it — it
// can see that RunIDFromWorkspace is CALLED, never that the value reaches the
// emitted bytes. Only a behavioural test over the real writer can.
func TestAppendSimLedger_StampsRunIDIntoEmittedLine(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "run.json"),
		[]byte(`{"cycle_id":42,"run_id":"01SIMRUNID000000000000000"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(workspace, "audit-report.md")
	if err := os.WriteFile(artifact, []byte("# Audit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledgerPath := filepath.Join(dir, ".evolve", "ledger.jsonl")

	if err := appendSimLedger(ledgerPath, 42, "auditor", artifact, "tok", dir,
		func() time.Time { return time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC) }); err != nil {
		t.Fatalf("appendSimLedger: %v", err)
	}

	b, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(b))
	var got struct {
		Kind  string `json:"kind"`
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("simulator ledger line is not valid JSON: %v\nline: %s", err, line)
	}
	if got.Kind != "agent_subprocess" {
		t.Fatalf("kind = %q, want agent_subprocess", got.Kind)
	}
	if got.RunID != "01SIMRUNID000000000000000" {
		t.Errorf("emitted run_id = %q, want the run.json value — an entry with no run id can never be bound by ship\nline: %s",
			got.RunID, line)
	}
}
