package bridge

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type failedTerminalTmux struct {
	*fakeTmux
	path  string
	err   error
	calls int
}

func (f *failedTerminalTmux) paneTTY(context.Context, string) (string, error) {
	f.calls++
	return f.path, f.err
}

func TestSandboxTerminalFailurePreventsLaunch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path string
		err        error
	}{
		{"lookup", "", errors.New("pane unavailable")},
		{"empty", "", nil},
		{"ordinary path", t.TempDir(), nil},
		{"traversal", "/dev/ttys001/../tty", nil},
		{"missing device", "/dev/ttys999999999", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mux := &failedTerminalTmux{fakeTmux: &fakeTmux{}, path: tc.path, err: tc.err}
			calls := 0
			deps := Deps{Tmux: mux, Stderr: io.Discard, Sleep: func(time.Duration) {}, Env: map[string]string{}, SandboxWrap: func(SandboxWrapRequest) ([]string, bool) { calls++; return nil, false }, hostOS: "darwin"}.withDefaults()
			cfg := &Config{ProjectRoot: root, Worktree: root, Workspace: root, RequireSandbox: true}
			code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{name: "terminal-fixture", session: "terminal-fixture", launchCmd: "fixture-must-not-launch", bootOnly: true})
			if err != nil || code != ExitSafetyGate {
				t.Fatalf("got code %d err %v, want safety gate", code, err)
			}
			if calls != 0 || mux.sentContains("fixture-must-not-launch") {
				t.Fatal("failed terminal lookup reached wrapping or child launch")
			}
		})
	}
}

func TestSandboxTerminalOptOutDoesNotQueryPane(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"off", " off "} {
		mux := &failedTerminalTmux{fakeTmux: &fakeTmux{}, err: errors.New("must not query")}
		path, err := sandboxTerminalPath(context.Background(), Deps{Tmux: mux, Env: map[string]string{envSandboxMode: mode}}, &Config{RequireSandbox: true}, "fixture")
		if path != "" || err != nil || mux.calls != 0 {
			t.Fatalf("mode %q: path=%q err=%v calls=%d", mode, path, err, mux.calls)
		}
	}
}

func TestSandboxTerminalLookupFailurePreventsLaunchOnEveryHost(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mux := &failedTerminalTmux{fakeTmux: &fakeTmux{}, err: errors.New("pane unavailable")}
	var stderr strings.Builder
	deps := Deps{Tmux: mux, Stderr: &stderr, Sleep: func(time.Duration) {}, Env: map[string]string{}, hostOS: "darwin"}.withDefaults()
	cfg := &Config{ProjectRoot: root, Worktree: root, Workspace: root, RequireSandbox: true}
	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{name: "terminal-fixture", session: "terminal-any-host", launchCmd: "fixture-must-not-launch", bootOnly: true})
	if err != nil || code != ExitSafetyGate || mux.calls != 1 {
		t.Fatalf("got code %d err %v calls %d, want the safety gate after one terminal lookup", code, err, mux.calls)
	}
	if !strings.Contains(stderr.String(), "terminal lookup failed") || mux.sentContains("fixture-must-not-launch") {
		t.Fatalf("a failed terminal lookup must stop before the launch and say so; stderr=%q", stderr.String())
	}
}

func TestSandboxTerminalPath_OnAMacHostWithNoPaneTerminalReaderGrantsNone(t *testing.T) {
	t.Parallel()
	path, err := sandboxTerminalPath(context.Background(), Deps{Tmux: &fakeTmux{}, hostOS: "darwin"}, &Config{RequireSandbox: true}, "fixture")
	if path != "" || err != nil {
		t.Fatalf("a controller that cannot read the pane terminal grants none; got %q, %v", path, err)
	}
}

func TestSandboxTerminalPath_OnAMacHostReturnsTheCheckedTerminal(t *testing.T) {
	t.Parallel()
	mux := &failedTerminalTmux{fakeTmux: &fakeTmux{}, path: "/dev/ttys004"}
	var checked string
	deps := Deps{Tmux: mux, hostOS: "darwin", terminalCheck: func(p string) error { checked = p; return nil }}
	path, err := sandboxTerminalPath(context.Background(), deps, &Config{RequireSandbox: true}, "fixture")
	if path != "/dev/ttys004" || err != nil || checked != "/dev/ttys004" {
		t.Fatalf("a checked terminal is the grant; got %q, %v, checked %q", path, err, checked)
	}
}

func TestSandboxTerminalPath_OffAMacHostQueriesNothing(t *testing.T) {
	t.Parallel()
	mux := &failedTerminalTmux{fakeTmux: &fakeTmux{}, err: errors.New("must not query")}
	path, err := sandboxTerminalPath(context.Background(), Deps{Tmux: mux, hostOS: "linux"}, &Config{RequireSandbox: true}, "fixture")
	if path != "" || err != nil || mux.calls != 0 {
		t.Fatalf("off macOS there is no terminal grant; got %q, %v, calls %d", path, err, mux.calls)
	}
}
