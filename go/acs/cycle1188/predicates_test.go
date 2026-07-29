//go:build acs

// Package cycle1188 materialises the cycle-1188 acceptance criteria for the one
// fleet-scoped task pinned to this lane: close-evaluate-batch-retry-parity-inbox.
//
// What this cycle is (and is NOT). The underlying design defect — the
// evaluate-batch retry loop having silently diverged from the main dispatch loop
// (missing optionalInfraSkip + postShipObserverSkip) — was ALREADY fixed in
// cycle-1166 and closed out at the repo-root inbox layer in cycle-1185. The
// shared retry core (retryPhaseRunner/retryOpts) is live in this worktree and its
// parity tests are green here BEFORE any change this cycle. So this cycle is pure
// state/paperwork reconciliation: this worktree carries its own isolated
// `.evolve/` snapshot whose inbox still holds the item at the OPEN root and whose
// state.json has no `evaluatedTasks` key at all.
//
// Because the code is already correct, the load-bearing predicates are 001–003
// (the applied state transition), and 004 is a REGRESSION PIN, not a RED target.
// Baseline measured at TDD time, in this worktree:
//
//   - `.evolve/inbox/2026-07-08T00-50-00Z-evaluate-batch-retry-parity.json` PRESENT
//   - no `.evolve/inbox/processed/*/` record for the item (only three
//     interactive-2026-06-* dirs exist)
//   - state.json keys: no `evaluatedTasks` at all (verified: key absent)
//   - `go test ./internal/core/... -run 'RetryOpts|RetryParity|DispatchRunnerWithRetry'`
//     → ok (pre-existing GREEN)
//
// Predicate strategy — every predicate reads REAL runtime state or executes the
// real system; none greps production source for a magic string (the cycle-85
// degenerate-predicate ban):
//
//   - 001 parses the emitted processed record as JSON and asserts it is the
//     genuine item that was MOVED (original id/created_at/weight/files survive),
//     not a hollow touch-file bearing the right name.
//   - 002 parses the LIVE state.json and asserts an evaluatedTasks completion
//     record exists with decision=="completed" — and that the rest of the file
//     survived (carryoverTodos intact), so a clobbering rewrite cannot pass.
//   - 003 is the NEGATIVE predicate: the item must be ABSENT from the open-inbox
//     root. A copy-instead-of-move fails this while passing 001.
//   - 004 EXECUTES the retry-parity suite as a subprocess and asserts exit 0 —
//     the paperwork closeout must not disturb the shipped fix it is closing.
//
// Root resolution mirrors the cycle-998 predicates: acsassert.RepoRoot resolves
// to this worktree (where Builder writes, per worktree isolation), so the
// artifacts read here are the ones Builder is required to produce.
package cycle1188

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// itemBasename is the inbox item's filename, identical at the open root and
// under processed/ (a move preserves the basename).
const itemBasename = "2026-07-08T00-50-00Z-evaluate-batch-retry-parity.json"

// itemID is the `id` field carried inside the inbox JSON. It is also the key
// fragment the evaluatedTasks completion record must carry.
const itemID = "evaluate-batch-retry-parity"

// openInboxRelPath is where the item still sits in this worktree today. The
// closeout must leave nothing here (predicate 003).
const openInboxRelPath = ".evolve/inbox/" + itemBasename

// findProcessedRecord returns the path of the processed record for the item,
// searching every `.evolve/inbox/processed/<dir>/` subdirectory. Builder may
// file it under cycle-1188/ or consumed/ or any sibling; the acceptance bar is
// "filed under processed", not one exact directory name. Returns "" if absent.
//
// The repo-root precedent (cycle-1185) prefixes the basename with a short hash
// (`c4e56157-<basename>`), so the match is a suffix match, not equality.
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

// TestC1188_001_processed_record_is_the_real_moved_item asserts the closeout
// filed a processed record AND that the record is the genuine inbox item, by
// parsing it and checking the identifying fields survived the move.
//
// AC: `test -f .evolve/inbox/processed/*/...evaluate-batch-retry-parity.json`
// exits 0.
//
// Anti-gaming: a bare existence check passes on `touch`. This parses the JSON
// and pins id/created_at/weight plus the two `files` entries from the original
// item, so an empty or fabricated stub fails. RED today: no processed record
// exists in this worktree.
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

	// The original item names the two files the shipped fix touched. Their
	// presence proves the whole payload moved, not just a header.
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

// TestC1188_002_state_json_records_completion asserts the LIVE state.json in
// this worktree gained an evaluatedTasks completion record for the item.
//
// AC: `grep -q '"evaluate-batch-retry-parity"' .evolve/state.json` under
// evaluatedTasks with `"decision": "completed"`.
//
// Anti-gaming / edge case: the key must live UNDER evaluatedTasks (a mention
// anywhere in the file is not enough — that is what a bare grep would accept),
// its decision must be exactly "completed", and the pre-existing carryoverTodos
// array must survive, so a state.json rewritten from scratch fails. RED today:
// state.json has no evaluatedTasks key at all.
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

	// Accept either the inbox item id or the todo-id as the key; require the
	// item fragment so an unrelated entry cannot satisfy this.
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

	// Regression guard: the closeout edits state.json in place. A wholesale
	// rewrite that drops the inherited carryover backlog is a data-loss bug
	// that would otherwise pass every check above.
	if len(state.CarryoverTodos) == 0 {
		t.Errorf("state.json carryoverTodos is empty — the closeout clobbered pre-existing state instead of adding a record")
	}
	if state.LastCycleNumber == 0 {
		t.Errorf("state.json lastCycleNumber = 0 — pre-existing state fields lost in the rewrite")
	}
}

// TestC1188_003_item_absent_from_open_inbox_root is the NEGATIVE predicate: the
// item must be GONE from the open-inbox root.
//
// AC: `test -f .evolve/inbox/<item>.json` exits non-zero.
//
// This is the anti-no-op signal for the whole cycle. Predicate 001 passes on a
// COPY; only this one forces an actual move, i.e. the item genuinely stops being
// open work in this worktree. RED today: the file is present.
func TestC1188_003_item_absent_from_open_inbox_root(t *testing.T) {
	root := acsassert.RepoRoot(t)
	openPath := filepath.Join(root, openInboxRelPath)

	if _, err := os.Stat(openPath); err == nil {
		t.Fatalf("%s still present at the open-inbox root — item was copied, not moved; it remains open work", openPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: unexpected error %v", openPath, err)
	}
}

// TestC1188_004_retry_parity_suite_stays_green EXECUTES the retry-parity suite
// and asserts exit 0.
//
// AC: `cd go && go test ./internal/core/... -run
// 'RetryOpts|RetryParity|DispatchRunnerWithRetry'` — 0 failures.
//
// This is a REGRESSION PIN, not a RED target: it is already green in this
// worktree at TDD time (verified: `ok .../internal/core`). It is here because the
// closeout's whole premise is "the code fix is done and verified" — if the
// paperwork lands while the fix it certifies is broken, the closeout is a lie.
// `go -C <root>/go` is used so the subprocess runs in the module dir regardless
// of the predicate runner's cwd.
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
	// Guard against a vacuous pass: `-run` matching nothing also exits 0.
	if !strings.Contains(stdout, "ok") {
		t.Errorf("retry-parity suite produced no ok package line — the -run filter matched no tests, so this proves nothing\nstdout:\n%s", stdout)
	}
}
