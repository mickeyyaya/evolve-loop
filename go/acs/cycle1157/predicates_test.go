//go:build acs

// Package cycle1157 materialises the acceptance criteria for the single task
// triage COMMITTED to this fleet lane (triage-report.md `## top_n`):
//
//   - inboxmover-promote-mkdir-fail-loud → 001-005
//
// No other id was assigned to this lane and nothing was deferred, so there is no
// deferred-floor predicate here (R9.3 floor-binding: predicates bind only to
// triage-committed work — cycle-280).
//
// # Continuation context (READ THIS FIRST, Builder)
//
// Cycle 1157 continues cycle 1156 under ADR-0076: this branch carries 1156's
// salvage snapshot (9effecb2), which already inverted the PRODUCER half of the
// contract — Promote (inboxmover.go:314-326) now returns ErrMvFailed on a
// destination mkdir failure instead of the (NoOp=true, nil) ship.sh-compat lie.
// Predicates 001, 002, 004 and 005 therefore start GREEN and are REGRESSION
// LOCKS on salvaged work: their job is to make an accidental revert loud, and
// the test-report records them as pre-existing GREEN rather than claiming a RED
// this cycle did not produce.
//
// Predicate 003 is the genuine RED. Making Promote fail loud only helps where a
// caller actually READS the error, and one caller still throws it on the floor:
//
//	// inboxmover.go:697 (releaseCycleProcessing, ADR-0072 S5 quarantine path)
//	if pr, pErr := Promote(opts, taskID, "quarantine", ...); pErr == nil && !pr.NoOp {
//	    ... quarantine bookkeeping ...
//	}
//
// pErr is bound and never inspected. When quarantine's destination mkdir fails,
// the item silently falls through to the ordinary release: it returns to the
// inbox root, the next triage re-picks the exact poison task the S5 ceiling
// exists to park, and NOTHING anywhere — stderr, ledger, or result — says the
// quarantine was attempted and failed. That is the same swallow the task names,
// one call site downstream of the fix, and it is the last one in the package
// (outcome.go:74 and ReconcileSuperseded both propagate correctly).
//
// The contract 003 pins is loud-but-fail-open, deliberately: the drain must
// still release the item (a quarantine mkdir failure that ALSO strands the file
// in processing/ would be a worse defect than the silent one), but the failure
// must reach the cycle's stderr naming the task and the quarantine attempt.
// Predicate 004 is its negative twin — a Builder who unconditionally logs an
// error to satisfy 003 fails 004.
//
// # Predicate quality (cycle-85 ban)
//
// Every predicate below CALLS the production function and asserts on its
// returned error, the diagnostic it emitted, or where the item physically
// landed on disk; 005 runs the real `evolve` binary and asserts its exit code.
// None is a source-grep for a magic string, so none can be satisfied by adding
// a comment or a literal.
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

// --- fixture helpers --------------------------------------------------------

// newInbox builds an isolated project root with an empty .evolve/inbox/ and
// returns (projectRoot, inboxDir). The lifecycle under test is filesystem
// shaped, so every predicate gets its own tree.
func newInbox(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	return root, inbox
}

// writeItem drops an inbox item JSON carrying id (and an optional pre-existing
// failure_count) into dir, mirroring the real <timestamp>-<id>.json convention.
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

// blockDir plants a REGULAR FILE where the lifecycle wants a directory, so the
// next os.MkdirAll on that path fails with ENOTDIR. This is the deterministic,
// permission-independent way to force the mkdir branch (running as root would
// defeat a chmod-based fixture); it is the same technique the package's own
// TestPromote_MkdirFailsLoudly uses.
func blockDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("block %s: %v", path, err)
	}
}

// testOpts returns Options rooted at root with the landing gate stubbed to
// "landed" — the real gate shells out to git, which is noise for a temp dir.
func testOpts(root string, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{
		ProjectRoot: root,
		Stderr:      stderr,
		IsLandedFn:  func(string) (bool, error) { return true, nil },
	}
}

// findItem returns the path of the file directly under dir whose JSON .id == id,
// or "" when no such file exists (including when dir is absent).
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

// containsAll reports whether s contains every needle (case-insensitive).
func containsAll(s string, needles ...string) bool {
	low := strings.ToLower(s)
	for _, n := range needles {
		if !strings.Contains(low, strings.ToLower(n)) {
			return false
		}
	}
	return true
}

// hasLineWith reports whether SOME SINGLE line of s contains every needle.
// Whole-buffer matching is not good enough for a diagnostic assertion: the
// unrelated "released: <file>" line already carries the task id and Promote's
// own mkdir line already carries the quarantine path, so a buffer-wide check
// passes today without anyone ever saying "this task failed to quarantine".
// The claim only holds if one line ties them together.
func hasLineWith(s string, needles ...string) bool {
	for _, line := range strings.Split(s, "\n") {
		if containsAll(line, needles...) {
			return true
		}
	}
	return false
}

// buildEvolve compiles the real evolve binary into a temp dir and returns its
// path. Predicates exec the BINARY rather than `go run ./cmd/evolve` because
// `go run` collapses every child exit code to its own 1, which would make an
// exact exit-code contract unassertable.
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

// --- Task: inboxmover-promote-mkdir-fail-loud -------------------------------

// AC1 (regression lock, salvaged in 1156): a destination mkdir failure is an
// infrastructure NON-DELIVERY, so Promote must return an ErrMvFailed-wrapped
// error with NoOp=false — NoOp is the "source already moved" compat contract and
// must never cover a stranded task. The item stays exactly where it was: a loud
// error that ALSO lost the file would be worse than the silent no-op it
// replaced.
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

// AC2 (regression lock, salvaged in 1156): the seam that promotes on a PASS
// (ApplyCycleOutcome → postship) must PROPAGATE that error rather than discard
// it — the original defect had a caller doing `_, _ = Promote(...)`, so even a
// loud producer would have been silent in production. A failed promote must also
// never be reported as promoted.
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

// AC3 (RED — the remaining swallow): releaseCycleProcessing's ADR-0072 S5
// quarantine path binds Promote's error and never inspects it
// (inboxmover.go:697, `pErr == nil && !pr.NoOp`). When the quarantine mkdir
// fails, the poison item silently falls back to the ordinary release and the
// next triage re-picks the exact task the ceiling exists to park.
//
// Contract: loud, but still fail-open. The drain MUST keep releasing the item
// (stranding it in processing/ would be a worse defect), and the failed
// quarantine MUST reach the cycle-visible stderr naming the task id and the
// quarantine attempt.
func TestC1157_003_quarantine_promote_failure_is_surfaced(t *testing.T) {
	root, inbox := newInbox(t)
	// failure_count 1 + ceiling 2 → this drain bumps to 2 and quarantines.
	writeItem(t, filepath.Join(inbox, "processing", "cycle-1157"), "poison-task", 1)
	blockDir(t, filepath.Join(inbox, "quarantine"))

	var errBuf bytes.Buffer
	res, err := inboxmover.ReleaseCycleProcessingWithQuarantine(
		testOpts(root, &errBuf), 1157, "cycle-failure-release", 2, false)
	if err != nil {
		t.Fatalf("ReleaseCycleProcessingWithQuarantine err = %v; the drain stays fail-open — the failed quarantine is reported, not raised", err)
	}

	stderr := errBuf.String()
	if !hasLineWith(stderr, "poison-task", "quarantine") {
		t.Errorf("failed quarantine promote is invisible: no single stderr line ties the task id to the failed quarantine attempt.\n"+
			"A silently un-quarantined poison item returns to the inbox root and is re-picked next cycle.\nstderr:\n%s", stderr)
	}
	if !hasLineWith(stderr, "poison-task", "quarantine", "ERROR") && !hasLineWith(stderr, "poison-task", "quarantine", "WARN") {
		t.Errorf("quarantine failure reported without an ERROR/WARN severity marker on the same line — operators grep severity; got:\n%s", stderr)
	}
	// Fail-open half: the item must still have been released, not stranded.
	if findItem(t, inbox, "poison-task") == "" {
		t.Errorf("poison-task is no longer at the inbox root after the failed quarantine (Recovered=%d): the drain must stay fail-open, a loud failure must not also strand the item in processing/", res.Recovered)
	}
	if findItem(t, filepath.Join(inbox, "processing", "cycle-1157"), "poison-task") != "" {
		t.Error("poison-task left behind in processing/cycle-1157/: the failed quarantine must fall through to the ordinary release")
	}
}

// AC3-negative (anti-no-op twin of 003): a SUCCESSFUL quarantine must emit no
// failure diagnostic. Without this, "always log an error in the quarantine
// branch" would satisfy 003 while telling operators the S5 ceiling is broken on
// every healthy park.
func TestC1157_004_successful_quarantine_emits_no_failure_diagnostic(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "processing", "cycle-1157"), "poison-task", 1)
	// No blockDir: inbox/quarantine/ is creatable, so the promote succeeds.

	var errBuf bytes.Buffer
	if _, err := inboxmover.ReleaseCycleProcessingWithQuarantine(
		testOpts(root, &errBuf), 1157, "cycle-failure-release", 2, false); err != nil {
		t.Fatalf("ReleaseCycleProcessingWithQuarantine: %v", err)
	}

	if findItem(t, filepath.Join(inbox, "quarantine"), "poison-task") == "" {
		t.Fatal("poison-task was not quarantined at the ceiling — fixture precondition for this predicate failed")
	}
	if strings.Contains(errBuf.String(), "ERROR") {
		t.Errorf("a SUCCESSFUL quarantine emitted an ERROR diagnostic: the failure log must be conditional on Promote actually failing, not unconditional.\nstderr:\n%s", errBuf.String())
	}
}

// AC4/AC5 (regression lock, salvaged in 1156 — edge/OOD axis through the real
// binary): the cmd layer must map a promote non-delivery to a non-zero exit,
// matching claim's existing mv-failed code (2), so a cycle shell-invoking
// `evolve inbox-mover promote` cannot read a stranded task as success. Exit 0
// stays reserved for the genuine ship.sh-compat paths.
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
