package observer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func busySnapshot(progressAt time.Time) panewatch.Snapshot {
	return panewatch.Snapshot{Session: "evolve-bridge-agy-c9-router-pid1-n1-1", CLI: "agy-tmux", Agent: "router", Cycle: 9,
		Busy: true, ProgressHash: "h", ProgressAt: progressAt, ModelLabel: "Gemini 3.1 Pro · high", WriterPID: os.Getpid()}
}

func watchFor(t *testing.T, cfg Config, d time.Duration) []Event {
	t.Helper()
	var sink bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	if err := New(cfg, &sink).Watch(ctx); err != nil && err != context.DeadlineExceeded {
		t.Fatalf("Watch: %v", err)
	}
	return parseEvents(t, sink.Bytes())
}

func TestWatch_FrozenPaneStallsAsNoProgressEvenWhileTheWorkspaceChurns(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	frozen := time.Now()
	writes := 0
	cfg := Config{
		StallS: 60 * time.Millisecond, PollS: 5 * time.Millisecond, Cycle: 9, Phase: "router", Agent: "router",
		StdoutLog: filepath.Join(ws, "router-stdout.log"), WorkspaceDir: ws,
		PaneWatch: func() (panewatch.Snapshot, bool, error) {
			writes++
			_ = os.WriteFile(filepath.Join(ws, "signals.ndjson"), []byte(strconv.Itoa(writes)), 0o644)
			return busySnapshot(frozen), true, nil
		},
	}
	var stall *Event
	for _, e := range watchFor(t, cfg, 400*time.Millisecond) {
		if e.Type == eventStallNoOutput {
			t.Errorf("a pane phase must not fall back to the stdout rule: %+v", e)
		}
		if e.Type == eventStallNoProgress {
			e := e
			stall = &e
		}
	}
	if stall == nil {
		t.Fatal("a frozen pane never produced stall_no_progress; host writes to the workspace masked the stall")
	}
	if stall.Severity != "incident" || stall.Source != "pane" || stall.Session != busySnapshot(frozen).Session || !stall.Busy {
		t.Errorf("stall event = %+v, want an incident naming the pane session and its busy state", *stall)
	}
	if !strings.Contains(stall.Reason, "no new transcript") || !strings.Contains(stall.Reason, stall.Session) {
		t.Errorf("stall reason %q must say what was watched", stall.Reason)
	}
}

func TestWatch_PaneProgressResetsTheStallClock(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	at := time.Now()
	cfg := Config{
		StallS: 60 * time.Millisecond, PollS: 5 * time.Millisecond, Phase: "build",
		PaneWatch: func() (panewatch.Snapshot, bool, error) {
			mu.Lock()
			defer mu.Unlock()
			at = at.Add(time.Second)
			return busySnapshot(at), true, nil
		},
	}
	for _, e := range watchFor(t, cfg, 300*time.Millisecond) {
		if strings.HasPrefix(e.Type, "stall_") {
			t.Errorf("a pane whose transcript keeps growing stalled: %+v", e)
		}
	}
}

func TestWatch_StdoutStallReasonNamesWhatWasWatched(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := Config{StallS: 40 * time.Millisecond, PollS: 5 * time.Millisecond, Phase: "scout",
		StdoutLog: filepath.Join(ws, "scout-stdout.log"), WorkspaceDir: ws,
		PaneWatch: func() (panewatch.Snapshot, bool, error) { return panewatch.Snapshot{}, false, nil }}
	for _, e := range watchFor(t, cfg, 300*time.Millisecond) {
		if e.Type != eventStallNoOutput {
			continue
		}
		if e.Source != "stdout" || !strings.Contains(e.Reason, "stdout") || !strings.Contains(e.Reason, "workspace") {
			t.Errorf("stdout stall = %+v, want source stdout and a reason naming the stdout log and the workspace", e)
		}
		return
	}
	t.Error("no stall_no_output for an idle headless phase")
}

func TestCoreAdapter_PaneStallIsALivenessSignal(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := panewatch.Write(ws, busySnapshot(time.Now().Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	rec := newRecordingCenter()
	a := &CoreAdapter{Sink: &syncSink{}, Config: fastObserverPolicy(), Signals: func() *signalcenter.Center { return rec.c }}
	cancel := a.Start(context.Background(), "router", core.PhaseRequest{Workspace: ws, Cycle: 9, RunID: "run-9"})
	defer cancel()

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range rec.all() {
			if e.Code != "LIVENESS_PHASE_STALLED" {
				continue
			}
			if e.Kind != signalcenter.KindPaneLiveness || e.Module != signalcenter.ModuleLiveness || e.Phase != "router" || e.Cycle != 9 || e.RunID != "run-9" {
				t.Errorf("stall signal envelope = %+v", e)
			}
			if e.Fields["source"] != "pane" || e.Fields["session"] != busySnapshot(time.Time{}).Session || e.Fields["busy"] != "true" {
				t.Errorf("stall signal fields = %v", e.Fields)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no LIVENESS_PHASE_STALLED signal for a router pane with no progress; got %+v", rec.all())
}

func TestCoreAdapter_StdoutStallIsAnObserverWarningSignal(t *testing.T) {
	t.Parallel()
	rec := newRecordingCenter()
	a := &CoreAdapter{Sink: &syncSink{}, Config: fastObserverPolicy(), Signals: func() *signalcenter.Center { return rec.c }}
	cancel := a.Start(context.Background(), "scout", core.PhaseRequest{Workspace: t.TempDir(), Cycle: 4})
	defer cancel()

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range rec.all() {
			if e.Code == "LIVENESS_PHASE_STALLED" {
				if e.Kind != signalcenter.KindObserverWarning || e.Fields["source"] != "stdout" {
					t.Errorf("stdout stall signal = %+v", e)
				}
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no LIVENESS_PHASE_STALLED signal for an idle headless phase; got %+v", rec.all())
}

func TestWatch_UnreadablePaneSnapshotIsReportedOnceAndFallsBack(t *testing.T) {
	t.Parallel()
	cfg := Config{StallS: 40 * time.Millisecond, PollS: 5 * time.Millisecond, Phase: "build",
		PaneWatch: func() (panewatch.Snapshot, bool, error) {
			return panewatch.Snapshot{}, false, errors.New("pane watch build: unexpected end of JSON input")
		}}
	unreadable, fellBack := 0, false
	for _, e := range watchFor(t, cfg, 300*time.Millisecond) {
		switch e.Type {
		case "pane_watch_unreadable":
			unreadable++
		case eventStallNoOutput:
			fellBack = true
		}
	}
	if unreadable != 1 || !fellBack {
		t.Errorf("unreadable snapshot reported %d times (want 1), stdout fallback stall=%v (want true)", unreadable, fellBack)
	}
}

func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func TestCoreAdapter_ADeadWritersSnapshotFallsBackToStdout(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	stale := busySnapshot(time.Now().Add(-time.Hour))
	stale.WriterPID = deadPID(t)
	if err := panewatch.Write(ws, stale); err != nil {
		t.Fatal(err)
	}
	sink := &syncSink{}
	a := &CoreAdapter{Sink: sink, Config: fastObserverPolicy()}
	cancel := a.Start(context.Background(), "router", core.PhaseRequest{Workspace: ws, Cycle: 9})
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(sink.String(), `"stall_`) {
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	out := sink.String()
	if strings.Contains(out, `"stall_no_progress"`) || !strings.Contains(out, `"stall_no_output"`) {
		t.Errorf("a snapshot whose writer is dead must read as absent (stdout rule), got:\n%s", out)
	}
}
