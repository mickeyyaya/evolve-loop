package bridge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type TmuxController interface {
	HasSession(ctx context.Context, name string) bool
	NewSession(ctx context.Context, name string, width, height int) error
	SendKeys(ctx context.Context, session, keys string, enter bool) error
	CapturePane(ctx context.Context, session string, scrollback int) (string, error)
	LoadBuffer(ctx context.Context, session, file string) error
	PasteBuffer(ctx context.Context, session string) error
	KillSession(ctx context.Context, session string) error
}

type PaneCommander interface {
	PaneCommand(ctx context.Context, session string) (string, error)
}

type paneTTYReader interface {
	paneTTY(context.Context, string) (string, error)
}

type execTmux struct{}

const tmuxCmdTimeout = 30 * time.Second

func runCmdBounded(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := boundedCommand(cctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil && cctx.Err() == context.DeadlineExceeded {
		return out.String(), fmt.Errorf("%s %s: timed out after %s: %w", name, strings.Join(args, " "), timeout, cctx.Err())
	}
	return out.String(), err
}

const TmuxSocket = "evolve-bridge"

const TmuxSocketEnv = ipcenv.TmuxSocketKey

func DeriveRunSocket(pid int) string {
	return TmuxSocket + "-p" + strconv.Itoa(pid)
}

func DeriveTestSocket(pid int) string {
	return TmuxSocket + "-t" + strconv.Itoa(pid)
}

func ExactSessionTarget(session string) string {
	return "=" + session + ":"
}

func tmuxSocketName() string {
	if s := strings.TrimSpace(os.Getenv(TmuxSocketEnv)); s != "" {
		return s
	}
	return TmuxSocket
}

func TmuxSocketDir() string {
	base := os.Getenv("TMUX_TMPDIR")
	if base == "" {
		base = "/tmp"
	}
	return filepath.Join(base, fmt.Sprintf("tmux-%d", os.Getuid()))
}

func tmuxSocketGuard() (string, []string, error) {
	dir := TmuxSocketDir()
	if !filepath.IsAbs(dir) {
		return "", nil, fmt.Errorf("tmux socket directory %q is not absolute; set TMUX_TMPDIR to an absolute path", dir)
	}
	real, err := canonicalSandboxPath(dir)
	if err != nil {
		return "", nil, err
	}
	return real, []string{filepath.Join(real, tmuxSocketName()), filepath.Join(real, "default")}, nil
}

func TmuxSocketArgs(args ...string) []string {
	return append([]string{"-L", tmuxSocketName()}, args...)
}

func (execTmux) run(ctx context.Context, args ...string) (string, error) {
	return runCmdBounded(ctx, tmuxCmdTimeout, "tmux", TmuxSocketArgs(args...)...)
}

func (t execTmux) HasSession(ctx context.Context, name string) bool {
	_, err := t.run(ctx, "has-session", "-t", ExactSessionTarget(name))
	return err == nil
}

func (t execTmux) NewSession(ctx context.Context, name string, width, height int) error {
	_, err := t.run(ctx, "new-session", "-d", "-s", name, "-x", fmt.Sprint(width), "-y", fmt.Sprint(height))
	return err
}

type workdirSessionStarter interface {
	NewSessionIn(ctx context.Context, name string, width, height int, workdir string) error
}

func (t execTmux) NewSessionIn(ctx context.Context, name string, width, height int, workdir string) error {
	_, err := t.run(ctx, "new-session", "-d", "-s", name, "-x", fmt.Sprint(width), "-y", fmt.Sprint(height), "-c", workdir)
	return err
}

func (t execTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	args := []string{"send-keys", "-t", ExactSessionTarget(session)}
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
	args := []string{"capture-pane", "-p", "-t", ExactSessionTarget(session)}
	if scrollback > 0 {
		args = []string{"capture-pane", "-p", "-S", fmt.Sprintf("-%d", scrollback), "-t", ExactSessionTarget(session)}
	}
	return t.run(ctx, args...)
}

func (t execTmux) LoadBuffer(ctx context.Context, session, file string) error {
	_, err := t.run(ctx, "load-buffer", "-b", session, file)
	return err
}

func (t execTmux) PasteBuffer(ctx context.Context, session string) error {
	_, err := t.run(ctx, "paste-buffer", "-b", session, "-t", ExactSessionTarget(session), "-d", "-p", "-r")
	return err
}

type windowJiggler interface {
	JiggleWindow(ctx context.Context, session string) error
}

func (t execTmux) JiggleWindow(ctx context.Context, session string) error {
	if _, err := t.run(ctx, "resize-window", "-t", ExactSessionTarget(session), "-L", "1"); err != nil {
		return err
	}
	_, err := t.run(ctx, "resize-window", "-t", ExactSessionTarget(session), "-R", "1")
	return err
}

func (t execTmux) KillSession(ctx context.Context, session string) error {
	_, err := t.run(ctx, "kill-session", "-t", ExactSessionTarget(session))
	return err
}

func (t execTmux) PaneCommand(ctx context.Context, session string) (string, error) {
	out, err := t.run(ctx, "display-message", "-p", "-t", ExactSessionTarget(session), "#{pane_current_command}")
	return strings.TrimSpace(out), err
}

func (t execTmux) paneTTY(ctx context.Context, session string) (string, error) {
	out, err := t.run(ctx, "display-message", "-p", "-t", ExactSessionTarget(session), "#{pane_tty}")
	return strings.TrimSpace(out), err
}

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
	PaneCmd        string
}

func (f *FakeTmuxController) PaneCommand(_ context.Context, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Events = append(f.Events, "panecmd")
	return f.PaneCmd, nil
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

func (f *FakeTmuxController) JiggleWindow(_ context.Context, session string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Events = append(f.Events, "jiggle:"+session)
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

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func (t execTmux) PanePIDs(ctx context.Context, session string) ([]int, error) {
	out, err := t.run(ctx, "list-panes", "-s", "-t", ExactSessionTarget(session), "-F", "#{pane_pid}")
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w", err)
	}
	return parsePanePIDs(out)
}

func parsePanePIDs(out string) ([]int, error) {
	var pids []int
	for _, line := range strings.Fields(out) {
		pid, err := strconv.Atoi(line)
		if err != nil {
			return nil, fmt.Errorf("pane pid %q: %w", line, err)
		}
		pids = append(pids, pid)
	}
	return pids, nil
}
