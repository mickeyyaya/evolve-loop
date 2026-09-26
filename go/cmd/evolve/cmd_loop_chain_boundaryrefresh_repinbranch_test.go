package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
)

// TestMaybeRefreshChainBoundary_PrePinsBeforeReExecSoChildBootRepinIsNoOp
// pins the parent-vs-child repin-branch split: the parent
// (maybeRefreshChainBoundary, pre-re-exec) performs the heal; the child's
// boot-recovery repin (attemptBootRepin, gated on detectShipSHAMismatch)
// finds nothing left to do.
func TestMaybeRefreshChainBoundary_PrePinsBeforeReExecSoChildBootRepinIsNoOp(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() { chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv }()
	chainRunningCommitFn = func() string { return "cafebabe1234" }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(c string) bool { return c == "cafebabe1234" }
	}

	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { return nil }
	// The stub never actually replaces the process image (a real syscall.Exec
	// never returns) — it stands in for "the re-exec'd child now boots".
	reExecCalled := false
	chainReExecFn = func(string, []string, []string) error { reExecCalled = true; return nil }

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 3, &stderr)

	if !refreshed {
		t.Fatalf("boundary refresh must fire on a verified stale pin; stderr=%s", stderr.String())
	}
	if !reExecCalled {
		t.Fatal("boundary refresh must reach the re-exec seam once the pin has moved")
	}

	// The PARENT branch: state.json must already carry the NEW pin (the
	// rebuilt binary's real hash), not the stale one — this is the heal, and
	// it happened strictly before the re-exec seam returned.
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("STALE_PIN")) {
		t.Fatalf("parent branch (maybeRefreshChainBoundary) must have re-pinned before re-exec: %s", raw)
	}

	// The CHILD branch: re-booting against the SAME evolveDir/binary must now
	// see no mismatch at all, so attemptBootRepin is never invoked — a
	// documented no-op, not an unexercised code path.
	var childStderr bytes.Buffer
	mismatch, onDisk := detectShipSHAMismatch(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, &childStderr)
	if mismatch {
		t.Errorf("child boot-recovery must see NO ship-SHA mismatch after the parent's pre-exec repin (on-disk=%s) — attemptBootRepin would fire redundantly, contradicting the parent-heals/child-no-ops contract this predicate pins", onDisk)
	}
}

// TestAttemptBootRepin_NoOpWhenPinAlreadyMatchesOnDiskBinary is the negative
// counterpart: attemptBootRepin, called directly (not merely gated out by
// detectShipSHAMismatch upstream), must report false — nothing to heal —
// when the pin already matches.
func TestAttemptBootRepin_NoOpWhenPinAlreadyMatchesOnDiskBinary(t *testing.T) {
	root, evolveDir, binSHA := brhProject(t, "", "ALREADY-CURRENT-BYTES")
	// Pin already matches the on-disk binary's real hash.
	brfWriteJSON(t, filepath.Join(evolveDir, "state.json"),
		map[string]any{"expected_ship_sha": binSHA})

	prev := shipRepinProvenanceFn
	defer func() { shipRepinProvenanceFn = prev }()
	shipRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return binSHA, func(c string) bool { return c == binSHA }
	}

	var stderr bytes.Buffer
	if healed := attemptBootRepin(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, &stderr); healed {
		t.Errorf("attemptBootRepin must be a no-op (return false) when the pin already matches the on-disk binary; stderr=%s", stderr.String())
	}
}
