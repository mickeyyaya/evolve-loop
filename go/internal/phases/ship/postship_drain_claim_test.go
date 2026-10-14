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

func writeDrainCycleState(t *testing.T, root string, cycleID int) {
	t.Helper()
	mustWriteState(t, filepath.Join(root, ".evolve", "cycle-state.json"), map[string]any{
		"cycle_id": float64(cycleID),
	})
}

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

func blockDrainPath(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("block %s: %v", path, err)
	}
}

func TestPromoteInbox_DrainFailure_NoFalseSuccessLog(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDrainCycleState(t, root, 42)
	writeDrainTriageDecision(t, root, 42, "committed-task")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "committed-task")
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

func TestPromoteInbox_PromoteError_StillDrainsResidualClaims(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDrainCycleState(t, root, 43)
	writeDrainTriageDecision(t, root, 43, "committed-task")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "committed-task")
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
