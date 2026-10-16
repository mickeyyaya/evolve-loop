package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestDefaultChainBoundaryFleetLaneActive_OwnFreshLeaseIsNotASibling(t *testing.T) {
	t.Parallel()
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

func TestDefaultChainBoundaryFleetLaneActive_OwnLeasePlusRealSiblingStillDetected(t *testing.T) {
	t.Parallel()
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
	sibling := exec.Command("sleep", "30")
	if err := sibling.Start(); err != nil {
		t.Fatalf("spawn a live sibling: %v", err)
	}
	t.Cleanup(func() { _ = sibling.Process.Kill(); _ = sibling.Wait() })
	siblingPID := sibling.Process.Pid
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

	restore := brfStubSeams(t, true, nil, nil)
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
