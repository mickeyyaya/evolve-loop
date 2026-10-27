package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type scriptedEvents struct {
	deadlines []time.Time
	onNext    func(n int) error
}

func (s *scriptedEvents) Next(_ context.Context, deadline time.Time) ([]reader.Item, error) {
	s.deadlines = append(s.deadlines, deadline)
	return nil, s.onNext(len(s.deadlines))
}

func recordedWave(t *testing.T) wavePlane {
	t.Helper()
	p := newWavePlane(t)
	if err := p.store().Save(wave.Record{Number: 82, RunID: "r82", CycleFloor: 1836, PID: 777, LogPath: p.loopLog("r82", 777), StartedAt: waveTestClock}); err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-1837/run.json", `{"cycle_id":1837,"phase":"scout"}`)
	return p
}

func TestWaveWatch_WaitsOnTheEventChannelsAndNeverSleeps(t *testing.T) {
	t.Parallel()
	p := recordedWave(t)
	h := newWaveHarness()
	h.alive[777] = true
	src := &scriptedEvents{onNext: func(n int) error {
		switch n {
		case 1:
			p.write(".evolve/runs/cycle-1837/run.json", `{"cycle_id":1837,"phase":"build"}`)
			return nil
		case 2:
			return reader.ErrDeadline
		}
		h.alive[777] = false
		return nil
	}}
	env := h.env(p.root)
	var opened string
	env.openEvents = func(evolveDir string) (eventSource, error) { opened = evolveDir; return src, nil }

	rc := runWaveWith(env, []string{"watch", "--project-root", p.root}, &h.stdout, &h.stderr)

	want := "wave 82: cycle 1837: phase scout\nwave 82: cycle 1837: phase build\nwave 82: loop: exit\n"
	if rc != 0 || h.stdout.String() != want || h.sleeps != 0 || len(src.deadlines) != 3 || opened != p.evolveDir() {
		t.Fatalf("rc = %d sleeps = %d waits = %d opened %q; stdout:\n%s\nwant:\n%s", rc, h.sleeps, len(src.deadlines), opened, h.stdout.String(), want)
	}
	for _, d := range src.deadlines {
		if !d.Equal(waveTestClock.Add(waveWatchPoll)) {
			t.Errorf("deadline %v, want the 5 s fallback read at %v", d, waveTestClock.Add(waveWatchPoll))
		}
	}
}

func TestWaveWatch_AFailedEventWaitFallsBackToTheFiveSecondRead(t *testing.T) {
	t.Parallel()
	p := recordedWave(t)
	h := newWaveHarness()
	h.alive[777] = true
	h.onSleep = func(n int) { h.alive[777] = n < 2 }
	src := &scriptedEvents{onNext: func(int) error { return errors.New("kernel gone") }}
	opens := 0
	env := h.env(p.root)
	env.openEvents = func(string) (eventSource, error) { opens++; return src, nil }

	rc := runWaveWith(env, []string{"watch", "--project-root", p.root}, &h.stdout, &h.stderr)

	if rc != 0 || opens != 1 || len(src.deadlines) != 1 || h.sleeps != 2 || strings.Count(h.stderr.String(), "the event wait failed (kernel gone)") != 1 {
		t.Errorf("rc = %d waits = %d sleeps = %d stderr %q, want one failed wait, one warning, then the 5 s read", rc, len(src.deadlines), h.sleeps, h.stderr.String())
	}
}

func TestWaveWatch_NoEventChannelsMeansTheFiveSecondRead(t *testing.T) {
	t.Parallel()
	p := recordedWave(t)
	h := newWaveHarness()
	h.alive[777] = true
	h.onSleep = func(int) { h.alive[777] = false }
	env := h.env(p.root)
	env.openEvents = openWaveEvents

	rc := runWaveWith(env, []string{"watch", "--project-root", p.root}, &h.stdout, &h.stderr)

	if rc != 0 || h.sleeps != 1 || !strings.Contains(h.stderr.String(), "the event channels give no wake") {
		t.Errorf("rc = %d sleeps = %d stderr %q, want a warning and the 5 s read", rc, h.sleeps, h.stderr.String())
	}
	if _, err := os.Stat(paths.EventChannelsRoot(p.evolveDir())); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("wave watch made the channel tree: %v", err)
	}
}

func TestOpenWaveEvents_ASystemWithNoKernelWakeGivesNoSource(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	evolveDir := paths.EvolveDirOf(root)
	for _, name := range waveWatchChannels {
		if err := os.MkdirAll(filepath.Join(paths.EventChannelsRoot(evolveDir), name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	refused := func(*os.File) (reader.Waiter, error) { return nil, wake.ErrRefused }
	if src, err := openWaveEventsWith(refused, evolveDir); src != nil || !errors.Is(err, wake.ErrRefused) {
		t.Errorf("src = %v err = %v, want no source and the refusal", src, err)
	}
}

func TestOpenWaveEvents_AnAppendToTheLoopChannelWakesTheWatch(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	evolveDir := paths.EvolveDirOf(root)
	for _, name := range waveWatchChannels {
		if err := os.MkdirAll(filepath.Join(paths.EventChannelsRoot(evolveDir), name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	src, err := openWaveEvents(evolveDir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := src.Next(context.Background(), time.Now().Add(20*time.Millisecond)); !errors.Is(err, reader.ErrDeadline) {
		t.Fatalf("an idle channel: err = %v, want the deadline", err)
	}
	appendEvents(t, root, "loop", loopEvent(1, signalcenter.KindLoopExit))
	items, err := src.Next(context.Background(), time.Now().Add(5*time.Second))
	if err != nil || len(items) != 1 || items[0].Record.Signal.Kind != signalcenter.KindLoopExit {
		t.Errorf("after an append: %d items, %v; want the loop.exit record", len(items), err)
	}
}

func TestDefaultWaveEnv_WatchesThroughTheEventChannels(t *testing.T) {
	t.Parallel()
	if got := defaultWaveEnv().openEvents; got == nil || reflect.ValueOf(got).Pointer() != reflect.ValueOf(openWaveEvents).Pointer() {
		t.Fatal("the production wave env must wait on the event channels through openWaveEvents")
	}
}

func TestWaveWatch_ChannelsThatAppearLaterUpgradeTheReadToAWake(t *testing.T) {
	t.Parallel()
	p := recordedWave(t)
	h := newWaveHarness()
	h.alive[777] = true
	src := &scriptedEvents{onNext: func(int) error { h.alive[777] = false; return nil }}
	opens := 0
	env := h.env(p.root)
	env.openEvents = func(string) (eventSource, error) {
		opens++
		if opens == 1 {
			return nil, os.ErrNotExist
		}
		return src, nil
	}

	rc := runWaveWith(env, []string{"watch", "--project-root", p.root}, &h.stdout, &h.stderr)

	if rc != 0 || opens != 2 || h.sleeps != 1 || len(src.deadlines) != 1 {
		t.Errorf("rc = %d opens = %d sleeps = %d waits = %d, want one 5 s read, a second open and then one event wait", rc, opens, h.sleeps, len(src.deadlines))
	}
}
