package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestNewCompletionDetector_DefaultsToArtifact(t *testing.T) {
	t.Parallel()
	cfg := &Config{Artifact: "/tmp/x"}
	lp := tmuxLaunch{promptMarker: "❯"}
	for _, mode := range []core.CompletionContract{"", "artifact", "bogus-typo"} {
		if _, ok := newCompletionDetector(mode, cfg, Deps{}, lp, dispatchBaseline{}).(*artifactDetector); !ok {
			t.Errorf("mode %q: want *artifactDetector (default-safe), got %T",
				mode, newCompletionDetector(mode, cfg, Deps{}, lp, dispatchBaseline{}))
		}
	}
	if _, ok := newCompletionDetector("stdout", cfg, Deps{}, lp, dispatchBaseline{}).(*stdoutDetector); !ok {
		t.Error("mode \"stdout\": want *stdoutDetector")
	}
}

func TestArtifactDetector_Poll(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	d := &artifactDetector{cfg: &Config{Workspace: ws, Artifact: canonical}}

	if ready, _, note, err := d.poll(context.Background()); ready || err != nil || note != "" {
		t.Fatalf("absent artifact: got (ready=%v, note=%q, err=%v), want (false, \"\", nil)", ready, note, err)
	}
	if err := os.WriteFile(canonical, []byte("DONE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, note := pollUntilReady(t, d, artifactStableTicks+2, nil)
	if got < 0 {
		t.Fatalf("present, unchanging artifact never reported ready within %d polls", artifactStableTicks+2)
	}
	if !strings.Contains(note, "appeared") {
		t.Errorf("note = %q, want it to mention 'appeared'", note)
	}
}

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
	t.Parallel()
	panes := []string{"prompt delivered", "thinking…", "⏺ [ {…} ]\n❯"}
	if got := stdoutPoll(t, "❯", 3, panes, 12); got < 0 {
		t.Fatal("stdout detector never reported ready despite activity + idle stability")
	}
}

func TestStdoutDetector_NotReadyWhileStreaming(t *testing.T) {
	t.Parallel()
	panes := []string{"a", "b❯", "c❯", "d❯", "e❯", "f❯", "g❯", "h❯"}
	if got := stdoutPoll(t, "❯", 3, panes, len(panes)); got >= 0 {
		t.Fatalf("streaming pane must not complete; reported ready at poll %d", got)
	}
}

func TestStdoutDetector_NotReadyBeforeActivity(t *testing.T) {
	t.Parallel()
	panes := []string{"❯ idle prompt"}
	if got := stdoutPoll(t, "❯", 3, panes, 8); got >= 0 {
		t.Fatalf("must not complete without observed activity; reported ready at poll %d", got)
	}
}

func TestParseLaunchArgs_CompletionFlag(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{
		tmuxPromptMarkerDefault,
		tmuxPromptMarkerDefault,
		tmuxPromptMarkerDefault,
		"thinking…",
		"⏺ [ done ]\n" + tmuxPromptMarkerDefault,
	}}
	code, stderr := runTmux(t, fx, tmux, nil, "--allow-bypass", "--completion=stdout")
	if code != ExitOK {
		t.Fatalf("stdout completion: exit = %d, want ExitOK; stderr=%q", code, stderr)
	}
	if fileNonEmpty(fx.artifact) {
		t.Fatal("stdout contract must not require (or create) an artifact file")
	}
}

func TestGitEvidenceDetector_ClosureCalled(t *testing.T) {
	t.Parallel()
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
