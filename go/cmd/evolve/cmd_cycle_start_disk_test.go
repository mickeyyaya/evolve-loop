package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func writeDiskGuardPolicy(t *testing.T, root string, seconds int) {
	t.Helper()
	p := filepath.Join(root, ".evolve", "policy.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"preflight":{"disk_guard":{"max_pause_s":` + strconv.Itoa(seconds) + `}}}`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustBuildCycleStartOptions(t *testing.T, root, evolveDir string, hooks cycleStartHooks) looppreflight.CycleStartOptions {
	t.Helper()
	opts, err := buildCycleStartOptions(root, evolveDir, nil, hooks)
	if err != nil {
		t.Fatal(err)
	}
	return opts
}

func lowDisk(string) (looppreflight.DiskUsage, error) {
	return looppreflight.DiskUsage{FreeBytes: 2 << 30, TotalBytes: 100 << 30}, nil
}

func TestCmdEvolveTestsNeverReadTheHostDisk(t *testing.T) {
	t.Parallel()
	if reflect.ValueOf(cycleStartDiskUsageFn).Pointer() == reflect.ValueOf(looppreflight.DiskUsageOf).Pointer() {
		t.Fatal("cycleStartDiskUsageFn is the real DiskUsageOf: a cmd/evolve test would read the host disk and hang when it is full")
	}
}

func TestBuildCycleStartOptions_UsesTheSeamWhenNoHookIsSet(t *testing.T) {
	prev := cycleStartDiskUsageFn
	cycleStartDiskUsageFn = lowDisk
	defer func() { cycleStartDiskUsageFn = prev }()
	opts := mustBuildCycleStartOptions(t, t.TempDir(), "", cycleStartHooks{})
	if st := opts.Guard.Measure(); !st.Paused || st.FreePct != 2 {
		t.Fatalf("guard status = %+v, want paused at 2%% from the seam", st)
	}
}

func TestBuildCycleStartOptions_HookWinsOverTheSeam(t *testing.T) {
	prev := cycleStartDiskUsageFn
	cycleStartDiskUsageFn = lowDisk
	defer func() { cycleStartDiskUsageFn = prev }()
	hook := func(string) (looppreflight.DiskUsage, error) {
		return looppreflight.DiskUsage{FreeBytes: 50 << 30, TotalBytes: 100 << 30}, nil
	}
	opts := mustBuildCycleStartOptions(t, t.TempDir(), "", cycleStartHooks{usage: hook})
	if st := opts.Guard.Measure(); st.Paused {
		t.Fatalf("guard status = %+v, want the hook (50%% free) to win", st)
	}
}

func TestBuildCycleStartOptions_MaxPauseComesFromPolicy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDiskGuardPolicy(t, root, 7)
	opts := mustBuildCycleStartOptions(t, root, filepath.Join(root, ".evolve"), cycleStartHooks{})
	if opts.MaxPause != 7*time.Second {
		t.Fatalf("MaxPause = %v, want 7s from preflight.disk_guard.max_pause_s", opts.MaxPause)
	}
	def := mustBuildCycleStartOptions(t, t.TempDir(), "", cycleStartHooks{})
	if def.MaxPause != 30*time.Minute {
		t.Fatalf("MaxPause without a policy = %v, want 30m", def.MaxPause)
	}
}

func TestEmitHostDiskLow_EmitsTheRegisteredWarnWithItsFields(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	emit := emitHostDiskLow(center)
	emit(looppreflight.DiskPauseEvent{FreePct: 2.04, PausePct: 5, Path: "/vol/x", Outcome: looppreflight.DiskPauseOutcomePaused})
	emit(looppreflight.DiskPauseEvent{FreePct: 2.04, PausePct: 5, Path: "/vol/x", Outcome: looppreflight.DiskPauseOutcomeGaveUp})
	got := center.Recent()
	if len(got) != 2 {
		t.Fatalf("recent events = %d, want 2", len(got))
	}
	for i, outcome := range []string{"paused", "gave_up"} {
		e := got[i]
		if e.Code != signalcenter.CodeHostDiskLow || e.Severity != signalcenter.SeverityWarn || e.Module != signalcenter.ModuleHost {
			t.Fatalf("event %d = %+v, want a HOST_DISK_LOW WARN of module host", i, e)
		}
		if wantVerb := []string{"waits", "stopped"}[i]; !strings.Contains(e.Reason, wantVerb) {
			t.Fatalf("event %d reason = %q, want it to hold %q", i, e.Reason, wantVerb)
		}
		want := map[string]string{"free_pct": "2.0", "pause_pct": "5.0", "path": "/vol/x", "outcome": outcome}
		if !reflect.DeepEqual(e.Fields, want) {
			t.Fatalf("event %d fields = %v, want %v", i, e.Fields, want)
		}
	}
	emitHostDiskLow(nil)(looppreflight.DiskPauseEvent{})
}

func TestDispatchSequentialCycle_DiskPauseBoundStopsTheBatchWithTheDiskCause(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	center := signalcenter.New()
	b := &loopBatchCoordinator{
		ctx:    context.Background(),
		cfg:    loopConfig{ProjectRoot: t.TempDir()},
		orch:   cycleStartSeqRunner{events: new([]string)},
		deps:   orchDeps{Storage: &fixtures.FakeStorage{}, Signals: center},
		result: &loopResult{},
		stdout: &bytes.Buffer{},
		stderr: &stderr,
		startHooks: cycleStartHooks{
			usage: lowDisk,
			wait:  func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() },
		},
	}
	writeDiskGuardPolicy(t, b.cfg.ProjectRoot, 1)
	b.cfg.EvolveDir = filepath.Join(b.cfg.ProjectRoot, ".evolve")

	_, decision := b.dispatchSequentialCycle()
	if decision.flow != batchStopIterations {
		t.Fatalf("decision.flow = %v, want batchStopIterations", decision.flow)
	}
	if b.result.StopReason != hostDiskLowStopReason {
		t.Fatalf("StopReason = %q, want %q", b.result.StopReason, hostDiskLowStopReason)
	}
	var outcomes []string
	for _, e := range center.Recent() {
		if e.Code == signalcenter.CodeHostDiskLow {
			outcomes = append(outcomes, e.Fields["outcome"])
		}
	}
	if !reflect.DeepEqual(outcomes, []string{"paused", "gave_up"}) {
		t.Fatalf("HOST_DISK_LOW outcomes = %v, want [paused gave_up]", outcomes)
	}
}

func TestLoop_MaxCyclesExit_ClearsCompletedMarker_WhenHostDiskIsLow(t *testing.T) {
	prev := cycleStartDiskUsageFn
	cycleStartDiskUsageFn = lowDisk
	defer func() { cycleStartDiskUsageFn = prev }()

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"dispatch":{"policy":"off"},"preflight":{"disk_guard":{"max_pause_s":1}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	defer installStubDeps(t, &fixtures.FakeStorage{}, newFakeLedger())()

	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runLoop([]string{
			"--project-root", projectRoot,
			"--evolve-dir", evolveDir,
			"--goal-text", "finalize goal",
			"--cycles", "1",
		}, nil, &stdout, &stderr)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("runLoop did not return within 30s on a 2%% free disk; stderr=%s", stderr.String())
	}
	var out struct {
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("loop outcome is not JSON: %v; stdout=%s", err, stdout.String())
	}
	if out.StopReason != hostDiskLowStopReason {
		t.Fatalf("stop_reason = %q, want %q; stderr=%s", out.StopReason, hostDiskLowStopReason, stderr.String())
	}
	stream, err := os.ReadFile(filepath.Join(evolveDir, signalcenter.StreamFileName))
	if err != nil || !strings.Contains(string(stream), string(signalcenter.CodeHostDiskLow)) {
		t.Fatalf("project signal stream holds no HOST_DISK_LOW (err=%v): runLoop did not wire the Center into the cycle start", err)
	}
	if !strings.Contains(stderr.String(), "cycle not started") {
		t.Fatalf("stderr does not name the unstarted cycle: %s", stderr.String())
	}
}

func TestCycleLaunch_DiskPauseBoundReturnsTheTypedError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDiskGuardPolicy(t, root, 1)
	var stderr bytes.Buffer
	launch := cycleLaunch{
		Bin: "unused", Root: root, Stdout: &bytes.Buffer{}, Stderr: &stderr,
		Signals:    signalcenter.New(),
		startHooks: cycleStartHooks{usage: lowDisk, wait: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }},
	}
	code, err := launch.LaunchFn()(context.Background(), fleet.CycleSpec{GoalHash: "deadbeef"})
	if code != -1 || !errors.Is(err, looppreflight.ErrDiskPauseExceeded) {
		t.Fatalf("LaunchFn = (%d, %v), want (-1, ErrDiskPauseExceeded)", code, err)
	}
	if got := hostDiskLowOutcomes(launch.Signals); !reflect.DeepEqual(got, []string{"paused", "gave_up"}) {
		t.Fatalf("HOST_DISK_LOW outcomes on the lane Center = %v, want [paused gave_up]", got)
	}
}

func hostDiskLowOutcomes(c *signalcenter.Center) []string {
	var out []string
	for _, e := range c.Recent() {
		if e.Code == signalcenter.CodeHostDiskLow {
			out = append(out, e.Fields["outcome"])
		}
	}
	return out
}

func TestPoolLane_LowDiskEmitsHostDiskLowOnTheCoordinatorCenter(t *testing.T) {
	prev := cycleStartDiskUsageFn
	cycleStartDiskUsageFn = lowDisk
	defer func() { cycleStartDiskUsageFn = prev }()
	root := t.TempDir()
	writeDiskGuardPolicy(t, root, 1)
	center := signalcenter.New()
	b := &loopBatchCoordinator{
		cfg:    loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")},
		deps:   orchDeps{Signals: center},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
	code, err := b.cycleLaunch("unused", 9).LaunchFn()(context.Background(), fleet.CycleSpec{GoalHash: "deadbeef"})
	if code != -1 || !errors.Is(err, looppreflight.ErrDiskPauseExceeded) {
		t.Fatalf("pool lane LaunchFn = (%d, %v), want (-1, ErrDiskPauseExceeded)", code, err)
	}
	if got := hostDiskLowOutcomes(center); !reflect.DeepEqual(got, []string{"paused", "gave_up"}) {
		t.Fatalf("HOST_DISK_LOW outcomes on the coordinator Center = %v, want [paused gave_up]", got)
	}
}

func TestBuildCycleStartOptions_AnInvalidMaxPauseIsRefusedNotDefaulted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDiskGuardPolicy(t, root, 0)
	_, err := buildCycleStartOptions(root, filepath.Join(root, ".evolve"), nil, cycleStartHooks{})
	if !errors.Is(err, errCycleStartPolicyRefused) {
		t.Fatalf("buildCycleStartOptions with max_pause_s 0 = %v, want errCycleStartPolicyRefused", err)
	}
}

func TestCycleLaunch_AnInvalidPolicyRefusesTheLaneStart(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDiskGuardPolicy(t, root, -3)
	launch := cycleLaunch{Bin: "unused", Root: root, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	code, err := launch.LaunchFn()(context.Background(), fleet.CycleSpec{GoalHash: "deadbeef"})
	if code != -1 || !errors.Is(err, errCycleStartPolicyRefused) {
		t.Fatalf("LaunchFn = (%d, %v), want (-1, errCycleStartPolicyRefused)", code, err)
	}
}

func TestDispatchSequentialCycle_AnInvalidPolicyStopsTheBatchLoudly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDiskGuardPolicy(t, root, 0)
	var stderr bytes.Buffer
	b := &loopBatchCoordinator{
		ctx:    context.Background(),
		cfg:    loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")},
		orch:   cycleStartSeqRunner{events: new([]string)},
		deps:   orchDeps{Storage: &fixtures.FakeStorage{}},
		result: &loopResult{},
		stdout: &bytes.Buffer{},
		stderr: &stderr,
	}
	_, decision := b.dispatchSequentialCycle()
	if decision.flow != batchStopIterations || b.result.StopReason != "error" {
		t.Fatalf("decision = %v, StopReason = %q, want batchStopIterations and error", decision.flow, b.result.StopReason)
	}
}
