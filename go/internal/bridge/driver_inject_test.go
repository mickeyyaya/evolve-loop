package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

func fixedTime() time.Time { return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC) }

func injectCfg(ws string) *Config {
	return &Config{Workspace: ws, Agent: "build", Worktree: ws}
}

func scratchPath(ws string) string {
	return filepath.Join(ws, ".bridge-inbox", "build-inject.txt")
}

func TestInjectEnvelope_CommandIdle_Pastes(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"❯"}}
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
	t.Parallel()
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"thinking..."}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindCommand, Body: "later"})

	if _, err := os.Stat(scratchPath(ws)); err == nil {
		t.Fatal("scratch file should not be written when agent is mid-turn")
	}
	envs, _ := inbox.NewCursor(ws, "build").Drain()
	if len(envs) != 1 || envs[0].Body != "later" || envs[0].DeferCount != 1 {
		t.Fatalf("expected one re-queued envelope with DeferCount=1, got %+v", envs)
	}
}

func TestInjectEnvelope_Keystroke_RawSendNoEscNoGate(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"thinking..."}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindKeystroke, Body: "Enter"})

	if len(tmux.sentSeq) != 1 || tmux.sentSeq[0] != "Enter|false" {
		t.Fatalf("keystroke must SendKeys body verbatim with enter=false; sentSeq=%v", tmux.sentSeq)
	}
	for _, k := range tmux.sentSeq {
		if k == "Escape|false" {
			t.Fatalf("keystroke must NOT pre-send Escape; sentSeq=%v", tmux.sentSeq)
		}
	}
	if _, err := os.Stat(scratchPath(ws)); err == nil {
		t.Fatal("keystroke must not write the paste-buffer scratch file")
	}
	envs, _ := inbox.NewCursor(ws, "build").Drain()
	if len(envs) != 0 {
		t.Fatalf("keystroke must not re-queue; got %+v", envs)
	}
}

func TestInjectEnvelope_Keystroke_SuspectWarnsButSends(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	var errb bytes.Buffer
	deps.Stderr = &errb
	tmux := &fakeTmux{paneSeq: []string{"thinking..."}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindKeystroke, Body: "Excape"})

	if !strings.Contains(errb.String(), "unrecognized key token") {
		t.Errorf("expected keyspec WARN on stderr, got %q", errb.String())
	}
	if len(tmux.sentSeq) != 1 || tmux.sentSeq[0] != "Excape|false" {
		t.Fatalf("suspect keystroke must still send verbatim; sentSeq=%v", tmux.sentSeq)
	}
}

func TestInjectEnvelope_Keystroke_EmptyBodyIsNoop(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"❯"}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindKeystroke, Body: ""})

	if len(tmux.sentSeq) != 1 || tmux.sentSeq[0] != "|false" {
		t.Fatalf("empty keystroke should still record one SendKeys with empty body; sentSeq=%v", tmux.sentSeq)
	}
	if _, err := os.Stat(scratchPath(ws)); err == nil {
		t.Fatal("empty keystroke must not paste")
	}
}

func TestInjectEnvelope_Keystroke_TmuxKeyNames(t *testing.T) {
	t.Parallel()
	keys := []string{
		"Escape",
		"Enter",
		"C-c",
		"C-d",
		"Up",
		"Down",
		"Left",
		"Right",
		"Tab",
		"BSpace",
		"PgUp",
		"PgDn",
		"Home",
		"End",
		"F1",
		"F12",
	}
	for _, k := range keys {
		t.Run(k, func(t *testing.T) {
			ws := t.TempDir()
			cfg := injectCfg(ws)
			deps := covDeps()
			tmux := &fakeTmux{paneSeq: []string{"busy"}}
			deps.Tmux = tmux
			lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

			injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindKeystroke, Body: k})

			want := k + "|false"
			if len(tmux.sentSeq) != 1 || tmux.sentSeq[0] != want {
				t.Errorf("key %q: sentSeq=%v want [%q]", k, tmux.sentSeq, want)
			}
		})
	}
}

func TestInjectEnvelope_Keystroke_MultiToken(t *testing.T) {
	t.Parallel()
	cases := []struct {
		body string
	}{
		{"y Enter"},
		{"Escape Enter"},
		{"Up Up Down Down Left Right Left Right"},
		{"C-x C-s"},
		{"hello Enter"},
	}
	for _, tc := range cases {
		t.Run(tc.body, func(t *testing.T) {
			ws := t.TempDir()
			cfg := injectCfg(ws)
			deps := covDeps()
			tmux := &fakeTmux{paneSeq: []string{"❯"}}
			deps.Tmux = tmux
			lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

			injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindKeystroke, Body: tc.body})

			want := tc.body + "|false"
			if len(tmux.sentSeq) != 1 || tmux.sentSeq[0] != want {
				t.Errorf("body %q: sentSeq=%v want [%q]", tc.body, tmux.sentSeq, want)
			}
		})
	}
}

func TestInjectEnvelope_Keystroke_UnicodeBody(t *testing.T) {
	t.Parallel()
	cases := []string{
		"はい",
		"네",
		"是",
		"oui",
		"é",
		"🚀 Enter",
	}
	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			ws := t.TempDir()
			cfg := injectCfg(ws)
			deps := covDeps()
			tmux := &fakeTmux{paneSeq: []string{"❯"}}
			deps.Tmux = tmux
			lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

			injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindKeystroke, Body: body})

			want := body + "|false"
			if len(tmux.sentSeq) != 1 || tmux.sentSeq[0] != want {
				t.Errorf("unicode body %q: sentSeq=%v want [%q]", body, tmux.sentSeq, want)
			}
		})
	}
}

func TestInjectEnvelope_Keystroke_DeferCountIgnored(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"busy"}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	envBig := inbox.Envelope{Kind: inbox.KindKeystroke, Body: "Enter", DeferCount: 999}
	injectEnvelope(context.Background(), cfg, deps, lp, envBig)

	if len(tmux.sentSeq) != 1 || tmux.sentSeq[0] != "Enter|false" {
		t.Fatalf("keystroke with high DeferCount must still fire; sentSeq=%v", tmux.sentSeq)
	}
	envs, _ := inbox.NewCursor(ws, "build").Drain()
	if len(envs) != 0 {
		t.Fatalf("keystroke with high DeferCount must not re-queue; got %+v", envs)
	}
}

func TestInjectEnvelope_Keystroke_LongBody(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"❯"}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	body := strings.Repeat("x", 4096)
	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindKeystroke, Body: body})

	if len(tmux.sentSeq) != 1 {
		t.Fatalf("expected 1 SendKeys call; got %d (sentSeq=%v)", len(tmux.sentSeq), tmux.sentSeq)
	}
	got := tmux.sentSeq[0]
	wantSuffix := "|false"
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("call must have enter=false suffix; got %q", got[:min(len(got), 80)])
	}
	gotBody := got[:len(got)-len(wantSuffix)]
	if gotBody != body {
		t.Fatalf("long body truncated/altered; len(got)=%d len(want)=%d", len(gotBody), len(body))
	}
}

func TestInjectEnvelope_Keystroke_SendKeysErrorSurfaced(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := injectCfg(ws)
	stderr := &bytes.Buffer{}
	deps := covDeps()
	deps.Stderr = stderr
	tmux := &errInjectingTmux{sendKeysErr: errors.New("session not found")}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindKeystroke, Body: "Enter"})

	out := stderr.String()
	if !strings.Contains(out, "keystroke send failed") {
		t.Errorf("expected 'keystroke send failed' in stderr; got:\n%s", out)
	}
	if strings.Contains(out, "injected keystroke") {
		t.Errorf("MUST NOT log success when SendKeys fails; got:\n%s", out)
	}
}

type errInjectingTmux struct {
	sendKeysErr error
}

func (e *errInjectingTmux) HasSession(context.Context, string) bool            { return true }
func (e *errInjectingTmux) NewSession(context.Context, string, int, int) error { return nil }
func (e *errInjectingTmux) SendKeys(context.Context, string, string, bool) error {
	return e.sendKeysErr
}
func (e *errInjectingTmux) CapturePane(context.Context, string, int) (string, error) {
	return "busy", nil
}
func (e *errInjectingTmux) LoadBuffer(context.Context, string, string) error { return nil }
func (e *errInjectingTmux) PasteBuffer(context.Context, string) error        { return nil }
func (e *errInjectingTmux) KillSession(context.Context, string) error        { return nil }

func TestInjectEnvelope_Interrupt_EscBeforeBody(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"thinking..."}}
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
	t.Parallel()
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"busy"}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindCommand, Body: "x", DeferCount: maxInjectDefer})

	envs, _ := inbox.NewCursor(ws, "build").Drain()
	if len(envs) != 0 {
		t.Fatalf("envelope past defer budget should be dropped, got %+v", envs)
	}
}

func TestRunTmuxREPL_SkipsPreLaunchBacklog(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	artifact := filepath.Join(ws, "a")
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

type captureHookTmux struct {
	fakeTmux
	artifact string
	marker   string
	n        int
	wrote    bool
}

func (c *captureHookTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	c.n++
	if c.n >= 2 && !c.wrote {
		_ = os.WriteFile(c.artifact, []byte("done"), 0o644)
		c.wrote = true
	}
	return c.marker, nil
}

func TestRunTmuxREPL_InjectsPostLaunchAppend(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	artifact := filepath.Join(ws, "a")
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws, Agent: "build",
		Artifact: artifact, StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	deps := covDeps()
	appended := false
	deps.Sleep = func(d time.Duration) {
		if d == 2*time.Second && !appended {
			appended = true
			_ = inbox.Append(ws, "build", inbox.Envelope{Kind: inbox.KindCommand, Body: "live cmd"}, fixedTime)
		}
	}
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

type pasteHookTmux struct {
	fakeTmux
	artifact string
	marker   string
	pasteN   int
}

func (p *pasteHookTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	return p.marker, nil
}

func (p *pasteHookTmux) PasteBuffer(_ context.Context, _ string) error {
	p.pasteN++
	if p.pasteN == 2 {
		_ = os.WriteFile(p.artifact, []byte("done"), 0o644)
	}
	return nil
}
