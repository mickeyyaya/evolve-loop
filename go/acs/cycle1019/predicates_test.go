//go:build acs

// Package cycle1019 materialises the acceptance criteria for the single
// fleet-scoped task adr0072-s5-task-quarantine (ADR-0072 S5: task-level failure
// memory + quarantine). S5 is the missing task-half of the ADR-0072 failure
// policy: the system-level halt (S3) and the orchestrator-judgment layer (S4)
// already shipped; S5 stops the loop from re-picking a poison inbox task
// forever by routing a task that has failed `task_retry_ceiling` times to
// `.evolve/inbox/quarantine/` (a sibling dir the triage scanner never walks)
// instead of releasing it back to the inbox root every cycle.
//
// Predicate strategy — every predicate EXERCISES the system under test (calls
// the real inboxmover/inboxbatch functions and asserts on the emitted files,
// ledger, and return values), never a source-grep of production code (the
// cycle-85 degenerate-predicate ban). Each fails RED on the current tree:
//
//   - 001 drives inboxmover.Promote(..., "quarantine", ...) and asserts the
//     item physically lands under .evolve/inbox/quarantine/ (AC1 routing). RED
//     now: "quarantine" is not yet a validStates target, so Promote errors and
//     never moves the file.
//   - 002 exercises the pure decision inboxmover.ShouldQuarantine at/above/below
//     the ceiling (AC1 count trigger). RED now: the function does not exist
//     (compile failure IS a valid RED per go/acs/README.md).
//   - 003 places a healthy item AND a poison item at the inbox ROOT (the S5
//     "released back every cycle" shape), quarantines the poison, and asserts
//     the real triage scanner inboxbatch.LoadDir(root) returns the healthy item
//     but NOT the poison (AC1 invisibility + AC2 siblings-keep-flowing). RED
//     now: the poison stays at root and LoadDir still returns it.
//   - 004 reads the emitted .evolve/ledger.jsonl after a quarantine and asserts
//     a durable record identifying WHY the item was quarantined (AC3 failure
//     diagnostic). RED now: no quarantine promote, no ledger line.
//   - 005 exercises ShouldQuarantine with the system-level (S3 floor) flag set
//     and asserts it NEVER quarantines even past the ceiling — the S3 halt
//     takes precedence over task quarantine (AC4). RED now: function absent.
//
// AC5 (go vet / -race on internal/inboxmover / no regression) is a
// manual+checklist disposition verified by the audit lane and CI, not a
// per-behavior predicate (see test-report.md).
package cycle1019

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// writeItem creates a minimal inbox item JSON (id-bearing) at path.
func writeItem(t *testing.T, path, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	body := []byte(`{"id":"` + id + `","title":"` + id + `","weight":0.9}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestC1019_001_QuarantineRoutingLandsInQuarantineDir — AC1 (routing).
// A quarantine promote must relocate the item into .evolve/inbox/quarantine/
// and out of its source dir, proving the new terminal routing state exists.
func TestC1019_001_QuarantineRoutingLandsInQuarantineDir(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	const taskID = "poison-task"
	src := filepath.Join(inbox, "processing", "cycle-7", taskID+".json")
	writeItem(t, src, taskID)

	opts := inboxmover.Options{ProjectRoot: root}
	res, err := inboxmover.Promote(opts, taskID, "quarantine", inboxmover.PromoteOpts{Cycle: "7"})
	if err != nil {
		t.Fatalf("Promote to quarantine returned error (quarantine must be a valid terminal state): %v", err)
	}

	qDir := filepath.Join(inbox, "quarantine")
	if !strings.HasPrefix(res.DestPath, qDir) {
		t.Errorf("quarantine dest = %q; want a path under %q", res.DestPath, qDir)
	}
	if _, statErr := os.Stat(res.DestPath); statErr != nil {
		t.Errorf("quarantined file absent at %q: %v", res.DestPath, statErr)
	}
	if _, statErr := os.Stat(src); statErr == nil {
		t.Errorf("source still present at %q — item was not moved out of processing/", src)
	}
}

// TestC1019_002_QuarantineDecisionAtCeiling — AC1 (count trigger).
// The pure S5 decision must fire once the task-level failure count reaches the
// configured ceiling and stay quiet below it.
func TestC1019_002_QuarantineDecisionAtCeiling(t *testing.T) {
	const ceiling = 2
	cases := []struct {
		count int
		want  bool
		desc  string
	}{
		{count: 1, want: false, desc: "below ceiling: keep retrying"},
		{count: 2, want: true, desc: "at ceiling: quarantine"},
		{count: 3, want: true, desc: "above ceiling: quarantine"},
	}
	for _, tc := range cases {
		if got := inboxmover.ShouldQuarantine(tc.count, ceiling, false); got != tc.want {
			t.Errorf("ShouldQuarantine(count=%d, ceiling=%d, systemLevel=false) = %v; want %v (%s)",
				tc.count, ceiling, got, tc.want, tc.desc)
		}
	}
	// A disabled ceiling (0) must never quarantine.
	if inboxmover.ShouldQuarantine(99, 0, false) {
		t.Errorf("ShouldQuarantine(99, ceiling=0, false) = true; a zero ceiling disables quarantine")
	}
}

// TestC1019_003_SiblingsFlowQuarantineInvisibleToTriage — AC1 invisibility + AC2.
// With a healthy item and a poison item both at the inbox ROOT (the pre-S5
// "released every cycle" shape), quarantining the poison must remove it from the
// candidate set the triage scanner (inboxbatch.LoadDir) returns while the
// healthy sibling keeps flowing.
func TestC1019_003_SiblingsFlowQuarantineInvisibleToTriage(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	const healthyID = "healthy-item"
	const poisonID = "poison-item"
	writeItem(t, filepath.Join(inbox, healthyID+".json"), healthyID)
	writeItem(t, filepath.Join(inbox, poisonID+".json"), poisonID)

	opts := inboxmover.Options{ProjectRoot: root}
	if _, err := inboxmover.Promote(opts, poisonID, "quarantine", inboxmover.PromoteOpts{Cycle: "9"}); err != nil {
		t.Fatalf("quarantine promote of poison item failed: %v", err)
	}

	items, _, err := inboxbatch.LoadDir(inbox)
	if err != nil {
		t.Fatalf("LoadDir(inbox root): %v", err)
	}
	seen := map[string]bool{}
	for _, it := range items {
		seen[it.ID] = true
	}
	if !seen[healthyID] {
		t.Errorf("healthy sibling %q missing from triage candidates — the loop must keep flowing", healthyID)
	}
	if seen[poisonID] {
		t.Errorf("quarantined item %q still visible to triage — it must be invisible to the next cycle", poisonID)
	}
}

// TestC1019_004_QuarantineRecordsDiagnostic — AC3 (failure diagnostic).
// A quarantine must leave a durable record in the ledger that identifies WHY
// the item was quarantined, so an operator can see the reason after the fact.
func TestC1019_004_QuarantineRecordsDiagnostic(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	const taskID = "poison-diag"
	writeItem(t, filepath.Join(inbox, "processing", "cycle-3", taskID+".json"), taskID)

	opts := inboxmover.Options{ProjectRoot: root}
	if _, err := inboxmover.Promote(opts, taskID, "quarantine", inboxmover.PromoteOpts{Cycle: "3"}); err != nil {
		t.Fatalf("quarantine promote failed: %v", err)
	}

	ledgerPath := filepath.Join(root, ".evolve", "ledger.jsonl")
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read ledger %s: %v", ledgerPath, err)
	}
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e struct {
			TaskID string `json:"task_id"`
			To     string `json:"to"`
			Reason string `json:"reason"`
		}
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if e.TaskID != taskID {
			continue
		}
		// The diagnostic must name the quarantine, not a bare cycle-release.
		if strings.Contains(e.Reason, "quarantine") || strings.Contains(e.To, "quarantine") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no ledger entry for %q records a quarantine diagnostic (reason/to naming 'quarantine')", taskID)
	}
}

// TestC1019_005_SystemFloorFailureNotQuarantined — AC4 (S3 precedence).
// When a failure is system-level (an S3 floor halt), quarantine must NOT fire
// even past the ceiling: the halt takes precedence over task quarantine.
func TestC1019_005_SystemFloorFailureNotQuarantined(t *testing.T) {
	const ceiling = 2
	// System-level failure well past the ceiling: still no quarantine.
	if inboxmover.ShouldQuarantine(5, ceiling, true) {
		t.Errorf("ShouldQuarantine(5, ceiling=%d, systemLevel=true) = true; S3 halt must take precedence over task quarantine", ceiling)
	}
	// Control: the same count at task level DOES quarantine — proves the flag,
	// not the count, is what suppresses it.
	if !inboxmover.ShouldQuarantine(5, ceiling, false) {
		t.Errorf("ShouldQuarantine(5, ceiling=%d, systemLevel=false) = false; a task-level failure past the ceiling must quarantine", ceiling)
	}
}
