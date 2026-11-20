package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type cycleStartSeqRunner struct {
	events *[]string
}

func (r cycleStartSeqRunner) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	*r.events = append(*r.events, "cycle")
	return core.CycleResult{Cycle: 1}, nil
}

func (r cycleStartSeqRunner) RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error) {
	return core.CycleResult{Cycle: 1}, nil
}

func (cycleStartSeqRunner) SignalSummary() signalcenter.Summary { return signalcenter.Summary{} }

func TestCycleStartTrim_LaunchFnRunsFakeTrimAndGuardsDisk(t *testing.T) {
	t.Parallel()
	binDir := t.TempDir()
	fakeclitest.Install(t, filepath.Join(binDir, "evolve"), "#!/bin/sh\necho launched\n")
	bin := filepath.Join(binDir, "evolve")

	const total = uint64(100) << 30
	pcts := []float64{4, 6}
	step := 0
	var events []string
	var stdout, stderr bytes.Buffer

	launch := cycleLaunch{
		Bin:      bin,
		Root:     t.TempDir(),
		GoalHash: "deadbeef",
		Stdout:   &stdout,
		Stderr:   &stderr,
		startHooks: cycleStartHooks{
			trim: func() error {
				events = append(events, "trim")
				return nil
			},
			usage: func(string) (looppreflight.DiskUsage, error) {
				events = append(events, "disk")
				return looppreflight.DiskUsage{FreeBytes: uint64(pcts[step] * float64(total) / 100), TotalBytes: total}, nil
			},
			wait: func(context.Context) error {
				events = append(events, "wait")
				step++
				return nil
			},
		},
	}.LaunchFn()

	code, err := launch(context.Background(), fleet.CycleSpec{GoalHash: "deadbeef", Scope: []string{"m4"}})
	if err != nil || code != 0 {
		t.Fatalf("LaunchFn = (%d, %v), want (0, nil); stderr=%s", code, err, stderr.String())
	}
	if len(events) < 3 || events[0] != "trim" || events[1] != "disk" || events[2] != "wait" {
		t.Fatalf("events = %v, want [trim disk wait ...]", events)
	}
	if !strings.Contains(stdout.String(), "[m4] launched") {
		t.Fatalf("stdout = %q, want child output after resume", stdout.String())
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Fatalf("stderr = %q, want WARN line for 4%% free", stderr.String())
	}
}

func TestCycleStartTrim_SequentialDispatchRunsFakeTrimBeforeCycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	const total = uint64(100) << 30
	var events []string
	var stderr bytes.Buffer
	b := &loopBatchCoordinator{
		ctx:    context.Background(),
		cfg:    loopConfig{ProjectRoot: root, EvolveDir: evolveDir},
		orch:   cycleStartSeqRunner{events: &events},
		deps:   orchDeps{Storage: &fixtures.FakeStorage{}},
		result: &loopResult{},
		stdout: &bytes.Buffer{},
		stderr: &stderr,
		startHooks: cycleStartHooks{
			trim: func() error {
				events = append(events, "trim")
				return nil
			},
			usage: func(string) (looppreflight.DiskUsage, error) {
				events = append(events, "disk")
				return looppreflight.DiskUsage{FreeBytes: uint64(9 * float64(total) / 100), TotalBytes: total}, nil
			},
		},
	}

	_, decision := b.dispatchSequentialCycle()
	if decision.flow != batchProceed {
		t.Fatalf("decision.flow = %v, want batchProceed; stderr=%s", decision.flow, stderr.String())
	}
	if len(events) != 3 || events[0] != "trim" || events[1] != "disk" || events[2] != "cycle" {
		t.Fatalf("events = %v, want [trim disk cycle]", events)
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Fatalf("stderr = %q, want WARN at 9%% free", stderr.String())
	}
}

func TestCycleStartTrim_ProductionTrimAppliesPolicyCapAndTTL(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	stale := gcTestCacheFile(t, evolveDir, "aa/stale.json", 100, 30*24*time.Hour)
	fresh := gcTestCacheFile(t, evolveDir, "bb/fresh.json", 100, time.Hour)
	gcWritePolicy(t, root, `{"testcache_ttl_days":7}`)

	var warn bytes.Buffer
	const total = uint64(100) << 30
	opts, buildErr := buildCycleStartOptions(root, evolveDir, &warn, cycleStartHooks{
		usage: func(string) (looppreflight.DiskUsage, error) {
			return looppreflight.DiskUsage{FreeBytes: uint64(50 * float64(total) / 100), TotalBytes: total}, nil
		},
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	if err := looppreflight.RunCycleStart(context.Background(), opts); err != nil {
		t.Fatalf("RunCycleStart = %v, want nil; warn=%s", err, warn.String())
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale testcache file survived cycle start: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh testcache file was removed: %v", err)
	}
}

func TestCycleStartTrim_OptionsFallbacksAndErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := buildCycleStartOptions("", evolveDir, nil, cycleStartHooks{}); !errors.Is(err, errCycleStartPolicyRefused) {
		t.Fatalf("buildCycleStartOptions with a malformed policy = %v, want errCycleStartPolicyRefused", err)
	}
	optsFromEmpty, err := buildCycleStartOptions("", "", nil, cycleStartHooks{})
	if err != nil || optsFromEmpty.Guard == nil || optsFromEmpty.Trim == nil {
		t.Fatalf("buildCycleStartOptions with empty paths = (%+v, %v), want a full option set", optsFromEmpty, err)
	}
	optsFromRoot, err := buildCycleStartOptions(t.TempDir(), "", nil, cycleStartHooks{})
	if err != nil || optsFromRoot.Guard == nil {
		t.Fatalf("buildCycleStartOptions with a root only = (%+v, %v)", optsFromRoot, err)
	}

	unreadable := filepath.Join(evolveDir, "landing", "testcache", "locked")
	if err := os.MkdirAll(unreadable, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o755) })
	if err := cycleStartTrim(evolveDir, gc.Policy{})(); err == nil {
		t.Fatal("cycleStartTrim on unreadable testcache dir = nil, want error")
	}
}

func TestCycleStartTrim_LaunchFnWaitErrorPropagates(t *testing.T) {
	t.Parallel()
	const total = uint64(100) << 30
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	launch := cycleLaunch{
		Bin:    filepath.Join(t.TempDir(), "unused"),
		Root:   t.TempDir(),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		startHooks: cycleStartHooks{
			trim: func() error { return nil },
			usage: func(string) (looppreflight.DiskUsage, error) {
				return looppreflight.DiskUsage{FreeBytes: uint64(4 * float64(total) / 100), TotalBytes: total}, nil
			},
			wait: func(c context.Context) error { return c.Err() },
		},
	}.LaunchFn()

	code, err := launch(ctx, fleet.CycleSpec{GoalHash: "deadbeef"})
	if code != -1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("LaunchFn = (%d, %v), want (-1, context.Canceled)", code, err)
	}
}

func TestCycleStartTrim_SequentialDispatchErrors(t *testing.T) {
	t.Parallel()
	const total = uint64(100) << 30
	newCoord := func(ctx context.Context, pct float64, waitFn func(context.Context) error) *loopBatchCoordinator {
		root := t.TempDir()
		evolveDir := filepath.Join(root, ".evolve")
		if err := os.MkdirAll(evolveDir, 0o755); err != nil {
			t.Fatal(err)
		}
		var events []string
		return &loopBatchCoordinator{
			ctx:    ctx,
			cfg:    loopConfig{ProjectRoot: root, EvolveDir: evolveDir},
			orch:   cycleStartSeqRunner{events: &events},
			deps:   orchDeps{Storage: &fixtures.FakeStorage{}},
			result: &loopResult{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
			startHooks: cycleStartHooks{
				trim: func() error { return nil },
				usage: func(string) (looppreflight.DiskUsage, error) {
					return looppreflight.DiskUsage{FreeBytes: uint64(pct * float64(total) / 100), TotalBytes: total}, nil
				},
				wait: waitFn,
			},
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bCanceled := newCoord(ctx, 4, func(c context.Context) error { return c.Err() })
	if _, dec := bCanceled.dispatchSequentialCycle(); dec.flow != batchReturn || dec.exitCode != 130 {
		t.Fatalf("canceled wait dispatchSequentialCycle = %+v, want batchReturn exitCode 130", dec)
	}

	bWaitErr := newCoord(context.Background(), 4, func(context.Context) error { return errors.New("wait fail") })
	if _, dec := bWaitErr.dispatchSequentialCycle(); dec.flow != batchStopIterations {
		t.Fatalf("error wait dispatchSequentialCycle = %+v, want batchStopIterations", dec)
	}
}
