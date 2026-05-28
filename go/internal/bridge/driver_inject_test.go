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

// TestInjectEnvelope_Keystroke_RawSendNoEscNoGate covers the cycle-124 F4
// hatch: a keystroke envelope is sent via SendKeys verbatim — NO ESC prefix
// (unlike interrupt), NO idle-gate (unlike command/nudge/system_rule), NO
// paste-buffer scratch file (unlike everything else), NO auto-Enter. This
// is the "full tmux control" channel the operator needs to dismiss the
// codex per-edit-approval modal that hung cycle-123 (`--body=Enter`),
// confirm y/N prompts (`--body=y`), navigate menus (`--body=Up`), or send
// control chars (`--body=C-c`). The gate-bypass is intentional: the
// operator may need to send keys precisely BECAUSE the agent isn't idle.
func TestInjectEnvelope_Keystroke_RawSendNoEscNoGate(t *testing.T) {
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	// Pane is busy (no marker) — keystroke must STILL fire (no idle-gate).
	tmux := &fakeTmux{paneSeq: []string{"thinking..."}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindKeystroke, Body: "Enter"})

	// The sole SendKeys call must be the body, with enter=false.
	if len(tmux.sentSeq) != 1 || tmux.sentSeq[0] != "Enter|false" {
		t.Fatalf("keystroke must SendKeys body verbatim with enter=false; sentSeq=%v", tmux.sentSeq)
	}
	// MUST NOT pre-send Escape (that's interrupt's behavior).
	for _, k := range tmux.sentSeq {
		if k == "Escape|false" {
			t.Fatalf("keystroke must NOT pre-send Escape; sentSeq=%v", tmux.sentSeq)
		}
	}
	// MUST NOT write a paste-buffer scratch file (that's injectText's path).
	if _, err := os.Stat(scratchPath(ws)); err == nil {
		t.Fatal("keystroke must not write the paste-buffer scratch file")
	}
	// MUST NOT re-queue (that's the command/nudge defer path).
	envs, _ := inbox.NewCursor(ws, "build").Drain()
	if len(envs) != 0 {
		t.Fatalf("keystroke must not re-queue; got %+v", envs)
	}
}

// TestInjectEnvelope_Keystroke_EmptyBodyIsNoop is a defensive pin: an empty
// --body must not crash and must not paste — the SendKeys impl (tmux.go
// line 59) treats an empty keys arg as a no-op send-keys call. The test
// also confirms no Escape pre-send leaks into the empty-body branch.
func TestInjectEnvelope_Keystroke_EmptyBodyIsNoop(t *testing.T) {
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"❯"}}
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	injectEnvelope(context.Background(), cfg, deps, lp, inbox.Envelope{Kind: inbox.KindKeystroke, Body: ""})

	// One SendKeys call recorded (kept for ledger uniformity), no scratch
	// file, no Escape token. The SendKeys body is empty.
	if len(tmux.sentSeq) != 1 || tmux.sentSeq[0] != "|false" {
		t.Fatalf("empty keystroke should still record one SendKeys with empty body; sentSeq=%v", tmux.sentSeq)
	}
	if _, err := os.Stat(scratchPath(ws)); err == nil {
		t.Fatal("empty keystroke must not paste")
	}
}

// TestInjectEnvelope_Keystroke_TmuxKeyNames covers the operator's
// per-modal-class repertoire: every tmux named key the bridge might need
// to send as a single envelope. The body is sent verbatim to SendKeys
// with enter=false — tmux interprets these names natively, so a body of
// "Escape" presses ESC, "C-c" sends Ctrl-C, "Up" presses ↑ etc. The fake
// tmux records `<keys>|<enter>` so the assertion pins both the body and
// the no-Enter contract. Idle-gate is bypassed (pane shows "busy") to
// prove the named-key send fires regardless of agent state.
func TestInjectEnvelope_Keystroke_TmuxKeyNames(t *testing.T) {
	keys := []string{
		"Escape", // cancel a modal
		"Enter",  // confirm a y/N prompt
		"C-c",    // Ctrl-C
		"C-d",    // Ctrl-D / EOF
		"Up",     // navigate menu
		"Down",
		"Left",
		"Right",
		"Tab",
		"BSpace", // backspace
		"PgUp",
		"PgDn",
		"Home",
		"End",
		"F1",  // function key
		"F12", // last function key
	}
	for _, k := range keys {
		t.Run(k, func(t *testing.T) {
			ws := t.TempDir()
			cfg := injectCfg(ws)
			deps := covDeps()
			tmux := &fakeTmux{paneSeq: []string{"busy"}} // gate-bypass
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

// TestInjectEnvelope_Keystroke_MultiToken pins that space-separated tmux
// tokens reach SendKeys as ONE concatenated string (tmux's send-keys
// itself parses the tokens). This is how the operator builds
// y-then-Enter, Esc-then-text, navigate-then-confirm sequences — the
// bridge is a transparent pass-through, the operator owns parsing.
func TestInjectEnvelope_Keystroke_MultiToken(t *testing.T) {
	cases := []struct {
		body string
	}{
		{"y Enter"},
		{"Escape Enter"},
		{"Up Up Down Down Left Right Left Right"}, // konami
		{"C-x C-s"},     // emacs save
		{"hello Enter"}, // literal + named
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

// TestInjectEnvelope_Keystroke_UnicodeBody pins that UTF-8 body bytes
// survive the SendKeys pass-through. Some CLIs prompt in non-ASCII
// (Japanese, Korean, Chinese localizations), and a literal `--body=はい`
// must reach the REPL byte-for-byte.
func TestInjectEnvelope_Keystroke_UnicodeBody(t *testing.T) {
	cases := []string{
		"はい",      // Japanese "yes"
		"네",       // Korean "yes"
		"是",       // Chinese "yes"
		"oui",     // ASCII (sanity)
		"é",       // composed accent
		"🚀 Enter", // emoji + named key
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

// TestInjectEnvelope_Keystroke_DeferCountIgnored pins that the
// DeferCount field on a keystroke envelope is NOT consulted by the
// dispatch — keystroke bypasses the idle-gate entirely, so it never
// enters the re-queue/defer path. An envelope arriving with
// DeferCount=99 (well past maxInjectDefer=10) must STILL fire its
// SendKeys, NOT drop.
func TestInjectEnvelope_Keystroke_DeferCountIgnored(t *testing.T) {
	ws := t.TempDir()
	cfg := injectCfg(ws)
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"busy"}} // idle-gate would block command/nudge
	deps.Tmux = tmux
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	envBig := inbox.Envelope{Kind: inbox.KindKeystroke, Body: "Enter", DeferCount: 999}
	injectEnvelope(context.Background(), cfg, deps, lp, envBig)

	if len(tmux.sentSeq) != 1 || tmux.sentSeq[0] != "Enter|false" {
		t.Fatalf("keystroke with high DeferCount must still fire; sentSeq=%v", tmux.sentSeq)
	}
	// And MUST NOT be re-queued to the inbox.
	envs, _ := inbox.NewCursor(ws, "build").Drain()
	if len(envs) != 0 {
		t.Fatalf("keystroke with high DeferCount must not re-queue; got %+v", envs)
	}
}

// TestInjectEnvelope_Keystroke_LongBody pins that very long bodies reach
// SendKeys intact. The tmux command line itself has limits in real
// deployments, but the bridge's pass-through must not truncate or split.
// 4 KB is well above any realistic operator-scripted body but below
// macOS ARG_MAX (256 KB) so the test stays portable.
func TestInjectEnvelope_Keystroke_LongBody(t *testing.T) {
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

// TestInjectEnvelope_Keystroke_SendKeysErrorSurfaced is the cycle-124
// review MEDIUM regression guard: a failing SendKeys MUST produce a
// "keystroke send failed" stderr line, NOT a "injected keystroke" success
// line. Prevents the silent-failure mode where an operator sees `injected
// keystroke "Enter"` in logs but nothing actually reached the (vanished)
// pane.
func TestInjectEnvelope_Keystroke_SendKeysErrorSurfaced(t *testing.T) {
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

// errInjectingTmux is a minimal fakeTmux that returns a configured error
// from SendKeys — used only by TestInjectEnvelope_Keystroke_SendKeysErrorSurfaced
// to drive the error branch added in cycle-124 review MEDIUM fix.
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
