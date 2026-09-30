//go:build acs

package cycle1188

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const itemBasename = "2026-07-08T00-50-00Z-evaluate-batch-retry-parity.json"

const itemID = "evaluate-batch-retry-parity"

const openInboxRelPath = ".evolve/inbox/" + itemBasename

func findProcessedRecord(t *testing.T, root string) string {
	t.Helper()
	processedDir := filepath.Join(root, ".evolve", "inbox", "processed")
	entries, err := os.ReadDir(processedDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(processedDir, e.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if strings.HasSuffix(f.Name(), itemBasename) {
				return filepath.Join(processedDir, e.Name(), f.Name())
			}
		}
	}
	return ""
}

func TestC1188_001_processed_record_is_the_real_moved_item(t *testing.T) {
	root := acsassert.RepoRoot(t)

	path := findProcessedRecord(t, root)
	if path == "" {
		t.Fatalf("no processed record for %q under %s/.evolve/inbox/processed/*/ — the closeout did not file the item", itemBasename, root)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("processed record %s unreadable: %v", path, err)
	}

	var item struct {
		ID        string   `json:"id"`
		CreatedAt string   `json:"created_at"`
		Weight    float64  `json:"weight"`
		Kind      string   `json:"kind"`
		Files     []string `json:"files"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("processed record %s is not valid JSON (a hollow stub, not the moved item): %v", path, err)
	}

	if item.ID != itemID {
		t.Errorf("processed record id = %q, want %q — this is not the item that was open", item.ID, itemID)
	}
	if item.CreatedAt != "2026-07-08T00:50:00Z" {
		t.Errorf("processed record created_at = %q, want %q — original provenance lost in the move", item.CreatedAt, "2026-07-08T00:50:00Z")
	}
	if item.Weight != 0.87 {
		t.Errorf("processed record weight = %v, want 0.87 — original payload not preserved", item.Weight)
	}
	if item.Kind != "bug" {
		t.Errorf("processed record kind = %q, want %q", item.Kind, "bug")
	}

	wantFiles := []string{
		"go/internal/core/cyclerun_dispatch.go",
		"go/internal/core/evaluate_batch.go",
	}
	for _, want := range wantFiles {
		found := false
		for _, got := range item.Files {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("processed record files missing %q (got %v) — payload truncated in the move", want, item.Files)
		}
	}
}

func TestC1188_002_state_json_records_completion(t *testing.T) {
	root := acsassert.RepoRoot(t)
	statePath := filepath.Join(root, ".evolve", "state.json")

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("state.json unreadable at %s: %v", statePath, err)
	}

	var state struct {
		EvaluatedTasks  map[string]json.RawMessage `json:"evaluatedTasks"`
		CarryoverTodos  []json.RawMessage          `json:"carryoverTodos"`
		LastCycleNumber int                        `json:"lastCycleNumber"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("state.json is not valid JSON after the closeout: %v", err)
	}

	if len(state.EvaluatedTasks) == 0 {
		t.Fatalf("state.json has no evaluatedTasks entries — no completion record was written")
	}

	var matchedKey string
	var matchedRaw json.RawMessage
	for k, v := range state.EvaluatedTasks {
		if strings.Contains(k, itemID) {
			matchedKey, matchedRaw = k, v
			break
		}
	}
	if matchedKey == "" {
		keys := make([]string, 0, len(state.EvaluatedTasks))
		for k := range state.EvaluatedTasks {
			keys = append(keys, k)
		}
		t.Fatalf("no evaluatedTasks key containing %q (got keys %v)", itemID, keys)
	}

	var record struct {
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal(matchedRaw, &record); err != nil {
		t.Fatalf("evaluatedTasks[%q] is not an object: %v", matchedKey, err)
	}
	if record.Decision != "completed" {
		t.Errorf("evaluatedTasks[%q].decision = %q, want %q", matchedKey, record.Decision, "completed")
	}

	if len(state.CarryoverTodos) == 0 {
		t.Errorf("state.json carryoverTodos is empty — the closeout clobbered pre-existing state instead of adding a record")
	}
	if state.LastCycleNumber == 0 {
		t.Errorf("state.json lastCycleNumber = 0 — pre-existing state fields lost in the rewrite")
	}
}

func TestC1188_003_item_absent_from_open_inbox_root(t *testing.T) {
	root := acsassert.RepoRoot(t)
	openPath := filepath.Join(root, openInboxRelPath)

	if _, err := os.Stat(openPath); err == nil {
		t.Fatalf("%s still present at the open-inbox root — item was copied, not moved; it remains open work", openPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: unexpected error %v", openPath, err)
	}
}

func TestC1188_004_retry_parity_suite_stays_green(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir, "test", "./internal/core/...",
		"-run", "RetryOpts|RetryParity|DispatchRunnerWithRetry", "-count=1",
	)
	if err != nil {
		t.Fatalf("could not run the retry-parity suite: %v\nstderr:\n%s", err, stderr)
	}
	if code != 0 {
		t.Fatalf("retry-parity suite exit=%d, want 0 — the closeout certifies a fix that is not green\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "ok") {
		t.Errorf("retry-parity suite produced no ok package line — the -run filter matched no tests, so this proves nothing\nstdout:\n%s", stdout)
	}
}
