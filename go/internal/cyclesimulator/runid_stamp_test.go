package cyclesimulator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
