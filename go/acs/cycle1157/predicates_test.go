//go:build acs

package cycle1157

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func newInbox(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	return root, inbox
}

func writeItem(t *testing.T, dir, id string, failureCount int) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	doc := map[string]any{
		"id":       id,
		"title":    "fixture item " + id,
		"kind":     "bug",
		"weight":   0.5,
		"priority": "medium",
	}
	if failureCount > 0 {
		doc["failure_count"] = failureCount
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal item %s: %v", id, err)
	}
	path := filepath.Join(dir, "2026-07-28T00-00-00Z-"+id+".json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write item %s: %v", id, err)
	}
	return path
}

func blockDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("block %s: %v", path, err)
	}
}

func testOpts(root string, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{
		ProjectRoot: root,
		Stderr:      stderr,
		IsLandedFn:  func(string) (bool, error) { return true, nil },
	}
}

func findItem(t *testing.T, dir, id string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		body, rErr := os.ReadFile(path)
		if rErr != nil {
			continue
		}
		var doc struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(body, &doc) == nil && doc.ID == id {
			return path
		}
	}
	return ""
}

func containsAll(s string, needles ...string) bool {
	low := strings.ToLower(s)
	for _, n := range needles {
		if !strings.Contains(low, strings.ToLower(n)) {
			return false
		}
	}
	return true
}

func hasLineWith(s string, needles ...string) bool {
	for _, line := range strings.Split(s, "\n") {
		if containsAll(line, needles...) {
			return true
		}
	}
	return false
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "build", "-o", bin, "../../cmd/evolve")
	if code == -1 && err != nil && strings.Contains(err.Error(), "not found") {
		t.Skipf("go toolchain not available: %v", err)
	}
	if code != 0 {
		t.Fatalf("go build ./cmd/evolve failed (exit %d): %s", code, stderr)
	}
	return bin
}

func TestC1157_001_promote_mkdir_failure_returns_errmvfailed(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "processing", "cycle-1157"), "stranded-task", 0)
	blockDir(t, filepath.Join(inbox, "processed"))

	var errBuf bytes.Buffer
	res, err := inboxmover.Promote(testOpts(root, &errBuf), "stranded-task", "processed",
		inboxmover.PromoteOpts{Cycle: "1157"})

	if !errors.Is(err, inboxmover.ErrMvFailed) {
		t.Fatalf("Promote err = %v; want ErrMvFailed: a destination mkdir failure is a non-delivery, not a no-op success", err)
	}
	if res.NoOp {
		t.Error("Promote res.NoOp = true on mkdir failure: NoOp is the ship.sh 'already moved' contract and must not cover a stranded task")
	}
	if findItem(t, filepath.Join(inbox, "processing", "cycle-1157"), "stranded-task") == "" {
		t.Error("item left processing/cycle-1157/ despite the failed promote: failing loud must not also lose the file")
	}
}

func TestC1157_002_apply_cycle_outcome_propagates_promote_failure(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "processing", "cycle-1157"), "committed-task", 0)
	blockDir(t, filepath.Join(inbox, "processed"))

	var errBuf bytes.Buffer
	or, err := inboxmover.ApplyCycleOutcome(testOpts(root, &errBuf), inboxmover.CycleOutcome{
		Cycle:        1157,
		Passed:       true,
		CommittedIDs: []string{"committed-task"},
		CommitSHA:    "cafef00dab",
	})

	if !errors.Is(err, inboxmover.ErrMvFailed) {
		t.Fatalf("ApplyCycleOutcome err = %v; want an ErrMvFailed-wrapping error: the ship-side caller must surface a non-delivery, not swallow it", err)
	}
	for _, id := range or.Promoted {
		if id == "committed-task" {
			t.Error("committed-task reported in OutcomeResult.Promoted although its promote failed: a stranded task must never be reported as delivered")
		}
	}
}

func TestC1157_003_quarantine_promote_failure_is_surfaced(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "processing", "cycle-1157"), "poison-task", 1)
	blockDir(t, filepath.Join(inbox, "quarantine"))

	var errBuf bytes.Buffer
	res, err := inboxmover.ApplyCycleOutcome(testOpts(root, &errBuf), inboxmover.CycleOutcome{
		Cycle:   1157,
		Passed:  false,
		Reason:  "cycle-failure-release",
		Ceiling: 2,
	})
	if err != nil {
		t.Fatalf("ApplyCycleOutcome(FAIL) err = %v; the drain stays fail-open — the failed quarantine is reported, not raised", err)
	}

	stderr := errBuf.String()
	if !hasLineWith(stderr, "poison-task", "quarantine") {
		t.Errorf("failed quarantine promote is invisible: no single stderr line ties the task id to the failed quarantine attempt.\n"+
			"A silently un-quarantined poison item returns to the inbox root and is re-picked next cycle.\nstderr:\n%s", stderr)
	}
	if !hasLineWith(stderr, "poison-task", "quarantine", "ERROR") && !hasLineWith(stderr, "poison-task", "quarantine", "WARN") {
		t.Errorf("quarantine failure reported without an ERROR/WARN severity marker on the same line — operators grep severity; got:\n%s", stderr)
	}
	if findItem(t, inbox, "poison-task") == "" {
		t.Errorf("poison-task is no longer at the inbox root after the failed quarantine (released=%v, quarantined=%v): the drain must stay fail-open, a loud failure must not also strand the item in processing/", res.Released, res.Quarantined)
	}
	if findItem(t, filepath.Join(inbox, "processing", "cycle-1157"), "poison-task") != "" {
		t.Error("poison-task left behind in processing/cycle-1157/: the failed quarantine must fall through to the ordinary release")
	}
}

func TestC1157_004_successful_quarantine_emits_no_failure_diagnostic(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "processing", "cycle-1157"), "poison-task", 1)

	var errBuf bytes.Buffer
	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root, &errBuf), inboxmover.CycleOutcome{
		Cycle:   1157,
		Passed:  false,
		Reason:  "cycle-failure-release",
		Ceiling: 2,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(FAIL): %v", err)
	}

	if findItem(t, filepath.Join(inbox, "quarantine"), "poison-task") == "" {
		t.Fatal("poison-task was not quarantined at the ceiling — fixture precondition for this predicate failed")
	}
	if strings.Contains(errBuf.String(), "ERROR") {
		t.Errorf("a SUCCESSFUL quarantine emitted an ERROR diagnostic: the failure log must be conditional on Promote actually failing, not unconditional.\nstderr:\n%s", errBuf.String())
	}
}

func TestC1157_005_cli_promote_nondelivery_exits_nonzero(t *testing.T) {
	bin := buildEvolve(t)
	root, inbox := newInbox(t)
	writeItem(t, inbox, "cli-stranded-task", 0)
	blockDir(t, filepath.Join(inbox, "processed"))
	t.Setenv("EVOLVE_PROJECT_ROOT", root)

	stdout, stderr, code, _ := acsassert.SubprocessOutput(bin,
		"inbox-mover", "promote", "cli-stranded-task", "processed", "1157")

	if code != 2 {
		t.Errorf("`evolve inbox-mover promote` exit = %d; want 2 (claim's mv-failed code): a stranded task must not read as ship.sh-compat success\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !containsAll(stderr, "mkdir") {
		t.Errorf("no mkdir diagnostic on cycle-visible stderr; got:\n%s", stderr)
	}
}
