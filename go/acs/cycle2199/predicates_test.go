//go:build acs

package cycle2199

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const itemID = "one-count-item"

type project struct {
	root  string
	inbox string
}

func newProject(t *testing.T) project {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"id": itemID, "title": "fixture", "kind": "bug"})
	if err := os.WriteFile(filepath.Join(inbox, "2026-11-06T00-00-00Z-"+itemID+".json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	return project{root: root, inbox: inbox}
}

func (p project) workspace(t *testing.T, cycle int) string {
	t.Helper()
	ws := filepath.Join(p.root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	decision, _ := json.Marshal(map[string]any{"top_n": []map[string]string{{"id": itemID}}})
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), decision, 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func (p project) fail(t *testing.T, cycle, ceiling int, systemLevel bool) {
	t.Helper()
	_, err := cycleoutcome.ApplyFailure(cycleoutcome.FailureInputs{
		ProjectRoot: p.root,
		Workspace:   p.workspace(t, cycle),
		Cycle:       cycle,
		Ceiling:     ceiling,
		SystemLevel: systemLevel,
		Reason:      "cycle-failure-release",
		Stderr:      io.Discard,
	})
	if err != nil {
		t.Fatalf("ApplyFailure cycle %d: %v", cycle, err)
	}
}

func (p project) failureCount(t *testing.T) int {
	t.Helper()
	path, err := inboxmover.FindFileByTaskID(p.inbox, itemID)
	if err != nil {
		path, err = inboxmover.FindFileByTaskID(filepath.Join(p.inbox, "quarantine"), itemID)
	}
	if err != nil {
		t.Fatalf("item %s is in no inbox directory: %v", itemID, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec struct {
		FailureCount int `json:"failure_count"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	return rec.FailureCount
}

func TestC2199_001_OneFailureEventOfACycleChargesTheItemOnce(t *testing.T) {
	p := newProject(t)
	p.fail(t, 7, 9, false)
	p.fail(t, 7, 9, false)
	if got := p.failureCount(t); got != 1 {
		t.Errorf("RED: the same failed cycle 7 applied twice left failure_count = %d, want 1 (one event adds at most one block)", got)
	}
}

func TestC2199_002_ARepeatedCloseoutDoesNotPushTheItemToTheCeiling(t *testing.T) {
	p := newProject(t)
	p.fail(t, 7, 2, false)
	p.fail(t, 7, 2, false)
	if got := p.failureCount(t); got != 1 {
		t.Errorf("RED: ceiling 2 and one failed cycle applied twice: failure_count = %d, want 1", got)
	}
	if _, err := inboxmover.FindFileByTaskID(filepath.Join(p.inbox, "quarantine"), itemID); err == nil {
		t.Errorf("RED: one failed cycle parked the item in quarantine at ceiling 2")
	}
}

func TestC2199_003_TwoDistinctFailedCyclesStillChargeTwoBlocks(t *testing.T) {
	p := newProject(t)
	p.fail(t, 7, 9, false)
	p.fail(t, 8, 9, false)
	if got := p.failureCount(t); got != 2 {
		t.Errorf("RED: failed cycles 7 and 8 left failure_count = %d, want 2 (each event is one block)", got)
	}
}

func TestC2199_004_ARepeatInALaterCycleDoesNotHideThatCyclesBlock(t *testing.T) {
	p := newProject(t)
	p.fail(t, 7, 9, false)
	p.fail(t, 7, 9, false)
	p.fail(t, 8, 9, false)
	p.fail(t, 8, 9, false)
	if got := p.failureCount(t); got != 2 {
		t.Errorf("RED: cycles 7 and 8 each applied twice left failure_count = %d, want 2", got)
	}
}

func TestC2199_005_AHostFaultChargesNoBlockToTheTask(t *testing.T) {
	p := newProject(t)
	p.fail(t, 7, 9, true)
	p.fail(t, 7, 9, true)
	if got := p.failureCount(t); got != 0 {
		t.Errorf("RED: a host fault charged the task: failure_count = %d, want 0", got)
	}
}

// acs-predicate: config-check
func TestC2199_006_AdrRule10StatesTheSameCountAsDecision16(t *testing.T) {
	adr := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "adr", "0160-the-host-lineage-record-is-the-one-trust-root-for-what-a-cycle-may-do-to-the-queue.md")
	if !acsassert.LineContainsAll(adr, "10. A failure before build charges the item", "one block", "decision 16") {
		t.Errorf("RED: %s rule 10 does not state that a charge is one block, per ADR-0126 decision 16", adr)
	}
}

// acs-predicate: config-check
func TestC2199_007_Decision16KeepsItsOneBlockPerEventCap(t *testing.T) {
	adr := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "adr", "0126-every-iterative-loop-converges-or-escalates.md")
	if !acsassert.LineContainsAll(adr, "One event adds at most one block to a subject") {
		t.Errorf("RED: %s lost the cap of decision 16 (one event adds at most one block to a subject)", adr)
	}
}
