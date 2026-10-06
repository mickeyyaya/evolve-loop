package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRunTmuxREPL_PublishesThePaneWatchSnapshotWhileWaiting(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	artifact := filepath.Join(ws, "a")
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws, Agent: "router", Cycle: 7, RunID: "run-1",
		Artifact: artifact, StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	deps := covDeps()
	ticks := 0
	var snap panewatch.Snapshot
	var ok bool
	var err error
	deps.Sleep = func(d time.Duration) {
		if d != artifactWaitInterval {
			return
		}
		ticks++
		if ticks == 2 {
			snap, ok, err = panewatch.Read(ws, "router")
			_ = os.WriteFile(artifact, []byte("done"), 0o644)
		}
	}
	deps.Tmux = &fakeTmux{paneSeq: []string{"● working\n❯"}}
	lp := tmuxLaunch{name: "claude-tmux", session: "evolve-bridge-c7-router-pid1-n1-1", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}

	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}
	if err != nil || !ok {
		t.Fatalf("no pane watch snapshot after a waited tick: ok=%v err=%v", ok, err)
	}
	if snap.Session != lp.session || snap.CLI != "claude-tmux" || snap.Cycle != 7 || snap.RunID != "run-1" {
		t.Errorf("snapshot identity = %+v", snap)
	}
	if snap.ProgressHash == "" || snap.ProgressAt.IsZero() || snap.Socket == "" {
		t.Errorf("snapshot carries no progress evidence: %+v", snap)
	}
}

func TestPaneWatcher_AgySpinnerTickIsBusyWithoutProgress(t *testing.T) {
	ws := t.TempDir()
	lp := agyLaunchForTest()
	w := newPaneWatcher(&Config{Workspace: ws, Agent: "build", Cycle: 3}, lp, paneProfileFor(lp), &bytes.Buffer{}, "[agy-tmux]")
	t0 := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

	w.observe(agy1217Frame(t, "editing-a.txt"), t0)
	w.observe(agy1217Frame(t, "editing-b.txt"), t0.Add(time.Minute))
	snap, ok, err := panewatch.Read(ws, "build")
	if err != nil || !ok {
		t.Fatalf("read: ok=%v err=%v", ok, err)
	}
	if !snap.Busy || snap.ProgressAt != t0 || snap.UpdatedAt != t0 {
		t.Errorf("after a spinner tick: busy=%v progress_at=%v updated_at=%v, want busy with no progress since %v",
			snap.Busy, snap.ProgressAt, snap.UpdatedAt, t0)
	}

	answerAt := t0.Add(2 * time.Minute)
	w.observe(agy1217Frame(t, "answer.txt"), answerAt)
	snap, _, _ = panewatch.Read(ws, "build")
	if snap.Busy || snap.ProgressAt != answerAt {
		t.Errorf("after the answer: busy=%v progress_at=%v, want idle with progress at %v", snap.Busy, snap.ProgressAt, answerAt)
	}
}

func TestPaneWatcher_WriteFailureWarnsOnceAndKeepsWaiting(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	lp := agyLaunchForTest()
	w := newPaneWatcher(&Config{Workspace: file, Agent: "build"}, lp, paneProfileFor(lp), &stderr, "[agy-tmux]")
	w.observe(agy1217Frame(t, "editing-a.txt"), time.Now())
	w.observe(agy1217Frame(t, "answer.txt"), time.Now())
	if n := bytes.Count(stderr.Bytes(), []byte("WARN pane watch")); n != 1 {
		t.Errorf("write failures logged %d times, want exactly once:\n%s", n, stderr.String())
	}
}

func TestPaneWatcher_NoWorkspaceOrBlankPaneIsInert(t *testing.T) {
	lp := agyLaunchForTest()
	if w := newPaneWatcher(&Config{Agent: "build"}, lp, paneProfileFor(lp), &bytes.Buffer{}, "x"); w != nil {
		t.Error("a launch with no workspace has nowhere to publish; the watcher must be nil")
	}
	var nilWatcher *paneWatcher
	nilWatcher.observe("anything", time.Now())
	ws := t.TempDir()
	w := newPaneWatcher(&Config{Workspace: ws, Agent: "build"}, lp, paneProfileFor(lp), &bytes.Buffer{}, "x")
	w.observe("   \n", time.Now())
	if _, ok, _ := panewatch.Read(ws, "build"); ok {
		t.Error("a blank capture is no observation and must not be published")
	}
}

func TestRunTmuxREPL_RemovesThePaneWatchSnapshotWhenTheWaitEnds(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	artifact := filepath.Join(ws, "a")
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws, Agent: "build",
		Artifact: artifact, StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	deps := covDeps()
	published := false
	deps.Sleep = func(d time.Duration) {
		if d != artifactWaitInterval {
			return
		}
		if _, ok, _ := panewatch.Read(ws, "build"); ok && !published {
			published = true
			_ = os.WriteFile(artifact, []byte("done"), 0o644)
		}
	}
	deps.Tmux = &fakeTmux{paneSeq: []string{"● working\n❯"}}
	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}

	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}
	if !published {
		t.Fatal("precondition: the snapshot was never published during the wait")
	}
	if _, ok, _ := panewatch.Read(ws, "build"); ok {
		t.Error("the snapshot outlived its session; a stale snapshot would keep a later attempt's observer on a dead pane")
	}
}

func TestPaneWatcher_AgySnapshotCarriesTheTokenLineAndFooterModel(t *testing.T) {
	ws := t.TempDir()
	lp := agyLaunchForTest()
	w := newPaneWatcher(&Config{Workspace: ws, Agent: "build"}, lp, paneProfileFor(lp), &bytes.Buffer{}, "[agy-tmux]")
	w.observe(agy1217Frame(t, "answer.txt"), time.Now())
	snap, _, err := panewatch.Read(ws, "build")
	if err != nil {
		t.Fatal(err)
	}
	if snap.TokenLine != "▸ Thought for 14s, 1.5k tokens" || snap.ModelLabel != "Gemini 3.8 Flash · high" {
		t.Errorf("snapshot token line %q and model label %q, want the pane's thought line and footer model", snap.TokenLine, snap.ModelLabel)
	}
}

func TestCodePhaseStalled_IsRegisteredUnderTheLivenessModule(t *testing.T) {
	module, ok := signalcenter.IsRegistered(CodePhaseStalled)
	if !ok || module != signalcenter.ModuleLiveness {
		t.Errorf("CodePhaseStalled registered=%v module=%q, want the liveness module beside the other LIVENESS_ codes", ok, module)
	}
	if CodePhaseStalled != "LIVENESS_PHASE_STALLED" {
		t.Errorf("CodePhaseStalled = %q; the observer, the docs and signal-codes.md name LIVENESS_PHASE_STALLED", CodePhaseStalled)
	}
}
