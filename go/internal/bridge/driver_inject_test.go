package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

func fixedTime() time.Time { return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC) }

// driver_inject_test.go — live command injection: draining an agent inbox
// from the artifact-wait poll loop and injecting envelopes into the running
// REPL. injectEnvelope is tested directly for the three semantics; the loop
// wiring (cursor-at-EOF + drain) is tested via runTmuxREPL.

func injectCfg(ws string) *Config {
	return &Config{Workspace: ws, Agent: "build", Worktree: ws}
}

func scratchPath(ws string) string {
	return filepath.Join(ws, ".bridge-inbox", "build-inject.txt")
}

func TestInjectEnvelope_CommandIdle_Pastes(t *testing.T) {
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"❯"}} // marker present → agent idle
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindCommand, Body: "do the thing"})

	body, err := os.ReadFile(scratchPath(ws))
	if err != nil {
		t.Fatalf("scratch file not written (paste path not taken): %v", err)
	}
	if string(body) != "do the thing" {
		t.Errorf("scratch body = %q, want %q", string(body), "do the thing")
	}
	if indexOf(tmux.sentSeq, "|true") < 0 {
		t.Errorf("paste should be followed by Enter; sentSeq=%v", tmux.sentSeq)
	}
}

func TestInjectEnvelope_CommandMidTurn_Defers(t *testing.T) {
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"thinking..."}} // marker absent → busy
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindCommand, Body: "later"})

	// Must NOT paste while busy.
	if _, err := os.Stat(scratchPath(ws)); err == nil {
		t.Fatal("scratch file should not be written when agent is mid-turn")
	}
	// Must re-queue with an incremented defer count.
	envs, _ := inbox.NewCursor(ws, "build").Drain()
	if len(envs) != 1 || envs[0].Body != "later" || envs[0].DeferCount != 1 {
		t.Fatalf("expected one re-queued envelope with DeferCount=1, got %+v", envs)
	}
}

func TestInjectEnvelope_Interrupt_EscBeforeBody(t *testing.T) {
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"thinking..."}} // busy, but interrupt ignores the gate
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindInterrupt, Body: "STOP"})

	if len(tmux.sentSeq) == 0 || tmux.sentSeq[0] != "Escape|false" {
		t.Fatalf("interrupt must send Escape first; sentSeq=%v", tmux.sentSeq)
	}
	body, err := os.ReadFile(scratchPath(ws))
	if err != nil || string(body) != "STOP" {
		t.Fatalf("interrupt should still paste the body; body=%q err=%v", string(body), err)
	}
}

func TestInjectEnvelope_DeferBudgetExhausted_Drops(t *testing.T) {
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"busy"}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	// Already at the max defer count → dropped, not re-queued.
	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindCommand, Body: "x", DeferCount: maxInjectDefer})

	envs, _ := inbox.NewCursor(ws, "build").Drain()
	if len(envs) != 0 {
		t.Fatalf("envelope past defer budget should be dropped, got %+v", envs)
	}
}

// --- loop wiring: cursor-at-EOF skips backlog; post-launch append injects ---

func TestRunTmuxREPL_SkipsPreLaunchBacklog(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	artifact := filepath.Join(ws, "a")
	// Pre-launch envelope: must be skipped (cursor seeks to EOF on entry).
	// The artifact is NOT pre-present, so the drain DOES run on iter 1 — if
	// cursor-at-EOF were broken, "stale" would be injected.
	if err := inbox.Append(ws, "build", inbox.Envelope{Kind: inbox.KindCommand, Body: "stale"}, fixedTime); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws, Agent: "build",
		Artifact: artifact, StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	deps := covDeps()
	deps.Tmux = &captureHookTmux{artifact: artifact, marker: "❯"}
	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}

	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}
	if _, err := os.Stat(scratchPath(ws)); err == nil {
		t.Fatal("pre-launch backlog must NOT be injected (cursor-at-EOF)")
	}
}

// captureHookTmux returns the marker always and writes the artifact on its
// 2nd CapturePane call (boot=1, first artifact-loop tick=2) so the wait loop
// runs exactly one drain before exiting.
type captureHookTmux struct {
	fakeTmux
	artifact string
	marker   string
	n        int
}

func (c *captureHookTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	c.n++
	if c.n >= 2 {
		_ = os.WriteFile(c.artifact, []byte("done"), 0o644)
	}
	return c.marker, nil
}

func TestRunTmuxREPL_InjectsPostLaunchAppend(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	artifact := filepath.Join(ws, "a")
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws, Agent: "build",
		Artifact: artifact, StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	deps := covDeps()
	// Queue a live command on the first artifact-wait Sleep (the loop's
	// distinctive 2s tick — boot/prompt sleeps are 1s) so the append lands
	// AFTER the cursor seeks to EOF, exactly as an external sender would.
	appended := false
	deps.Sleep = func(d time.Duration) {
		if d == 2*time.Second && !appended {
			appended = true
			_ = inbox.Append(ws, "build", inbox.Envelope{Kind: inbox.KindCommand, Body: "live cmd"}, fixedTime)
		}
	}
	// pasteHookTmux writes the artifact once the injection paste completes
	// (PasteBuffer #2; #1 is the prompt) so the loop exits deterministically.
	pt := &pasteHookTmux{artifact: artifact, marker: "❯"}
	deps.Tmux = pt
	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}

	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}
	body, err := os.ReadFile(scratchPath(ws))
	if err != nil || string(body) != "live cmd" {
		t.Fatalf("post-launch command should be injected; body=%q err=%v", string(body), err)
	}
}

// pasteHookTmux is a fakeTmux variant: it reports idle (marker) always, and
// the 2nd PasteBuffer (the injection; #1 is the prompt) writes the artifact
// so the wait loop exits once the injected command has been delivered.
type pasteHookTmux struct {
	fakeTmux
	artifact string
	marker   string
	pasteN   int
}

func (p *pasteHookTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	return p.marker, nil // always idle
}

func (p *pasteHookTmux) PasteBuffer(_ context.Context, _ string) error {
	p.pasteN++
	if p.pasteN == 2 { // injection paste done → finish the work
		_ = os.WriteFile(p.artifact, []byte("done"), 0o644)
	}
	return nil
}
