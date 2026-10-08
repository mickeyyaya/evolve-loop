package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// haltMarker is the operator-facing string blockerBreakerHalt prints on a
// trip (cmd_loop_blockerbreaker.go). Asserting on the breadcrumb rather than
// on runLoop's exit code keeps these predicates pinned to the BREAKER's
// behavior and immune to unrelated changes in the loop's own exit vocabulary.
const haltMarker = "LOOP_PIPELINE_BLOCKER_HALT"

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

func TestReadBatchWindowFloor_ReturnsTheStateReadFault(t *testing.T) {
	fault := errors.New("state.json is locked")
	st := &fixtures.FakeStorage{ReadStateErr: fault, State: core.State{LastCycleNumber: 1325}}

	got, err := readBatchWindowFloor(context.Background(), st)

	if !errors.Is(err, fault) || got != 0 {
		t.Errorf("readBatchWindowFloor = %d, %v; want 0 and the read fault", got, err)
	}
}

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

func TestPrepareIteration_ResolvesTheFleetBinaryWhenThePolicyAsksForAFleet(t *testing.T) {
	root, evolveDir := iscsProject(t)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"fleet":{"count":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	st := &fixtures.FakeStorage{State: core.State{LastCycleNumber: iscsLastCycle, LastAllocatedCycleNumber: iscsLastCycle}}
	b, console := iscsCoordinator(root, evolveDir, st)
	b.cycleEnv = map[string]string{"EVOLVE_CLI_HEALTH": "0"}
	fc, bin := policy.FleetConfig{Count: 1}, ""
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	d := b.prepareIteration(0, &fc, &bin, iscsLastCycle)
	b.deps.Signals.Flush()

	if d.flow != batchProceed || fc.Count != 2 || bin != self {
		t.Fatalf("flow=%v count=%d binary=%q, want batchProceed, count 2 and %q; console:\n%s", d.flow, fc.Count, bin, self, console.String())
	}
}

func TestTheFleetBinaryFallbackHasOneHome(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var homes []string
	for _, f := range sources {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), "cannot resolve binary for fleet dispatch") {
			homes = append(homes, f)
		}
	}
	if len(homes) != 1 || homes[0] != "cmd_loop_window.go" {
		t.Fatalf("resolving the fleet binary, or falling back to sequential, must live only in prepareIteration's resolveWaveBinary (cmd_loop_window.go); found it in %v", homes)
	}
}
