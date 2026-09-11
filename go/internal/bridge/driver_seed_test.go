package bridge

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// driver_seed_test.go — Realization.REPLInput seed injection: lines fed into
// the REPL after the boot marker, before the task prompt. Closes the
// previously-dead REPLInput field (ADR-0022).

func indexOf(seq []string, want string) int {
	for i, s := range seq {
		if s == want {
			return i
		}
	}
	return -1
}

func TestRunTmuxREPL_SeedREPLInput_BeforePrompt(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	writeJSON(t, filepath.Join(ws, "a"), "done") // artifact present → exits after boot+prompt
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws,
		Artifact: filepath.Join(ws, "a"), StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e"),
		Realization: Realization{REPLInput: []string{"/model sonnet", "/foo"}}}
	deps := covDeps()
	var observations []modelDispatch
	deps.onModelDispatch = func(observation modelDispatch) {
		observations = append(observations, observation)
	}
	tmux := &fakeTmux{paneSeq: []string{"❯"}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1,
		modelDispatch: modelDispatch{model: "argv-model", source: modelDispatchArgv}}

	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}
	seedIdx := indexOf(tmux.sentSeq, "/model sonnet|true")
	if seedIdx < 0 || indexOf(tmux.sentSeq, "/foo|true") < 0 {
		t.Fatalf("seed lines not sent; sentSeq=%v", tmux.sentSeq)
	}
	// The prompt is delivered via paste + a bare Enter ("|true"); the seed
	// must precede it.
	promptEnter := indexOf(tmux.sentSeq, "|true")
	if promptEnter < 0 || seedIdx >= promptEnter {
		t.Fatalf("seed must precede prompt Enter; seedIdx=%d promptEnter=%d seq=%v", seedIdx, promptEnter, tmux.sentSeq)
	}
	if len(observations) != 2 || observations[0].model != "argv-model" || observations[0].source != modelDispatchArgv ||
		observations[1].model != "sonnet" || observations[1].source != modelDispatchREPL {
		t.Fatalf("dispatch observations = %+v, want launch argv then sent REPL selector", observations)
	}
}

func TestRunTmuxREPL_SeedSkippedOnNamedResume(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	writeJSON(t, filepath.Join(ws, "a"), "done")
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws, SessionName: "work",
		Artifact: filepath.Join(ws, "a"), StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e"),
		Realization: Realization{REPLInput: []string{"/model sonnet"}}}
	deps := covDeps()
	tmux := &fakeTmux{existing: map[string]bool{"s": true}, paneSeq: []string{"❯"}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", named: true, launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}

	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}
	if indexOf(tmux.sentSeq, "/model sonnet|true") >= 0 {
		t.Fatalf("seed must be skipped on named-session resume; sentSeq=%v", tmux.sentSeq)
	}
}

type failLaunchSendTmux struct {
	*fakeTmux
	sends int
}

func (f *failLaunchSendTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	f.sends++
	if f.sends == 2 { // cd succeeds; the CLI launch line fails
		return errors.New("tmux transport unavailable")
	}
	return f.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func TestRunTmuxREPL_FailedLaunchSendDoesNotClaimDispatch(t *testing.T) {
	ws := t.TempDir()
	promptFile := writeJSON(t, filepath.Join(ws, "prompt.txt"), "hi")
	cfg := &Config{
		PromptFile: promptFile, Workspace: ws, Worktree: ws,
		Artifact: filepath.Join(ws, "artifact"), StdoutLog: filepath.Join(ws, "stdout"), StderrLog: filepath.Join(ws, "stderr"),
	}
	deps := covDeps()
	deps.Sleep = func(time.Duration) {}
	deps.Tmux = &failLaunchSendTmux{fakeTmux: &fakeTmux{}}
	var observations []modelDispatch
	deps.onModelDispatch = func(observation modelDispatch) {
		observations = append(observations, observation)
	}
	lp := tmuxLaunch{
		name: "claude-tmux", session: "s", launchCmd: "claude --model opus", promptMarker: "❯",
		modelDispatch: modelDispatch{model: "opus", source: modelDispatchArgv},
	}

	code, err := runTmuxREPL(context.Background(), cfg, deps, lp)
	if code != ExitBadFlags || err == nil || !strings.Contains(err.Error(), "tmux transport unavailable") {
		t.Fatalf("runTmuxREPL = (%d, %v), want contextual launch-send failure", code, err)
	}
	if len(observations) != 0 {
		t.Fatalf("failed launch transport claimed dispatch: %+v", observations)
	}
}
