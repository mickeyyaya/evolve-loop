package main

// cmd_loop_chain_boundaryrefresh_selflease_test.go — RED-then-GREEN
// regression guard for cycle-1364 D1/D2 (defect-ledger ids
// d0dfe5b123f4142f7765c19a4e03b3f4d / d73c31a6ef1bb00dac049419a1207f939,
// inherited through cycle-1368 into this continuation).
//
// D1: defaultChainBoundaryFleetLaneActive (cmd_loop_chain.go) had no
// self-exclusion, so the CALLING lane's own just-heartbeated run dir (a
// fresh .lease, TTL runlease.DefaultTTL=10m, written by the same process
// that is now asking "is anyone ELSE live?") read back as a live sibling.
// maybeRefreshChainBoundary therefore refused the rebuild on every single
// boundary, silently disabling the auto-refresh-binary-at-boundary feature
// this fleet_scope item exists to provide.
//
// D2: cmd_loop_chain_boundaryrefresh_fleetlane_test.go's own regression
// guard (TestMaybeRefreshChainBoundary_NoFleetLaneActiveStillRefreshes)
// deliberately leaves runs/ empty and plants no lease at all — structurally
// incapable of exercising the "my OWN lease is still fresh" path D1 lives
// in. That file's header says "Do NOT modify this file", so the missing
// case is covered here instead, in a new file, per this cycle's build-report
// disposition of D2.
//
// This file plants the CALLING process's own PID (os.Getpid()) into a fresh
// lease under a run dir and asserts the boundary heal still proceeds — the
// exact scenario D1's evidence quoted ("active=true, live=3 including this
// lane's own .evolve/runs/cycle-1364").

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// TestDefaultChainBoundaryFleetLaneActive_OwnFreshLeaseIsNotASibling is the
// direct unit-level regression guard: a run dir carrying THIS process's own
// pid in a fresh lease must not count as an active sibling.
func TestDefaultChainBoundaryFleetLaneActive_OwnFreshLeaseIsNotASibling(t *testing.T) {
	evolveDir := t.TempDir()
	runsDir := brflRunsDir(t, evolveDir)
	ownDir := filepath.Join(runsDir, "cycle-own-lane")
	if err := os.MkdirAll(ownDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brflWriteRunMarker(t, ownDir)
	if err := runlease.Write(ownDir, runlease.Lease{RunID: "cycle-own-lane", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}

	active, err := defaultChainBoundaryFleetLaneActive(loopConfig{EvolveDir: evolveDir})
	if err != nil {
		t.Fatalf("own fresh lease must not error: %v", err)
	}
	if active {
		t.Fatal("a fresh lease owned by THIS process's own pid must not report an active fleet lane — self-exclusion is required (cycle-1364 D1)")
	}
}

// TestDefaultChainBoundaryFleetLaneActive_OwnLeasePlusRealSiblingStillDetected
// proves self-exclusion never masks a GENUINE sibling: this process's own
// lease and a sibling's lease coexist under runs/, and the sibling must still
// be found.
func TestDefaultChainBoundaryFleetLaneActive_OwnLeasePlusRealSiblingStillDetected(t *testing.T) {
	evolveDir := t.TempDir()
	runsDir := brflRunsDir(t, evolveDir)

	ownDir := filepath.Join(runsDir, "cycle-own-lane")
	if err := os.MkdirAll(ownDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brflWriteRunMarker(t, ownDir)
	if err := runlease.Write(ownDir, runlease.Lease{RunID: "cycle-own-lane", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}

	siblingDir := filepath.Join(runsDir, "cycle-sibling-live")
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brflWriteRunMarker(t, siblingDir)
	// A different pid — never our own os.Getpid() — so it must still count.
	siblingPID := os.Getpid() + 1
	if err := runlease.Write(siblingDir, runlease.Lease{RunID: "cycle-sibling-live", OwnerPID: siblingPID}, time.Now()); err != nil {
		t.Fatal(err)
	}

	active, err := defaultChainBoundaryFleetLaneActive(loopConfig{EvolveDir: evolveDir})
	if err != nil {
		t.Fatalf("mixed own+sibling leases must not error: %v", err)
	}
	if !active {
		t.Fatal("a genuine sibling lease (different OwnerPID) must still be detected even when self-exclusion is applied to our own lease")
	}
}

// TestMaybeRefreshChainBoundary_OwnFreshLeaseStillRefreshes is the end-to-end
// regression guard through the actual call site: with ONLY this process's own
// fresh lease present (the exact D1 evidence shape — no other lane running),
// maybeRefreshChainBoundary must proceed to rebuild, not refuse.
func TestMaybeRefreshChainBoundary_OwnFreshLeaseStillRefreshes(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	runsDir := brflRunsDir(t, evolveDir)
	ownDir := filepath.Join(runsDir, "cycle-own-lane")
	if err := os.MkdirAll(ownDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brflWriteRunMarker(t, ownDir)
	if err := runlease.Write(ownDir, runlease.Lease{RunID: "cycle-own-lane", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}

	restore := brfStubSeams(t, true, nil, nil) // stale=true (ahead)
	defer restore()

	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() { chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv }()
	chainRunningCommitFn = func() string { return "cafebabe1234" }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(c string) bool { return c == "cafebabe1234" }
	}

	rebuildCalled := false
	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { rebuildCalled = true; return nil }
	chainReExecFn = func(string, []string, []string) error { return nil }

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if !refreshed || !rebuildCalled {
		t.Fatalf("this lane's own fresh lease must never refuse its own boundary refresh (cycle-1364 D1); refreshed=%v rebuildCalled=%v stderr=%s", refreshed, rebuildCalled, stderr.String())
	}
	if got := stderr.String(); strings.Contains(got, "fleet lane") {
		t.Errorf("must not log a fleet-lane refusal against our own lease, got stderr=%q", got)
	}
}
