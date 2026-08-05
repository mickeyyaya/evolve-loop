package main

// cmd_loop_window_test.go — RED contract for Defect B of the cycle-1335
// incident (fault-localization-report.md, suspects 1/2/6; premise-challenge
// Attack 1 corrected the framing).
//
// The defect, verified on live state: aborted cycles exit through
// abnormalEpilogue (cyclerun_epilogue.go:41-107), which writes the failure
// digest but NEVER advances state.LastCycleNumber — every loopAbort path
// returns from RunCycle (orchestrator.go:865-916) before the finalizeCycle
// call at orchestrator.go:969. Meanwhile the breaker's batch window is
// derived from that same COMPLETION counter (cmd_loop.go:522-525), so the
// digests of cycles 1326/1328/1329 (all phase:"aborted", all carrying
// fingerprint ship|unknown|76d0f4fca190) stayed inside `> batchStartCycle`
// on every relaunch and Rule B tripped at i==0, before any cycle ran. Three
// re-halts; the operator's repair was a manual lastCycleNumber advance.
//
// The fix anchors the window on the monotone ALLOCATION lease instead:
// state.LastAllocatedCycleNumber advances at MINT time (alloc.go:14-17 —
// "a crashed run BURNS its number"), so it tracks cycles DISPATCHED while
// LastCycleNumber tracks cycles COMPLETED. A time boundary belongs on the
// dispatch counter. Live state carries exactly this asymmetry:
// lastCycleNumber=1334 alongside lastAllocatedCycleNumber=1335.
//
// NOT reset.go: nothing in the tree rewinds either counter — reset.go:322
// advances under the comment "number never reused" (fault-localization V11).

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// haltMarker is the operator-facing string blockerBreakerHalt prints on a
// trip (cmd_loop_blockerbreaker.go). Asserting on the breadcrumb rather than
// on runLoop's exit code keeps these predicates pinned to the BREAKER's
// behavior and immune to unrelated changes in the loop's own exit vocabulary.
const haltMarker = "PIPELINE-BLOCKER HALT"

// TestReadBatchWindowFloor_PrefersAllocationLease is the unit-level core of
// the fix: the breaker window floor must be the MAX of the completion and
// allocation counters, so digests written by cycles that were minted but
// never completed fall OUTSIDE a fresh batch's window.
//
// Shape taken from live state at the time of the incident: three aborted
// cycles at 1326/1328/1329 left lastCycleNumber pinned at 1325 while the
// allocation lease had advanced to 1330.
func TestReadBatchWindowFloor_PrefersAllocationLease(t *testing.T) {
	st := &fixtures.FakeStorage{State: core.State{
		LastCycleNumber:          1325,
		LastAllocatedCycleNumber: 1330,
	}}
	got, err := readBatchWindowFloor(context.Background(), st)
	if err != nil {
		t.Fatalf("readBatchWindowFloor: %v", err)
	}
	if got != 1330 {
		t.Fatalf("window floor = %d, want 1330 (the allocation lease) — anchoring on the completion counter (1325) re-collects the aborted cycles' digests on every relaunch, which is the cycle-1335 triple re-halt", got)
	}
}

// TestReadBatchWindowFloor_LegacyStateFallsBackToCompletionCounter is the
// edge case that forbids a bare swap: allocateCycle falls back to
// LastCycleNumber+1 when storage is not a StateUpdater (alloc.go:47-51), so a
// legacy state can carry lastAllocatedCycleNumber=0. A bare swap would zero
// the window and re-collect the entire runs/ history — a far worse halt.
func TestReadBatchWindowFloor_LegacyStateFallsBackToCompletionCounter(t *testing.T) {
	st := &fixtures.FakeStorage{State: core.State{
		LastCycleNumber:          1325,
		LastAllocatedCycleNumber: 0,
	}}
	got, err := readBatchWindowFloor(context.Background(), st)
	if err != nil {
		t.Fatalf("readBatchWindowFloor: %v", err)
	}
	if got != 1325 {
		t.Fatalf("window floor = %d, want 1325 — a legacy state with no allocation lease must fall back to the completion counter, never to 0 (which would re-collect all of runs/)", got)
	}
}

// TestReadLastCycleNumber_StillReportsCompletionCounter pins the OTHER
// reader unchanged. unfinishedCycle (cmd_loop_control.go:179-181) compares
// cs.CycleID against the COMPLETION counter to detect a stuck cycle; if the
// fix widened readLastCycleNumber in place, a minted-but-unfinished cycle
// would stop being flagged. Two counters, two named readers.
func TestReadLastCycleNumber_StillReportsCompletionCounter(t *testing.T) {
	st := &fixtures.FakeStorage{State: core.State{
		LastCycleNumber:          1325,
		LastAllocatedCycleNumber: 1330,
	}}
	got, err := readLastCycleNumber(context.Background(), st)
	if err != nil {
		t.Fatalf("readLastCycleNumber: %v", err)
	}
	if got != 1325 {
		t.Fatalf("readLastCycleNumber = %d, want 1325 — it must keep reporting the COMPLETION counter; unfinishedCycle's stuck-cycle detection depends on it", got)
	}
}

// TestRunLoop_AbortedCycleDigestsFallOutsideBatchWindow is the caller proof:
// the incident replayed through the REAL production entrypoint (runLoop),
// not through the derivation helper in isolation. A predicate that only
// calls readBatchWindowFloor proves nothing about cmd_loop.go:525 actually
// consuming it.
func TestRunLoop_AbortedCycleDigestsFallOutsideBatchWindow(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "off")
	// The exact live shape: three aborted cycles' digests share one
	// fingerprint (ceiling is 3), the completion counter is stuck behind
	// them, the allocation lease is ahead of them.
	writeDigestFixture(t, evolveDir, 1326, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1328, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1329, incidentFingerprint, "gate-block")

	storage := &fixtures.FakeStorage{State: core.State{
		LastCycleNumber:          1325,
		LastAllocatedCycleNumber: 1330,
	}}
	defer installStubDeps(t, storage, newFakeLedger())()

	var stdout, stderr bytes.Buffer
	runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
	}, nil, &stdout, &stderr)

	if strings.Contains(stderr.String(), haltMarker) {
		t.Fatalf("a fresh relaunch must NOT halt on digests from cycles that were minted (lease=1330) but never completed — this is the cycle-1335 triple re-halt; stderr=%q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "pipeline-escalation.json")); err == nil {
		t.Fatal("no escalation dossier may be written when the batch window excludes every digest")
	}
}

// TestRunLoop_InBatchDigestsStillHalt is the negative half: the fix must not
// weaken Rule B. Digests minted INSIDE this batch (above BOTH counters) must
// still trip the breaker at the ceiling, through the same real entrypoint.
func TestRunLoop_InBatchDigestsStillHalt(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "off")
	writeDigestFixture(t, evolveDir, 1331, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1332, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1333, incidentFingerprint, "gate-block")

	storage := &fixtures.FakeStorage{State: core.State{
		LastCycleNumber:          1325,
		LastAllocatedCycleNumber: 1330,
	}}
	defer installStubDeps(t, storage, newFakeLedger())()

	var stdout, stderr bytes.Buffer
	runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
	}, nil, &stdout, &stderr)

	if !strings.Contains(stderr.String(), haltMarker) {
		t.Fatalf("3x identical-fingerprint digests minted INSIDE the batch (above both counters) must still halt — the window fix must not weaken Rule B's sensitivity; stderr=%q", stderr.String())
	}
}
