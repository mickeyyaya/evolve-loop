package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func brflRunsDir(t *testing.T, evolveDir string) string {
	t.Helper()
	dir := filepath.Join(evolveDir, "runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func brflWriteRunMarker(t *testing.T, runDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMaybeRefreshChainBoundary_FleetLaneActiveRefusesRebuild(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	runsDir := brflRunsDir(t, evolveDir)
	siblingDir := filepath.Join(runsDir, "cycle-sibling-live")
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brflWriteRunMarker(t, siblingDir)
	if err := runlease.Write(siblingDir, runlease.Lease{RunID: "cycle-sibling-live"}, time.Now()); err != nil {
		t.Fatal(err)
	}

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	rebuildCalled, reexecCalled := false, false
	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { rebuildCalled = true; return nil }
	chainReExecFn = func(string, []string, []string) error { reexecCalled = true; return nil }

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if refreshed {
		t.Error("an active sibling fleet lane must never report refreshed=true")
	}
	if rebuildCalled || reexecCalled {
		t.Errorf("an active sibling fleet lane must refuse BEFORE rebuild/exec (rebuild=%v reexec=%v) — the standing rule is NEVER rebuild the plane binary mid-batch", rebuildCalled, reexecCalled)
	}
	if !strings.Contains(stderr.String(), "fleet lane") {
		t.Errorf("the refusal must be logged (auditable degrade, not a silent no-op), stderr=%q", stderr.String())
	}
}

func TestMaybeRefreshChainBoundary_FleetLaneCheckErrorRefusesRebuild(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevLane := chainBoundaryFleetLaneFn
	defer func() { chainBoundaryFleetLaneFn = prevLane }()
	chainBoundaryFleetLaneFn = func(loopConfig) (bool, error) {
		return false, errors.New("runlease: parse .evolve/runs/cycle-garbage/.lease: unexpected end of JSON input")
	}

	rebuildCalled, reexecCalled := false, false
	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { rebuildCalled = true; return nil }
	chainReExecFn = func(string, []string, []string) error { reexecCalled = true; return nil }

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if refreshed {
		t.Error("an unverifiable fleet-lane state must never report refreshed=true")
	}
	if rebuildCalled || reexecCalled {
		t.Errorf("an unverifiable fleet-lane state must refuse BEFORE rebuild/exec (rebuild=%v reexec=%v) — cannot prove the plane is idle, so it must not rebuild it", rebuildCalled, reexecCalled)
	}
	if stderr.Len() == 0 {
		t.Error("an unverifiable fleet-lane state must be logged (auditable degrade), not swallowed silently")
	}
}

func TestMaybeRefreshChainBoundary_NoFleetLaneActiveStillRefreshes(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

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
		t.Fatalf("with no active sibling lane, the boundary heal must still proceed to rebuild; refreshed=%v rebuildCalled=%v stderr=%s", refreshed, rebuildCalled, stderr.String())
	}
}

func TestDefaultChainBoundaryFleetLaneActive_DetectsLiveSiblingDiscoveredByGC(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	runsDir := brflRunsDir(t, evolveDir)
	siblingDir := filepath.Join(runsDir, "cycle-sibling-live")
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brflWriteRunMarker(t, siblingDir)
	if err := runlease.Write(siblingDir, runlease.Lease{RunID: "cycle-sibling-live"}, time.Now()); err != nil {
		t.Fatal(err)
	}

	active, err := defaultChainBoundaryFleetLaneActive(loopConfig{EvolveDir: evolveDir})
	if err != nil {
		t.Fatalf("a fresh sibling lease must not error: %v", err)
	}
	if !active {
		t.Fatal("a fresh sibling lease discovered by gc.Discover must report an active fleet lane")
	}
}

func TestDefaultChainBoundaryFleetLaneActive_NoRunsDirIsInactive(t *testing.T) {
	t.Parallel()
	active, err := defaultChainBoundaryFleetLaneActive(loopConfig{EvolveDir: t.TempDir()})
	if err != nil {
		t.Fatalf("no runs/ dir must not error: %v", err)
	}
	if active {
		t.Fatal("no runs/ dir must report no active fleet lane")
	}
}
