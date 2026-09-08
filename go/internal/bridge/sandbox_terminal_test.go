package bridge

import (
	"context"
	"errors"
	"io"
	"runtime"
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
	if runtime.GOOS != "darwin" {
		t.Skip("macOS terminal grant")
	}
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
			deps := Deps{Tmux: mux, Stderr: io.Discard, Sleep: func(time.Duration) {}, Env: map[string]string{}, SandboxWrap: func(SandboxWrapRequest) ([]string, bool) { calls++; return nil, false }}.withDefaults()
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
	for _, mode := range []string{"off", " off "} {
		mux := &failedTerminalTmux{fakeTmux: &fakeTmux{}, err: errors.New("must not query")}
		path, err := sandboxTerminalPath(context.Background(), Deps{Tmux: mux, Env: map[string]string{envSandboxMode: mode}}, &Config{RequireSandbox: true}, "fixture")
		if path != "" || err != nil || mux.calls != 0 {
			t.Fatalf("mode %q: path=%q err=%v calls=%d", mode, path, err, mux.calls)
		}
	}
}
