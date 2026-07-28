package ship

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// postship_drain_claim_test.go pins the cycle-1156 D1 *aggravator*: promoteInbox
// used to append "[ship] OK: inbox lifecycle drain complete" unconditionally,
// after having already logged a WARN for the very failure that stopped the
// drain. An operator (and every log-grepping gate) read success from a cycle
// whose lifecycle transition demonstrably did not complete. The code defect and
// the false-success claim are separately regressible, so they are pinned
// separately — these are the two tests the cycle-1158 eval's third score_cap
// names as its evidence command.

// writeDrainCycleState drops a cycle-state.json carrying cycle_id.
func writeDrainCycleState(t *testing.T, root string, cycleID int) {
	t.Helper()
	mustWriteState(t, filepath.Join(root, ".evolve", "cycle-state.json"), map[string]any{
		"cycle_id": float64(cycleID),
	})
}

// writeDrainTriageDecision drops the triage-decision.json promoteInbox reads for the
// committed set.
func writeDrainTriageDecision(t *testing.T, root string, cycleID int, ids ...string) {
	t.Helper()
	type entry struct {
		ID string `json:"id"`
	}
	doc := struct {
		TopN []entry `json:"top_n"`
	}{}
	for _, id := range ids {
		doc.TopN = append(doc.TopN, entry{ID: id})
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal triage decision: %v", err)
	}
	dir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycleID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "triage-decision.json"), body, 0o644); err != nil {
		t.Fatalf("write triage-decision.json: %v", err)
	}
}

// writeDrainInboxItem drops an inbox item JSON with the given id into dir.
func writeDrainInboxItem(t *testing.T, dir, id string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	body := []byte(`{"id":"` + id + `","title":"fixture ` + id + `","kind":"bug"}`)
	if err := os.WriteFile(filepath.Join(dir, id+".json"), body, 0o644); err != nil {
		t.Fatalf("write item %s: %v", id, err)
	}
}

// blockDrainPath writes a regular FILE where a directory is needed, so the
// destination MkdirAll inside Promote fails — the infrastructure non-delivery
// ADR-0079 decision 4 made loud.
func blockDrainPath(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("block %s: %v", path, err)
	}
}

// TestPromoteInbox_DrainFailure_NoFalseSuccessLog: when the inbox outcome apply
// fails, promoteInbox must NOT also claim the drain completed. The success line
// lives in the success branch; the failure branch says INCOMPLETE, so the two
// claims cannot both appear for one cycle.
func TestPromoteInbox_DrainFailure_NoFalseSuccessLog(t *testing.T) {
	root := t.TempDir()
	writeDrainCycleState(t, root, 42)
	writeDrainTriageDecision(t, root, 42, "committed-task")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "committed-task")
	// processed/ is a FILE ⇒ processed/cycle-42/ cannot be created ⇒ the
	// committed promote is a non-delivery and ApplyCycleOutcome returns an error.
	blockDrainPath(t, filepath.Join(inbox, "processed"))

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox returned err = %v; a lifecycle bookkeeping failure is logged, not raised", err)
	}

	logs := strings.Join(res.Logs, "\n")
	if strings.Contains(logs, "inbox lifecycle drain complete") {
		t.Errorf("promoteInbox claimed 'inbox lifecycle drain complete' on a run whose outcome apply failed — "+
			"that success claim is what let a broken drain read as a healthy cycle (cycle-1156 D1 aggravator).\nlogs:\n%s", logs)
	}
	if !strings.Contains(logs, "INCOMPLETE") {
		t.Errorf("no INCOMPLETE line for the failed drain: the failure must be visible, not merely un-claimed.\nlogs:\n%s", logs)
	}
}

// TestPromoteInbox_PromoteError_StillDrainsResidualClaims: the D1 defect proper.
// A failed committed-id promote must not skip the residual drain — the early
// return stranded every item parked in processing/cycle-N/ (the cross-cycle
// orphan shape of cycles 124/265/294/295/308). The residual item must be back
// at the inbox root even though the promote failed.
func TestPromoteInbox_PromoteError_StillDrainsResidualClaims(t *testing.T) {
	root := t.TempDir()
	writeDrainCycleState(t, root, 43)
	writeDrainTriageDecision(t, root, 43, "committed-task")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "committed-task")
	// A residual claim from this cycle, sitting where only the drain can free it.
	writeDrainInboxItem(t, filepath.Join(inbox, "processing", "cycle-43"), "residual-task")
	blockDrainPath(t, filepath.Join(inbox, "processed"))

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox returned err = %v; want nil (the failure is logged)", err)
	}

	if _, err := os.Stat(filepath.Join(inbox, "residual-task.json")); err != nil {
		t.Errorf("residual-task was not drained back to the inbox root after a failed promote: %v — "+
			"one unwritable processed/cycle-N/ must not strand every claimed item for the next triage", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "processing", "cycle-43", "residual-task.json")); err == nil {
		t.Error("residual-task left behind in processing/cycle-43/: the drain must run even when a promote failed")
	}
}
