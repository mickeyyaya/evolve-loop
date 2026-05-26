package bridge

import (
	"context"
	"path/filepath"
	"testing"
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
	tmux := &fakeTmux{paneSeq: []string{"❯"}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}

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
