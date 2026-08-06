package main

// cmd_loop_chain_boundaryrefresh_repinbranch_test.go — RED test (cycle 1356,
// inbox item auto-refresh-binary-at-boundary, task
// pin-boundary-repin-branch-residual).
//
// Residual embedded in the inbox item's own live-fire note (2026-08-05
// 17:24): a boot-time binary refresh healed via the RE-EXEC'D CHILD's
// boot-recovery auto-repin rather than the PARENT's pre-exec reconcile ("the
// pin heal fired in the CHILD's auto-repin... verify which parent-repin
// branch no-op'd and tighten its test").
//
// Read-first (rule 8): that boot-time mechanism (a `cmd_loop_boot_refresh.go`
// file, `bootRefreshRepinFn` seam) does not exist anywhere in this worktree's
// checked-out source —
//
//	grep -rn 'bootRefreshRepinFn|BootBinaryRefresh' go/   -> zero hits
//	find go -name 'cmd_loop_boot_refresh*.go'             -> no file
//
// This worktree's merge-base with origin/main (4dadf62a923640c) is 71 commits
// behind current main; the boot-time rebuild+re-exec self-heal is a main-line
// feature this worktree's snapshot predates. Re-litigating the EXACT
// parent/child split the note describes would mean writing a predicate
// against code that is not present here — inventing an API, which rule 8
// forbids.
//
// What IS present, and carries the identical shape (a repin that can fire on
// either side of a re-exec boundary), is THIS SAME inbox item's other half:
// maybeRefreshChainBoundary (cmd_loop_chain.go) re-pins expected_ship_sha
// BEFORE it re-execs. The re-exec'd child's own boot path
// (defaultBootRecovery -> detectShipSHAMismatch -> attemptBootRepin,
// cmd_loop_boot_recovery.go) is the other candidate healer. Nothing pins
// which of the two actually performs the heal for a boundary refresh, or
// proves the other is a documented no-op rather than an accidental race —
// exactly the ambiguity class the live-fire note flagged, applied to the
// mechanism this worktree actually has.
//
// This predicate proves: (1) maybeRefreshChainBoundary's pre-exec repin is
// the branch that performs the heal (state.json's pin moves BEFORE the
// re-exec seam is invoked), and (2) with that pin already moved, the child
// boot path's detectShipSHAMismatch reports NO mismatch — attemptBootRepin is
// therefore a documented no-op on the boundary-refresh path, never reached.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
)

// TestMaybeRefreshChainBoundary_PrePinsBeforeReExecSoChildBootRepinIsNoOp
// pins the parent-vs-child repin-branch split for the boundary-refresh
// mechanism: the PARENT (maybeRefreshChainBoundary, pre-re-exec) performs the
// heal; the CHILD's boot-recovery repin (attemptBootRepin, gated on
// detectShipSHAMismatch) finds nothing left to do.
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
// counterpart: attemptBootRepin itself, called directly (not merely gated out
// by detectShipSHAMismatch upstream), must report false — "nothing to heal" —
// when the pin already matches. This is the "tighten its test" half of the
// residual note: the no-op branch gets its own direct assertion, not just an
// inference from detectShipSHAMismatch's gate.
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
