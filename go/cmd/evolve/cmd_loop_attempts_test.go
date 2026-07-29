package main

// cmd_loop_attempts_test.go — ADR-0080 P2 wiring proof: a graded FAIL bumps
// exactly the committed ids where they live, and the ceiling quarantines.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func attemptsFixture(t *testing.T, committed []string) (root, workspace string) {
	t.Helper()
	root = t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range committed {
		b, _ := json.Marshal(map[string]any{"id": id, "weight": 0.9, "failure_count": 2})
		if err := os.WriteFile(filepath.Join(inbox, id+".json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// An UNCOMMITTED menu-mate that must never bump.
	b, _ := json.Marshal(map[string]any{"id": "bystander", "weight": 0.5})
	if err := os.WriteFile(filepath.Join(inbox, "bystander.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	workspace = filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	topN := make([]map[string]string, 0, len(committed))
	for _, id := range committed {
		topN = append(topN, map[string]string{"id": id})
	}
	td, _ := json.Marshal(map[string]any{"top_n": topN})
	if err := os.WriteFile(filepath.Join(workspace, "triage-decision.json"), td, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, workspace
}

func TestRecordCommittedFailures_BumpsCommittedQuarantinesAtCeilingSparesBystanders(t *testing.T) {
	root, ws := attemptsFixture(t, []string{"grinder"})
	var warn bytes.Buffer
	recordCommittedFailures(root, ws, 9, 3, &warn)
	inbox := filepath.Join(root, ".evolve", "inbox")
	if _, err := os.Stat(filepath.Join(inbox, "quarantine", "grinder.json")); err != nil {
		t.Fatalf("grinder at count 2 + this FAIL = ceiling 3 must quarantine: %v\n%s", err, warn.String())
	}
	if !strings.Contains(warn.String(), "QUARANTINED after 3") {
		t.Errorf("quarantine must be loud with the count: %s", warn.String())
	}
	raw, err := os.ReadFile(filepath.Join(inbox, "bystander.json"))
	if err != nil {
		t.Fatalf("bystander must stay in the root: %v", err)
	}
	if strings.Contains(string(raw), "failure_count") {
		t.Fatalf("an UNCOMMITTED menu-mate must never bump: %s", raw)
	}
}

func TestRecordCommittedFailures_NoDecisionAccountsNothing(t *testing.T) {
	root := t.TempDir()
	var warn bytes.Buffer
	recordCommittedFailures(root, filepath.Join(root, "nope"), 9, 3, &warn)
	if warn.Len() != 0 {
		t.Fatalf("no committed set ⇒ silence, got %s", warn.String())
	}
}
