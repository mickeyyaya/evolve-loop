package bridge

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sync"
)

// TmuxController is the seam over the `tmux` operations the *-tmux
// drivers need to drive an interactive CLI REPL. The real impl
// (execTmux) shells out to tmux; tests inject a scriptable fake so the
// REPL state machine is exercised with no tmux and no wall-clock waits.
type TmuxController interface {
	// HasSession reports whether a session with name exists.
	HasSession(ctx context.Context, name string) bool
	// NewSession creates a detached session of the given pane size.
	NewSession(ctx context.Context, name string, width, height int) error
	// SendKeys sends literal keys to the session; when enter is true a
	// trailing Enter keypress is appended (the bash `send-keys … Enter`).
	SendKeys(ctx context.Context, session, keys string, enter bool) error
	// CapturePane returns the pane contents. scrollback>0 captures that
	// many lines of history (bash `-S -<n>`); 0 captures the visible pane.
	CapturePane(ctx context.Context, session string, scrollback int) (string, error)
	// LoadBuffer loads a file into the tmux paste buffer.
	LoadBuffer(ctx context.Context, session, file string) error
	// PasteBuffer pastes the buffer into the session.
	PasteBuffer(ctx context.Context, session string) error
	// KillSession terminates the session (best-effort; no error if absent).
	KillSession(ctx context.Context, session string) error
}

// execTmux is the production TmuxController — thin wrappers over the
// tmux binary. Mirrors the exact invocations in drivers/claude-tmux.sh.
type execTmux struct{}

func (execTmux) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func (t execTmux) HasSession(ctx context.Context, name string) bool {
	_, err := t.run(ctx, "has-session", "-t", name)
	return err == nil
}

func (t execTmux) NewSession(ctx context.Context, name string, width, height int) error {
	_, err := t.run(ctx, "new-session", "-d", "-s", name, "-x", fmt.Sprint(width), "-y", fmt.Sprint(height))
	return err
}

func (t execTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	args := []string{"send-keys", "-t", session}
	if keys != "" {
		args = append(args, keys)
	}
	if enter {
		args = append(args, "Enter")
	}
	_, err := t.run(ctx, args...)
	return err
}

func (t execTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	args := []string{"capture-pane", "-p", "-t", session}
	if scrollback > 0 {
		args = []string{"capture-pane", "-p", "-S", fmt.Sprintf("-%d", scrollback), "-t", session}
	}
	return t.run(ctx, args...)
}

func (t execTmux) LoadBuffer(ctx context.Context, session, file string) error {
	// Name the buffer after the session (via -b) so concurrent launches on the
	// shared tmux server each have their own buffer and cannot cross-paste.
	// Single-launch behavior is identical to the old global-buffer approach.
	_, err := t.run(ctx, "load-buffer", "-b", session, file)
	return err
}

func (t execTmux) PasteBuffer(ctx context.Context, session string) error {
	// -b selects this session's named buffer; -d deletes it after pasting so
	// the server's buffer table doesn't accumulate one entry per launch.
	_, err := t.run(ctx, "paste-buffer", "-b", session, "-t", session, "-d")
	return err
}

func (t execTmux) KillSession(ctx context.Context, session string) error {
	_, err := t.run(ctx, "kill-session", "-t", session)
	return err
}

// FakeTmuxController is a scriptable TmuxController for deterministic REPL
// state-machine tests. CapturePane consumes CaptureFrames in order and panics on
// underrun, so a fixture that forgets a frame fails at the exact missing read.
type FakeTmuxController struct {
	mu             sync.Mutex
	Existing       map[string]bool
	CaptureFrames  []string
	Events         []string
	SentKeys       []string
	SentSeq        []string
	LoadedBuffers  []string
	PasteCount     int
	KilledSessions []string
	NewSessionErr  error
}

func (f *FakeTmuxController) HasSession(_ context.Context, name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Existing[name]
}

func (f *FakeTmuxController) NewSession(_ context.Context, name string, _, _ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.NewSessionErr != nil {
		return f.NewSessionErr
	}
	if f.Existing == nil {
		f.Existing = map[string]bool{}
	}
	f.Existing[name] = true
	f.Events = append(f.Events, "new-session:"+name)
	return nil
}

func (f *FakeTmuxController) SendKeys(_ context.Context, _ string, keys string, enter bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SentKeys = append(f.SentKeys, keys)
	f.SentSeq = append(f.SentSeq, fmt.Sprintf("%s|%v", keys, enter))
	f.Events = append(f.Events, fmt.Sprintf("send:%s|%v", keys, enter))
	return nil
}

func (f *FakeTmuxController) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.CaptureFrames) == 0 {
		panic("FakeTmuxController CapturePane underrun")
	}
	frame := f.CaptureFrames[0]
	f.CaptureFrames = f.CaptureFrames[1:]
	f.Events = append(f.Events, "capture")
	return frame, nil
}

func (f *FakeTmuxController) LoadBuffer(_ context.Context, _ string, file string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.LoadedBuffers = append(f.LoadedBuffers, file)
	f.Events = append(f.Events, "load-buffer")
	return nil
}

func (f *FakeTmuxController) PasteBuffer(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.PasteCount++
	f.Events = append(f.Events, "paste-buffer")
	return nil
}

func (f *FakeTmuxController) KillSession(_ context.Context, session string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Existing, session)
	f.KilledSessions = append(f.KilledSessions, session)
	f.Events = append(f.Events, "kill-session:"+session)
	return nil
}

// ansiRE matches the CSI / OSC escape sequences the bash driver strips
// from scrollback (sed 's/\x1b\[[0-9;]*[a-zA-Z]//g; s/\x1b\][^\x07]*\x07//g').
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

// stripANSI removes terminal escape sequences from captured scrollback so
// the stdout-log is plain text (the bash driver's sed pass).
func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}
