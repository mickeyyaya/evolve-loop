//go:build acs

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
	if inboxmover.ShouldQuarantine(99, 0, false) {
		t.Errorf("ShouldQuarantine(99, ceiling=0, false) = true; a zero ceiling disables quarantine")
	}
}

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
		if strings.Contains(e.Reason, "quarantine") || strings.Contains(e.To, "quarantine") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no ledger entry for %q records a quarantine diagnostic (reason/to naming 'quarantine')", taskID)
	}
}

func TestC1019_005_SystemFloorFailureNotQuarantined(t *testing.T) {
	const ceiling = 2
	if inboxmover.ShouldQuarantine(5, ceiling, true) {
		t.Errorf("ShouldQuarantine(5, ceiling=%d, systemLevel=true) = true; S3 halt must take precedence over task quarantine", ceiling)
	}
	if !inboxmover.ShouldQuarantine(5, ceiling, false) {
		t.Errorf("ShouldQuarantine(5, ceiling=%d, systemLevel=false) = false; a task-level failure past the ceiling must quarantine", ceiling)
	}
}
