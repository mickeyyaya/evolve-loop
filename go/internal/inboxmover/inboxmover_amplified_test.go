package inboxmover

// inboxmover_amplified_test.go — cycle-308 adversarial amplification tests
// for ReleaseCycleProcessing (inbox-promote-on-ship-missing).
//
// Targets gaps in the TDD contract: missing dirs, empty dirs, nil Stderr
// (panic guard), mixed-outcome batches, and double-move not counted as recovered.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReleaseCycleProcessing_EntirelyMissingProcessingDir: even the parent
// .evolve/inbox/processing/ directory doesn't exist → clean no-op. The function
// must not error on a completely absent processing tree.
func TestReleaseCycleProcessing_EntirelyMissingProcessingDir(t *testing.T) {
	repo := makeRepo(t)
	// makeRepo creates .evolve/inbox/ but NOT .evolve/inbox/processing/.
	// Verify the parent is absent so the test doesn't accidentally pass for the
	// wrong reason.
	procRoot := filepath.Join(repo, ".evolve", "inbox", "processing")
	if _, err := os.Stat(procRoot); err == nil {
		t.Skip("processing/ dir was created by makeRepo — test precondition violated")
	}

	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 42)
	if err != nil {
		t.Fatalf("entirely absent processing/ must be a clean no-op; got err = %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("Recovered = %d, want 0 (nothing to recover from an absent tree)", res.Recovered)
	}
}

// TestReleaseCycleProcessing_EmptyCycleDir: the cycle subdir exists but contains
// no files → Recovered=0, no error. Distinct from an absent dir (exercises the
// empty-ReadDir path rather than the missing-dir path).
func TestReleaseCycleProcessing_EmptyCycleDir(t *testing.T) {
	repo := makeRepo(t)
	cycleDir := filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-77")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Dir exists, zero files.
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 77)
	if err != nil {
		t.Fatalf("empty cycle dir must be a clean no-op; got err = %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("Recovered = %d, want 0 (empty dir has nothing to release)", res.Recovered)
	}
}

// TestReleaseCycleProcessing_NilStderrNoPanicOnDoubleMove: a double-move race
// must not panic when Options.Stderr is nil. The WARN must be emitted somewhere
// (e.g. swallowed or directed to os.Stderr), but the important invariant is: no
// panic and err=nil.
func TestReleaseCycleProcessing_NilStderrNoPanicOnDoubleMove(t *testing.T) {
	repo := makeRepo(t)
	// Pre-existing inbox-root file AND same basename in processing/ → double-move.
	dropInboxFile(t, repo, "dup.json", "dup-original")
	dropProcessingFile(t, repo, "5", "dup.json", "dup-claimed")

	// Stderr=nil (zero value) — must NOT panic.
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 5) // Stderr omitted = nil
	if err != nil {
		t.Fatalf("double-move with nil Stderr must not error; got err = %v", err)
	}
	// The original inbox-root file must survive unclobbered regardless of Stderr.
	body, readErr := os.ReadFile(filepath.Join(repo, ".evolve", "inbox", "dup.json"))
	if readErr != nil {
		t.Fatalf("inbox-root dup.json missing: %v", readErr)
	}
	if !strings.Contains(string(body), "dup-original") {
		t.Errorf("inbox-root file was clobbered with nil Stderr: %s", body)
	}
	_ = res
}

// TestReleaseCycleProcessing_MixedOutcome: 2 clean files + 1 double-move in the
// same cycle dir. The clean files must be released (Recovered=2), the collision
// must WARN but not error, and the total must be consistent (no off-by-one from
// treating the collision as a recovered item).
func TestReleaseCycleProcessing_MixedOutcome(t *testing.T) {
	repo := makeRepo(t)
	// Two clean items.
	dropProcessingFile(t, repo, "10", "clean1.json", "clean1")
	dropProcessingFile(t, repo, "10", "clean2.json", "clean2")
	// One collision: pre-existing inbox-root file.
	dropInboxFile(t, repo, "coll.json", "coll-original")
	dropProcessingFile(t, repo, "10", "coll.json", "coll-claimed")

	var stderr bytes.Buffer
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo, Stderr: &stderr}, 10)
	if err != nil {
		t.Fatalf("mixed-outcome release must not error; got err = %v", err)
	}
	if res.Recovered != 2 {
		t.Errorf("Recovered = %d, want 2 (only the two clean items count)", res.Recovered)
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Errorf("collision must emit WARN to stderr; got %q", stderr.String())
	}
	// Clean items released to inbox root.
	for _, id := range []string{"clean1.json", "clean2.json"} {
		if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", id)); err != nil {
			t.Errorf("%s not released to inbox root: %v", id, err)
		}
	}
	// Collision: original inbox-root file must survive unclobbered.
	body, readErr := os.ReadFile(filepath.Join(repo, ".evolve", "inbox", "coll.json"))
	if readErr != nil {
		t.Fatalf("inbox-root coll.json missing: %v", readErr)
	}
	if !strings.Contains(string(body), "coll-original") {
		t.Errorf("inbox-root collision file was clobbered: %s", body)
	}
}

// TestReleaseCycleProcessing_DoubleMove_NotCountedAsRecovered: a collision (file
// already at inbox root) must NOT increment Recovered — only a successful move
// counts. This pins the accounting invariant: Recovered = actual filesystem moves.
func TestReleaseCycleProcessing_DoubleMove_NotCountedAsRecovered(t *testing.T) {
	repo := makeRepo(t)
	dropInboxFile(t, repo, "x.json", "x-original")
	dropProcessingFile(t, repo, "3", "x.json", "x-claimed")

	var stderr bytes.Buffer
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo, Stderr: &stderr}, 3)
	if err != nil {
		t.Fatalf("collision must not error; got err = %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("Recovered = %d for a pure-collision release; want 0 (no successful move)", res.Recovered)
	}
}
