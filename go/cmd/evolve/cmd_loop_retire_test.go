package main

import (
	"context"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestLoopConfig_RetiringClosesTheInnerCenterBeforeTheOuterOne(t *testing.T) {
	t.Parallel()
	var order []string
	chain := loopConfig{}.retiring(func() { order = append(order, "chain") })
	batch := chain.retiring(func() { order = append(order, "batch") })

	batch.beforeReExec(func() { order = append(order, "flush") })()

	if !slices.Equal(order, []string{"batch", "chain"}) {
		t.Errorf("order = %v, want the batch Center closed first and the chain Center second, and no bare flush: a closer flushes first", order)
	}
}

func TestLoopConfig_BeforeReExecOnlyFlushesWhenNoCloserWasHandedDown(t *testing.T) {
	t.Parallel()
	flushed := 0

	loopConfig{}.beforeReExec(func() { flushed++ })()

	if flushed != 1 {
		t.Errorf("flushed %d times, want once", flushed)
	}
}

func TestBootBinaryRefresh_RetiresTheCentersAfterTheRebuildAndJustBeforeItExecs(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	swapRepinSuccess(t)
	swapExecTargetOK(t)
	cfg := loopConfig{ProjectRoot: "/proj", EvolveDir: t.TempDir()}.retiring(func() { spy.orderLog = append(spy.orderLog, "retire") })

	bootBinaryRefresh(cfg, io.Discard)

	if got := strings.Join(spy.orderLog, ","); got != "rebuild,retire,exec" {
		t.Errorf("order = %s, want rebuild,retire,exec: exec replaces the process image, so the best-effort queues drain before it", got)
	}
}

func TestBootBinaryRefresh_RetiresNothingWhenItDoesNotExec(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	swapRefreshSeams(t, "abc123def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	retired := 0
	cfg := loopConfig{ProjectRoot: "/proj", EvolveDir: t.TempDir()}.retiring(func() { retired++ })

	bootBinaryRefresh(cfg, io.Discard)

	if retired != 0 {
		t.Errorf("retired %d times on an up-to-date binary, want none: a loop that keeps running keeps its Publisher", retired)
	}
}

func TestWiredRefresher_RetiresTheCentersJustBeforeTheReExec(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	u13StubRefresh(t, func() bool { return true })
	var order []string
	chainReExecFn = func(string, []string, []string) error {
		order = append(order, "exec")
		return nil
	}
	signals, _ := recordedCenter()
	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir}.retiring(func() { order = append(order, "retire") })

	refreshed := wiredRefresher(cfg, io.Discard, signals).Refresh(context.Background(), 1)

	if !refreshed || !slices.Equal(order, []string{"retire", "exec"}) {
		t.Errorf("refreshed = %v, order = %v, want retire then exec", refreshed, order)
	}
}

func TestLoopBatchCoordinator_RetiresItsCentersBeforeAWaveBoundaryReExec(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	u13StubRefresh(t, func() bool { return true })
	var order []string
	chainReExecFn = func(string, []string, []string) error {
		order = append(order, "exec")
		return nil
	}
	signals, _ := recordedCenter()
	b := &loopBatchCoordinator{
		ctx:    context.Background(),
		cfg:    loopConfig{ProjectRoot: root, EvolveDir: evolveDir}.retiring(func() { order = append(order, "retire") }),
		deps:   orchDeps{Signals: signals},
		stderr: io.Discard,
	}

	b.maybeRefreshChainBoundaryAtWave(0)

	if !slices.Equal(order, []string{"retire", "exec"}) {
		t.Errorf("order = %v, want retire then exec at the wave boundary too", order)
	}
}

func swapPlainBatchDeps(t *testing.T, closers *[]string, name string) {
	t.Helper()
	prevDeps, prevPf := wireOrchestratorDepsFn, runLoopPreflightFn
	t.Cleanup(func() { wireOrchestratorDepsFn, runLoopPreflightFn = prevDeps, prevPf })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		c, _ := recordedCenter()
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger(), Signals: c, CloseSignals: func() { *closers = append(*closers, name) }}
	}
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result { return forcedHalt() }
}

func TestRunLoopBatch_HandsItsOwnCloserAndTheChainCloserToTheBootRefresh(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var closers []string
	swapPlainBatchDeps(t, &closers, "batch")
	var closedAtExec []string
	prevBoot := bootBinaryRefreshFn
	t.Cleanup(func() { bootBinaryRefreshFn = prevBoot })
	bootBinaryRefreshFn = func(cfg loopConfig, _ io.Writer) bootBinaryRefreshResult {
		cfg.beforeReExec(func() { closers = append(closers, "flush") })()
		closedAtExec = slices.Clone(closers)
		return bootBinaryRefreshResult{}
	}
	cfg, rc := parseLoopArgs(haltedLoopArgs(projectRoot, evolveDir), io.Discard)
	if rc != 0 {
		t.Fatalf("parseLoopArgs rc = %d", rc)
	}
	cfg = cfg.retiring(func() { closers = append(closers, "chain") })

	runLoopBatch(cfg, nil, io.Discard, io.Discard)

	if !slices.Equal(closedAtExec, []string{"batch", "chain"}) {
		t.Errorf("closers run by the exec seam = %v, want the batch Center and then the chain Center of the same process", closedAtExec)
	}
	if !slices.Equal(closers, []string{"batch", "chain", "batch"}) {
		t.Errorf("closers over the whole run = %v, want the deferred closer to run again at the return: a failed exec leaves the loop running, and the second close writes the gap records of what the first close missed", closers)
	}
}

func TestRunLoopChain_HandsItsCloserToItsBoundaryRefresh(t *testing.T) {
	root, evolveDir := u13ChainEnv(t, 1)
	u13StubRefresh(t, func() bool { return true })
	closes := map[string]int{}
	prevAttach := attachRootEventsFn
	t.Cleanup(func() { attachRootEventsFn = prevAttach })
	attachRootEventsFn = func(c *signalcenter.Center, role, _ string, _ io.Writer) func() {
		return func() {
			c.Flush()
			closes[role]++
		}
	}
	var closedAtExec map[string]int
	chainReExecFn = func(string, []string, []string) error {
		closedAtExec = maps.Clone(closes)
		return nil
	}
	prevBatch := runLoopBatchFn
	t.Cleanup(func() { runLoopBatchFn = prevBatch })
	runLoopBatchFn = func(loopConfig, io.Reader, io.Writer, io.Writer) int { return 0 }

	runLoopChain(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, policy.ChainConfig{Enabled: true, MaxBatches: 3}, nil, io.Discard, io.Discard)

	if closedAtExec["loop-chain"] != 1 {
		t.Errorf("closers run before the exec = %v, want the chain Center closed once: the boundary refresh holds the closer of the Center it flushes", closedAtExec)
	}
}
