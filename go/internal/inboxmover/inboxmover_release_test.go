package inboxmover

// inboxmover_release_test.go — RED tests for cycle-308 task
// `inbox-promote-on-ship-missing` (inbox item 2026-06-12T17-08-02Z).
//
// The gap: items claimed into processing/cycle-<N>/ are stranded forever when
// a cycle FAILs (RecoverOrphans is never auto-called) or when a claimed item is
// dropped from triage's top_n on a SUCCESSFUL ship (promoteInbox only promotes
// top_n). processing/ currently holds orphans from cycles 124/234/240/243/248/
// 265/294/295.
//
// New API this file pins (Builder implements in inboxmover.go):
//
//	ReleaseCycleProcessing(opts Options, cycle int) (RecoverResult, error)
//
// — a SCOPED, idempotent release of processing/cycle-<cycle>/*.json back to the
// inbox root. Scoped means it touches ONLY the named cycle's dir (never a
// concurrent batch's other cycles). A pre-existing inbox-root file with the same
// basename (double-move race) is a WARN, not an error, and must NOT clobber the
// existing file. Helpers makeRepo/dropProcessingFile/dropInboxFile/setCycleState
// live in inboxmover_test.go (same package).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReleaseCycleProcessing_ReleasesScopedCycle pins the core SCOPE property:
// only processing/cycle-<N>/ is drained; items under a different cycle's dir are
// left untouched (so a concurrent batch's in-progress claims are never released).
func TestReleaseCycleProcessing_ReleasesScopedCycle(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "7", "a.json", "a")
	dropProcessingFile(t, repo, "9", "b.json", "b") // different cycle — must stay

	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 7)
	if err != nil {
		t.Fatalf("ReleaseCycleProcessing err = %v", err)
	}
	if res.Recovered != 1 {
		t.Errorf("Recovered = %d, want 1 (only cycle-7's one item)", res.Recovered)
	}
	// a.json released to inbox root.
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "a.json")); err != nil {
		t.Errorf("a.json not released to inbox root: %v", err)
	}
	// cycle-7 dir drained.
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-7", "a.json")); err == nil {
		t.Errorf("a.json still in processing/cycle-7 — should have moved")
	}
	// cycle-9 untouched — release MUST be scoped to the named cycle only.
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-9", "b.json")); err != nil {
		t.Errorf("cycle-9 item wrongly released — release is not scoped to cycle 7: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "b.json")); err == nil {
		t.Errorf("cycle-9's b.json leaked to inbox root — scope violation")
	}
}

// TestReleaseCycleProcessing_Idempotent pins fail-open idempotency: a second
// call after the dir is already drained is a clean no-op (0 recovered, no error),
// not a crash on the empty/absent dir.
func TestReleaseCycleProcessing_Idempotent(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "7", "a.json", "a")

	if _, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 7); err != nil {
		t.Fatalf("first release err = %v", err)
	}
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 7)
	if err != nil {
		t.Fatalf("second release must be a clean no-op, got err = %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("second release Recovered = %d, want 0 (already drained)", res.Recovered)
	}
}

// TestInboxRelease_FailedCycleReleasesAllClaimed is the cycle-fail terminal
// scenario: every item claimed under processing/cycle-N/ returns to inbox root so
// the next batch re-triages them — no permanent strand (the cycle-124/234/240/...
// orphan class). Behavioral: asserts on the real filesystem side effects.
func TestInboxRelease_FailedCycleReleasesAllClaimed(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "12", "t1.json", "t1")
	dropProcessingFile(t, repo, "12", "t2.json", "t2")
	dropProcessingFile(t, repo, "12", "t3.json", "t3")

	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 12)
	if err != nil {
		t.Fatalf("ReleaseCycleProcessing err = %v", err)
	}
	if res.Recovered != 3 {
		t.Errorf("Recovered = %d, want 3 (all claimed items released on cycle fail)", res.Recovered)
	}
	for _, id := range []string{"t1", "t2", "t3"} {
		if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", id+".json")); err != nil {
			t.Errorf("%s not released to inbox root: %v", id, err)
		}
	}
	// processing/cycle-12 fully drained.
	left, _ := os.ReadDir(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-12"))
	if len(left) != 0 {
		t.Errorf("processing/cycle-12 still has %d file(s) — not all claims released", len(left))
	}
}

// TestInboxRelease_DoubleClaimRaceIsWarn is the negative/edge case: a file with
// the same basename already sits at the inbox root (a concurrent batch released
// it first). The release must WARN — NOT error, and NOT clobber the existing
// inbox-root file with the processing copy (which would destroy whichever copy
// the other batch is acting on).
func TestInboxRelease_DoubleClaimRaceIsWarn(t *testing.T) {
	repo := makeRepo(t)
	// Pre-existing inbox-root file with DISTINCT content (id "dup-original").
	dropInboxFile(t, repo, "dup.json", "dup-original")
	// Same basename claimed under a cycle, with different content (id "dup-claimed").
	dropProcessingFile(t, repo, "5", "dup.json", "dup-claimed")

	var stderr bytes.Buffer
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo, Stderr: &stderr}, 5)
	if err != nil {
		t.Fatalf("double-move must be a WARN, not an error; got err = %v", err)
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Errorf("double-move must emit a WARN to stderr; got %q", stderr.String())
	}
	// The pre-existing inbox-root file must survive unclobbered.
	body, readErr := os.ReadFile(filepath.Join(repo, ".evolve", "inbox", "dup.json"))
	if readErr != nil {
		t.Fatalf("inbox-root dup.json missing after release: %v", readErr)
	}
	if !strings.Contains(string(body), "dup-original") {
		t.Errorf("inbox-root file was clobbered by the processing copy: %s", body)
	}
	_ = res
}
