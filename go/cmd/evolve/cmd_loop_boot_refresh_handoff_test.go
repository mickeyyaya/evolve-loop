package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	bootRefreshOldImageCommit = "abc123def456"
	bootRefreshNewImageCommit = "eeee23def456"
	bootRefreshNewHead        = "eeee23def456fffffffffffffffffffffffffff0"
)

type nextImage struct {
	execs     int
	wavesDone int
	handedOff bool
}

func interceptExecAsNextImage(t *testing.T, evolveDir string) *nextImage {
	t.Helper()
	next := &nextImage{}
	prevExec, prevCommit := bootRefreshExecFn, chainRunningCommitFn
	t.Cleanup(func() { bootRefreshExecFn, chainRunningCommitFn = prevExec, prevCommit })
	bootRefreshExecFn = func() error {
		next.execs++
		chainRunningCommitFn = func() string { return bootRefreshNewImageCommit }
		next.wavesDone, next.handedOff = takeReexecHandoff(evolveDir, io.Discard)
		return nil
	}
	return next
}

func staleBinaryWithGoDelta(t *testing.T) {
	t.Helper()
	swapBinaryCommit(t, bootRefreshOldImageCommit)
	swapRefreshSeams(t, bootRefreshNewHead, nil, true, nil, nil, nil)
	swapRepinSuccess(t)
	swapExecTargetOK(t)
	swapFleetLaneCheck(t, false, nil)
}

func TestBootBinaryRefresh_ReExecArmsTheWaveHandoff(t *testing.T) {
	const wavesDone = 3
	staleBinaryWithGoDelta(t)
	evolveDir := t.TempDir()
	next := interceptExecAsNextImage(t, evolveDir)

	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: evolveDir, ResumeWaves: wavesDone}, &bytes.Buffer{})

	if !res.Rebuilt || next.execs != 1 {
		t.Fatalf("stale binary with a go/ delta must rebuild and re-exec once: res=%+v execs=%d", res, next.execs)
	}
	if !next.handedOff || next.wavesDone != wavesDone {
		t.Fatalf("the re-exec'd image must resume at wave %d, got wavesDone=%d handedOff=%v", wavesDone, next.wavesDone, next.handedOff)
	}
}

func TestBootBinaryRefresh_ZeroResumeWavesArmsNothing(t *testing.T) {
	staleBinaryWithGoDelta(t)
	evolveDir := t.TempDir()
	next := interceptExecAsNextImage(t, evolveDir)

	bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: evolveDir}, &bytes.Buffer{})

	if next.execs != 1 {
		t.Fatalf("a boot with no resumed waves still re-execs once, got %d", next.execs)
	}
	if next.handedOff || next.wavesDone != 0 {
		t.Fatalf("nothing was armed, so nothing may be taken: wavesDone=%d handedOff=%v", next.wavesDone, next.handedOff)
	}
	if _, err := os.Stat(filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile)); !os.IsNotExist(err) {
		t.Fatalf("no attempt marker may be written when there is nothing to hand off: %v", err)
	}
}

func TestBootBinaryRefresh_FailedArmBootsAsIsWithoutReExec(t *testing.T) {
	staleBinaryWithGoDelta(t)
	evolveDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile), 0o755); err != nil {
		t.Fatal(err)
	}
	next := interceptExecAsNextImage(t, evolveDir)

	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: evolveDir, ResumeWaves: 2}, &bytes.Buffer{})

	if next.execs != 0 {
		t.Fatalf("an unwritable handoff marker must not be followed by a re-exec that drops the wave resume point, execs=%d", next.execs)
	}
	if !res.Rebuilt {
		t.Fatalf("the rebuild already happened and stays reported: %+v", res)
	}
}

func TestRunLoop_BootRefreshReExecCarriesTheResumePointTheLaunchTook(t *testing.T) {
	const wavesDone = 2
	staleBinaryWithGoDelta(t)
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prevCommit := chainRunningCommitFn
	t.Cleanup(func() { chainRunningCommitFn = prevCommit })
	chainRunningCommitFn = func() string { return bootRefreshOldImageCommit }
	armed, err := json.Marshal(map[string]any{
		"running_commit": "0ldc0mm1t000",
		"batch":          1,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"pid":            os.Getpid(),
		"waves_done":     wavesDone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile), armed, 0o644); err != nil {
		t.Fatal(err)
	}
	next := interceptExecAsNextImage(t, evolveDir)

	prevRecover := bootRecoverFn
	t.Cleanup(func() { bootRecoverFn = prevRecover })
	bootRecoverFn = func(_ context.Context, _ loopConfig, _ core.Ledger, _ io.Writer) bootRecoveryResult {
		return bootRecoveryResult{HaltSelfSHA: true}
	}
	prevDeps := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = prevDeps })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger()}
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "anything",
		"--cycles", "1",
		"--force-fresh",
	}, nil, &stdout, &stderr)

	if rc != 2 {
		t.Fatalf("boot recovery halt expected rc=2, got %d; stderr=%q", rc, stderr.String())
	}
	if next.execs != 1 {
		t.Fatalf("the launch must reach the boot refresh re-exec once, got %d; stderr=%q", next.execs, stderr.String())
	}
	if !next.handedOff || next.wavesDone != wavesDone {
		t.Fatalf("the image that took %d waves from the launch must pass them on through the re-exec, got wavesDone=%d handedOff=%v", wavesDone, next.wavesDone, next.handedOff)
	}
}

type planeFixture struct {
	plane *gittest.Repo
	base  string
}

func newShippedPlane(t *testing.T) planeFixture {
	t.Helper()
	seed := gittest.Fixture(t)
	seed.Write("go/a.go", "package a\n")
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "base")
	base := seed.Git("rev-parse", "HEAD")
	origin := gittest.Bare(t)
	seed.Git("push", "-q", origin.Dir, "main")
	return planeFixture{plane: gittest.Clone(t, origin.Dir), base: base}
}

func (p planeFixture) commitGoChange(file string) {
	p.plane.Write("go/"+file, "package a\n")
	p.plane.Git("add", "-A")
	p.plane.Git("commit", "-q", "-m", "change "+file)
}

func realGitBootRefreshSeams(t *testing.T, binaryCommit string) *refreshSpy {
	t.Helper()
	swapBinaryCommit(t, binaryCommit)
	spy := swapRefreshSeams(t, "", nil, false, nil, nil, nil)
	bootRefreshHeadFn, bootRefreshSourceDeltaFn = defaultBootRefreshHead, defaultBootRefreshSourceDelta
	swapRepinSuccess(t)
	swapExecTargetOK(t)
	swapFleetLaneCheck(t, false, nil)
	return spy
}

func TestBootBinaryRefresh_UnshippedPlaneHeadDoesNotRebuild(t *testing.T) {
	p := newShippedPlane(t)
	p.commitGoChange("unshipped.go")
	spy := realGitBootRefreshSeams(t, p.base[:12])

	res := bootBinaryRefresh(loopConfig{ProjectRoot: p.plane.Dir, EvolveDir: t.TempDir()}, &bytes.Buffer{})

	if spy.rebuildCalls != 0 || spy.execCalls != 0 || res.Rebuilt {
		t.Fatalf("a plane HEAD ahead of origin must not be built into the loop binary: res=%+v spy=%+v", res, spy)
	}
}

func TestBootBinaryRefresh_ShippedPlaneHeadStillRebuilds(t *testing.T) {
	p := newShippedPlane(t)
	p.commitGoChange("shipped.go")
	p.plane.Git("push", "-q", "origin", "main")
	spy := realGitBootRefreshSeams(t, p.base[:12])

	res := bootBinaryRefresh(loopConfig{ProjectRoot: p.plane.Dir, EvolveDir: t.TempDir()}, &bytes.Buffer{})

	if !res.Rebuilt || spy.rebuildCalls != 1 || spy.execCalls != 1 {
		t.Fatalf("a shipped go/ delta must still rebuild and re-exec once: res=%+v spy=%+v", res, spy)
	}
}
