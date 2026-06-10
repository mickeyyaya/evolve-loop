package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// completion_test.go — unit coverage for the ADR-0027 completion Strategy:
// the factory's default-to-artifact safety, the artifact detector's parity
// with artifactReady, and the stdout detector's idle-debounce + activity
// gating (the router/advisor's contract).

func TestNewCompletionDetector_DefaultsToArtifact(t *testing.T) {
	cfg := &Config{Artifact: "/tmp/x"}
	lp := tmuxLaunch{promptMarker: "❯"}
	for _, mode := range []string{"", "artifact", "bogus-typo"} {
		if _, ok := newCompletionDetector(mode, cfg, Deps{}, lp).(*artifactDetector); !ok {
			t.Errorf("mode %q: want *artifactDetector (default-safe), got %T",
				mode, newCompletionDetector(mode, cfg, Deps{}, lp))
		}
	}
	if _, ok := newCompletionDetector("stdout", cfg, Deps{}, lp).(*stdoutDetector); !ok {
		t.Error("mode \"stdout\": want *stdoutDetector")
	}
}

func TestArtifactDetector_Poll(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	d := &artifactDetector{cfg: &Config{Workspace: ws, Artifact: canonical}}

	// Absent → not ready, no error, no note.
	if ready, _, note, err := d.poll(context.Background()); ready || err != nil || note != "" {
		t.Fatalf("absent artifact: got (ready=%v, note=%q, err=%v), want (false, \"\", nil)", ready, note, err)
	}
	// Present (non-empty) → ready with an "appeared" note.
	if err := os.WriteFile(canonical, []byte("DONE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ready, _, note, err := d.poll(context.Background())
	if !ready || err != nil {
		t.Fatalf("present artifact: ready=%v err=%v, want ready", ready, err)
	}
	if !strings.Contains(note, "appeared") {
		t.Errorf("note = %q, want it to mention 'appeared'", note)
	}
}

// stdoutPoll drives a stdoutDetector across a pane sequence (last value
// repeats, matching fakeTmux) and returns the poll index at which it reported
// ready, or -1 if it never did within maxPolls.
func stdoutPoll(t *testing.T, marker string, threshold int, panes []string, maxPolls int) int {
	t.Helper()
	tmux := &fakeTmux{paneSeq: panes}
	d := &stdoutDetector{
		cfg:       &Config{},
		deps:      Deps{Tmux: tmux},
		lp:        tmuxLaunch{promptMarker: marker},
		threshold: threshold,
	}
	for i := 0; i < maxPolls; i++ {
		ready, _, _, err := d.poll(context.Background())
		if err != nil {
			t.Fatalf("poll %d: unexpected err %v", i, err)
		}
		if ready {
			return i
		}
	}
	return -1
}

func TestStdoutDetector_ReadyAfterActivityThenIdle(t *testing.T) {
	// baseline → activity (thinking, differs) → settled output w/ marker.
	// fakeTmux repeats the last entry, so the idle run accrues to threshold.
	panes := []string{"prompt delivered", "thinking…", "⏺ [ {…} ]\n❯"}
	if got := stdoutPoll(t, "❯", 3, panes, 12); got < 0 {
		t.Fatal("stdout detector never reported ready despite activity + idle stability")
	}
}

func TestStdoutDetector_NotReadyWhileStreaming(t *testing.T) {
	// Marker present every tick, but the pane keeps changing (streaming) →
	// the stability counter resets each poll → never ready.
	panes := []string{"a", "b❯", "c❯", "d❯", "e❯", "f❯", "g❯", "h❯"}
	if got := stdoutPoll(t, "❯", 3, panes, len(panes)); got >= 0 {
		t.Fatalf("streaming pane must not complete; reported ready at poll %d", got)
	}
}

func TestStdoutDetector_NotReadyBeforeActivity(t *testing.T) {
	// Marker present from the very first capture and the pane never changes:
	// the agent never demonstrably did work → must NOT false-fire on the
	// marker that was already visible before the turn began.
	panes := []string{"❯ idle prompt"}
	if got := stdoutPoll(t, "❯", 3, panes, 8); got >= 0 {
		t.Fatalf("must not complete without observed activity; reported ready at poll %d", got)
	}
}

func TestParseLaunchArgs_CompletionFlag(t *testing.T) {
	// Both --completion=stdout and the env fallback round-trip into rawLaunch.
	raw, err := parseLaunchArgs([]string{"--completion=stdout"}, nil)
	if err != nil {
		t.Fatalf("parse --completion=stdout: %v", err)
	}
	if raw.completion != "stdout" {
		t.Errorf("flag form: completion = %q, want stdout", raw.completion)
	}
	rawEnv, err := parseLaunchArgs(nil, map[string]string{"BRIDGE_COMPLETION": "stdout"})
	if err != nil {
		t.Fatalf("parse BRIDGE_COMPLETION env: %v", err)
	}
	if rawEnv.completion != "stdout" {
		t.Errorf("env form: completion = %q, want stdout", rawEnv.completion)
	}
}

func TestClaudeTmux_StdoutCompletion_NoArtifactNeeded(t *testing.T) {
	// The stdout contract completes WITHOUT any artifact file: the REPL boots
	// (marker in pane[0]), shows activity, then settles on the marker — and the
	// driver returns ExitOK even though fx.artifact was never written. This is
	// the cycle-117 advisor deadlock, fixed.
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{
		tmuxPromptMarkerDefault,                  // boot: marker seen → REPL ready
		tmuxPromptMarkerDefault,                  // interval baseline pre-capture
		"thinking…",                              // detector baseline (poll 1)
		"⏺ [ done ]\n" + tmuxPromptMarkerDefault, // settles here; repeats → idle accrues
	}}
	code, stderr := runTmux(t, fx, tmux, nil, "--allow-bypass", "--completion=stdout")
	if code != ExitOK {
		t.Fatalf("stdout completion: exit = %d, want ExitOK; stderr=%q", code, stderr)
	}
	if fileNonEmpty(fx.artifact) {
		t.Fatal("stdout contract must not require (or create) an artifact file")
	}
}

// TestGitEvidenceDetector_ClosureCalled covers the gitCmd closure body
// (lines 98-103 in completion.go) which is only executed when d.poll() fires.
// The closure calls deps.Runner; a fake runner lets us verify the dispatch
// without a real git worktree.
func TestGitEvidenceDetector_ClosureCalled(t *testing.T) {
	called := false
	deps := Deps{
		Runner: func(_ context.Context, _, _ string, _ []string, _ []string,
			_ io.Reader, stdout, _ io.Writer) (int, error) {
			called = true
			_, _ = stdout.Write([]byte("abc123"))
			return 0, nil
		},
	}
	cfg := &Config{Workspace: t.TempDir(), Agent: "build", Worktree: t.TempDir()}
	d := newGitEvidenceDetector(cfg, deps)
	_, _, _, _ = d.poll(context.Background())
	if !called {
		t.Error("gitCmd closure was not invoked by poll()")
	}
}
