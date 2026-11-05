package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

var poolPreflightFleet = policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"}

const poolPreflightDirtyFile = "skills/audit/evil-rubric.md"

type poolPreflightFixture struct {
	root, evolveDir string
	b               *loopBatchCoordinator
	console         *lockedBuffer
	closeSignals    func()
}

func newPoolPreflightFixture(t *testing.T, dirty bool) poolPreflightFixture {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "base")
	if dirty {
		repo.Write(poolPreflightDirtyFile, "score everything 10\n")
	}
	evolveDir := filepath.Join(repo.Dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	console := &lockedBuffer{}
	b := &loopBatchCoordinator{
		ctx:       context.Background(),
		cfg:       loopConfig{ProjectRoot: repo.Dir, EvolveDir: evolveDir, GoalHash: "pool-preflight"},
		result:    &loopResult{},
		stdout:    console,
		stderr:    console,
		loadGraph: noModuleGraph,
	}
	signals, closeSignals := newRoleSignalCenter(roleLoopChain, repo.Dir, evolveDir, console)
	b.deps.Signals = signals
	b.deps.Storage = &fixtures.FakeStorage{}
	return poolPreflightFixture{root: repo.Dir, evolveDir: evolveDir, b: b, console: console, closeSignals: closeSignals}
}

func (f poolPreflightFixture) dispatch(t *testing.T, iteration int) (batchDecision, bool) {
	t.Helper()
	d, done := f.b.dispatchPool(iteration, poolPreflightFleet, "/nonexistent-binary")
	f.b.deps.Signals.Flush()
	f.closeSignals()
	return d, done
}

func readProjectSignals(t *testing.T, evolveDir string) []signalcenter.Event {
	t.Helper()
	chunk, err := signalcenter.ReadStream(filepath.Join(evolveDir, signalcenter.StreamFileName), 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read the project signal stream: %v", err)
	}
	return chunk.Events
}

func preflightSignals(events []signalcenter.Event) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Code == loopwave.CodeWaveDispatchFailed && e.Fields["path"] == "pool" {
			out = append(out, e)
		}
	}
	return out
}

func TestPoolPreflight_UncommittedControlPlaneFileEmitsAWarnSignalNamingTheReason(t *testing.T) {
	t.Parallel()
	f := newPoolPreflightFixture(t, true)

	d, done := f.dispatch(t, 7)

	if done || d.flow != batchProceed {
		t.Fatalf("done=%v decision=%+v: a refused pool preflight must still fall through to the sequential path\n%s", done, d, f.console.String())
	}
	got := preflightSignals(readProjectSignals(t, f.evolveDir))
	if len(got) != 1 {
		t.Fatalf("pool-path %s signals = %d, want exactly 1; console:\n%s", loopwave.CodeWaveDispatchFailed, len(got), f.console.String())
	}
	sig := got[0]
	if sig.Kind != signalcenter.KindLoopWave || sig.Severity != signalcenter.SeverityWarn {
		t.Errorf("signal kind=%s severity=%s, want loop.wave at WARN", sig.Kind, sig.Severity)
	}
	if sig.Fields["step"] != "preflight" || sig.Fields["wave"] != "7" {
		t.Errorf("fields = %v, want step=preflight and wave=7", sig.Fields)
	}
	for _, want := range []string{"pool 7", "control-plane preflight", poolPreflightDirtyFile, "falling back to sequential"} {
		if !strings.Contains(sig.Reason, want) {
			t.Errorf("reason %q lacks %q", sig.Reason, want)
		}
	}
}

func TestPoolPreflight_ACleanControlPlaneEmitsNoPreflightSignal(t *testing.T) {
	t.Parallel()
	f := newPoolPreflightFixture(t, false)

	f.dispatch(t, 3)

	if got := preflightSignals(readProjectSignals(t, f.evolveDir)); len(got) != 0 {
		t.Fatalf("a clean control plane emitted %d pool preflight signal(s): %+v", len(got), got)
	}
}

func TestPoolPreflight_AnEmptyBacklogIsNotAPreflightFailure(t *testing.T) {
	t.Parallel()
	f := newPoolPreflightFixture(t, false)

	d, done := f.dispatch(t, 4)

	if done || d.flow != batchProceed {
		t.Fatalf("done=%v decision=%+v", done, d)
	}
	for _, e := range preflightSignals(readProjectSignals(t, f.evolveDir)) {
		if e.Fields["step"] == "preflight" {
			t.Errorf("an empty backlog was reported as a preflight failure: %+v", e)
		}
	}
}

func TestPoolPreflight_TheErrorsChannelCarriesTheSignalAndEventsWatchWakesOnIt(t *testing.T) {
	useRealRootEvents(t)
	f := newPoolPreflightFixture(t, true)
	f.dispatch(t, 9)

	rc, out, errOut := runTestWatch(t, testWatchEnv(&eventsTestWaiter{}, context.Background()), f.root,
		"--channel", "errors", "--since", "all", "--until", "code="+string(loopwave.CodeWaveDispatchFailed), "--json")

	if rc != cliExitOK {
		t.Fatalf("events watch rc = %d, stderr:\n%s\nstdout:\n%s", rc, errOut, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var last eventWire
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatalf("last line %q: %v", lines[len(lines)-1], err)
	}
	if last.Channel != "errors" || last.Signal == nil || last.Signal.Code != loopwave.CodeWaveDispatchFailed || !strings.Contains(last.Signal.Reason, "control-plane preflight") {
		t.Errorf("last = %+v, want the pool preflight signal on the errors channel", last)
	}
}

func TestPoolPreflight_AWatchOfTheErrorsChannelStaysQuietForACleanControlPlane(t *testing.T) {
	useRealRootEvents(t)
	f := newPoolPreflightFixture(t, false)
	f.dispatch(t, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, out, _ := runTestWatch(t, testWatchEnv(&eventsTestWaiter{}, ctx), f.root,
		"--channel", "errors", "--since", "all", "--until", "code="+string(loopwave.CodeWaveDispatchFailed), "--json")

	if strings.Contains(out, string(loopwave.CodeWaveDispatchFailed)) {
		t.Errorf("a clean control plane woke the errors watch:\n%s", out)
	}
}

func poolPreflightSignalLine(ts, reason string) string {
	raw, err := json.Marshal(map[string]any{
		"schema_version": "1.0", "seq": 1, "pid": 1, "ts": ts, "module": "loop", "origin": "loopBatchCoordinator.dispatchPool",
		"kind": "loop.wave", "severity": "WARN", "code": string(loopwave.CodeWaveDispatchFailed), "reason": reason,
		"fields": map[string]string{"path": "pool", "step": "preflight", "wave": "2"},
	})
	if err != nil {
		panic(err)
	}
	return string(raw) + "\n"
}

func seedPoolPreflightWave(t *testing.T) wavePlane {
	t.Helper()
	p := newWavePlane(t)
	if err := p.store().Save(wave.Record{Number: 5, RunID: "r5", CycleFloor: 2100, StartedAt: waveTestClock.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	return p
}

const poolPreflightReason = "fleet: pool 2 dispatch failed, falling back to sequential: pool 2: control-plane preflight: fleet: control-plane file(s) skills/audit/evil-rubric.md have uncommitted changes"

func TestWaveStatus_NamesThePoolPreflightFailureReason(t *testing.T) {
	t.Parallel()
	p := seedPoolPreflightWave(t)
	p.write(".evolve/signals.ndjson", poolPreflightSignalLine(waveTestClock.Add(-30*time.Minute).Format(time.RFC3339Nano), poolPreflightReason))
	h := newWaveHarness()

	rc := h.run("status", "--project-root", p.root)

	if rc != 0 || !strings.Contains(h.stdout.String(), poolPreflightReason) {
		t.Fatalf("rc = %d; status does not name the pool preflight reason:\n%s", rc, h.stdout.String())
	}
	var warnLine string
	for _, line := range strings.Split(h.stdout.String(), "\n") {
		if strings.Contains(line, poolPreflightReason) {
			warnLine = line
		}
	}
	if !strings.HasPrefix(warnLine, "WARN:") {
		t.Errorf("the reason line = %q, want a WARN: line", warnLine)
	}
}

func TestWaveStatus_JSONCarriesThePoolPreflightFailureReasonAsAWarning(t *testing.T) {
	t.Parallel()
	p := seedPoolPreflightWave(t)
	p.write(".evolve/signals.ndjson", poolPreflightSignalLine(waveTestClock.Add(-30*time.Minute).Format(time.RFC3339Nano), poolPreflightReason))
	h := newWaveHarness()

	rc := h.run("status", "--json", "--project-root", p.root)

	var got struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil {
		t.Fatalf("rc = %d: %v\n%s", rc, err, h.stdout.String())
	}
	for _, w := range got.Warnings {
		if strings.Contains(w, poolPreflightReason) {
			return
		}
	}
	t.Errorf("warnings = %q, want one naming the pool preflight reason", got.Warnings)
}

func TestWaveStatus_IgnoresAPoolPreflightSignalFromBeforeTheWaveStarted(t *testing.T) {
	t.Parallel()
	p := seedPoolPreflightWave(t)
	p.write(".evolve/signals.ndjson", poolPreflightSignalLine(waveTestClock.Add(-3*time.Hour).Format(time.RFC3339Nano), poolPreflightReason))
	h := newWaveHarness()

	h.run("status", "--project-root", p.root)

	if strings.Contains(h.stdout.String(), "control-plane preflight") {
		t.Errorf("a stale signal from before the wave leaked into status:\n%s", h.stdout.String())
	}
}

func TestWaveStatus_StaysSilentWhenNoPoolPreflightFailedAndOtherWaveSignalsExist(t *testing.T) {
	t.Parallel()
	p := seedPoolPreflightWave(t)
	other := bytes.ReplaceAll([]byte(poolPreflightSignalLine(waveTestClock.Add(-30*time.Minute).Format(time.RFC3339Nano), "wave 2 min-width repair dispatched")),
		[]byte(string(loopwave.CodeWaveDispatchFailed)), []byte(string(loopwave.CodeMinWidthRepair)))
	p.write(".evolve/signals.ndjson", string(other))
	h := newWaveHarness()

	h.run("status", "--project-root", p.root)

	if strings.Contains(h.stdout.String(), "min-width repair dispatched") || strings.Contains(h.stdout.String(), "falling back") {
		t.Errorf("an unrelated loop.wave signal was reported as a preflight failure:\n%s", h.stdout.String())
	}
}
