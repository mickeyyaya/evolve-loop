package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
)

func publishedSnapshotFor(t *testing.T, pane string) panewatch.Snapshot {
	t.Helper()
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	artifact := filepath.Join(ws, "a")
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws, Agent: "build", Cycle: 7, RunID: "run-1",
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
			snap, ok, err = panewatch.Read(ws, "build")
			_ = os.WriteFile(artifact, []byte("done"), 0o644)
		}
	}
	deps.Tmux = &fakeTmux{paneSeq: []string{pane}}
	lp := tmuxLaunch{name: "claude-tmux", session: "evolve-bridge-c7-build-pid1-n1-1", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}
	if err != nil || !ok {
		t.Fatalf("no pane watch snapshot after a waited tick: ok=%v err=%v", ok, err)
	}
	return snap
}

func TestRunTmuxREPL_PublishesTheRunningShellCountFromTheFooter(t *testing.T) {
	t.Parallel()
	snap := publishedSnapshotFor(t, "● waiting on a build\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 2 shells · ↓ to manage")
	if snap.Shells != 2 {
		t.Errorf("snapshot Shells = %d, want 2 for a footer that shows 2 shells: %+v", snap.Shells, snap)
	}
}

func TestRunTmuxREPL_PublishesNoShellsWhenTheFooterShowsNone(t *testing.T) {
	t.Parallel()
	snap := publishedSnapshotFor(t, "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · ← for agents")
	if snap.Shells != 0 {
		t.Errorf("snapshot Shells = %d, want 0 for a footer with no shell: %+v", snap.Shells, snap)
	}
}
