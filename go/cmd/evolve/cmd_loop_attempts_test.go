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

// The full graded-FAIL chain (classify → task-level gate → policy ceiling →
// bump) as ONE shared helper — `evolve cycle run` (every fleet lane) and the
// sequential loop body must run the SAME chain. Batch-18 wave 1 proved the
// gap live: two graded FAILs (1171/1172), failure_count still absent, because
// the chain lived only in the sequential body fleet lanes never execute.

// TestCycleRunEpilogue_CallsFailureAccounting pins the WIRING, not the
// behavior (review M1): the helper tests below all invoke the chain directly,
// so deleting the cmd_cycle.go call site would leave them green while
// reopening the exact batch-18 no-op. runCycleRun cannot be driven to a FAIL
// verdict hermetically (--simulate always PASSes), so this is a source
// assertion — same class as the manifest/template pins used elsewhere: the
// epilogue must call recordTaskFailureForCycle AFTER the halt branch.
func TestCycleRunEpilogue_CallsFailureAccounting(t *testing.T) {
	src, err := os.ReadFile("cmd_cycle.go")
	if err != nil {
		t.Fatal(err)
	}
	haltAt := strings.Index(string(src), "haltOnSystemFailure(evolveDir")
	callAt := strings.Index(string(src), "recordTaskFailureForCycle(projectRoot, evolveDir, result.Cycle")
	if haltAt < 0 || callAt < 0 {
		t.Fatalf("epilogue wiring missing: halt branch at %d, accounting call at %d", haltAt, callAt)
	}
	if callAt < haltAt {
		t.Fatalf("accounting must run AFTER the ADR-0072 halt early-return (halt at %d, call at %d) — a halting system failure is never the task's fault", haltAt, callAt)
	}
}

func TestRecordTaskFailureForCycle_TaskLevelFailBumpsCommitted(t *testing.T) {
	root, ws := attemptsFixture(t, []string{"grinder"})
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("Verdict: FAIL — audit red\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	recordTaskFailureForCycle(root, filepath.Join(root, ".evolve"), 9, &warn)
	// Fixture seeds failure_count=2; the default policy ceiling is 2, so the
	// bump to 3 crosses it ⇒ quarantine. That also pins the policy load.
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "quarantine", "grinder.json")); err != nil {
		t.Fatalf("task-level FAIL must run the accounting chain end-to-end: %v\n%s", err, warn.String())
	}
}

func TestRecordTaskFailureForCycle_SystemLevelFailSkipsAccounting(t *testing.T) {
	root, ws := attemptsFixture(t, []string{"grinder"})
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("INFRASTRUCTURE FAILURE: connection refused\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	recordTaskFailureForCycle(root, filepath.Join(root, ".evolve"), 9, &warn)
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "inbox", "grinder.json"))
	if err != nil {
		t.Fatalf("system-level failure must not move the item: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if fc, _ := got["failure_count"].(float64); fc != 2 {
		t.Fatalf("system-level failure is not the task's fault (AC4) — failure_count must stay 2, got %v", got["failure_count"])
	}
}

func TestRecordTaskFailureForCycle_PolicyCeilingOverridesDefault(t *testing.T) {
	root, ws := attemptsFixture(t, []string{"grinder"})
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("Verdict: FAIL\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pol := []byte(`{"failure_policy":{"thresholds":{"task_retry_ceiling":9}}}`)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), pol, 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	recordTaskFailureForCycle(root, filepath.Join(root, ".evolve"), 9, &warn)
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "inbox", "grinder.json"))
	if err != nil {
		t.Fatalf("ceiling 9 with count 3 must bump in place, not quarantine: %v\n%s", err, warn.String())
	}
	if !strings.Contains(string(raw), `"failure_count": 3`) && !strings.Contains(string(raw), `"failure_count":3`) {
		t.Fatalf("bump to 3 expected under ceiling 9: %s", raw)
	}
}
