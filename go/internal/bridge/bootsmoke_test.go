package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// bootSmokeDeps returns Deps wired for a deterministic boot smoke-test: the
// scripted fakeTmux, a no-op Sleep (boot/poll loops iterate instantly), and an
// empty env (so the cost-leak guards see no ANTHROPIC_* keys).
func bootSmokeDeps(tmux *fakeTmux) (Deps, *bytes.Buffer) {
	var stderr bytes.Buffer
	return Deps{
		Tmux:      tmux,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(nil),
		Stderr:    &stderr,
	}, &stderr
}

// TestBootSmokeTest_BootSuccess — when the REPL prompt marker appears, the boot
// smoke-test returns ExitOK, cleanly exits the REPL (/exit), and delivers NO
// task prompt (boot-only: resolved-prompt.txt is never written).
func TestBootSmokeTest_BootSuccess(t *testing.T) {
	ws := t.TempDir()
	tmux := &fakeTmux{paneSeq: []string{"❯"}} // ❯ marker on first capture
	deps, _ := bootSmokeDeps(tmux)
	rc, _ := BootSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws}, deps)
	if rc != ExitOK {
		t.Fatalf("rc = %d, want ExitOK (%d)", rc, ExitOK)
	}
	if !tmux.sentContains("/exit") {
		t.Errorf("boot smoke-test must cleanly exit the REPL (/exit); sent=%v", tmux.sentKeys)
	}
	if _, err := os.Stat(filepath.Join(ws, "resolved-prompt.txt")); !os.IsNotExist(err) {
		t.Errorf("boot-only must NOT write a resolved prompt; stat err=%v", err)
	}
}

// TestBootSmokeTest_BootTimeout — when the marker never appears, return
// ExitREPLBootTimeout and the captured pane scrollback (for diagnosis).
func TestBootSmokeTest_BootTimeout(t *testing.T) {
	ws := t.TempDir()
	tmux := &fakeTmux{paneSeq: []string{"booting... (no marker yet)"}}
	deps, _ := bootSmokeDeps(tmux)
	rc, scroll := BootSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws}, deps)
	if rc != ExitREPLBootTimeout {
		t.Fatalf("rc = %d, want ExitREPLBootTimeout (%d)", rc, ExitREPLBootTimeout)
	}
	if scroll == "" {
		t.Errorf("boot timeout should return the captured scrollback for diagnosis")
	}
}

// TestBootSmokeTest_UnknownDriver — an unregistered driver name is a usage
// error, not a boot attempt.
func TestBootSmokeTest_UnknownDriver(t *testing.T) {
	deps, _ := bootSmokeDeps(&fakeTmux{})
	if rc, _ := BootSmokeTest(context.Background(), "no-such-driver", &Config{Workspace: t.TempDir()}, deps); rc != ExitBadFlags {
		t.Errorf("rc = %d, want ExitBadFlags (%d) for an unknown driver", rc, ExitBadFlags)
	}
}

// TestBootSmokeTest_NonTmuxDriver — only the *-tmux drivers have a bootable REPL;
// a non-tmux driver (claude-p, headless) is rejected as a usage error.
func TestBootSmokeTest_NonTmuxDriver(t *testing.T) {
	deps, _ := bootSmokeDeps(&fakeTmux{})
	if rc, _ := BootSmokeTest(context.Background(), "claude-p", &Config{Workspace: t.TempDir()}, deps); rc != ExitBadFlags {
		t.Errorf("rc = %d, want ExitBadFlags (%d) for a non-tmux driver", rc, ExitBadFlags)
	}
}

// TestBootSmokeTest_SandboxPrefixApplied — with a worktree set and a sandbox
// wrapper available, the boot launch is sandbox-wrapped (the riskiest boot path
// the write-phases use) and still boots + exits cleanly.
func TestBootSmokeTest_SandboxPrefixApplied(t *testing.T) {
	ws := t.TempDir()
	wt := t.TempDir()
	tmux := &fakeTmux{paneSeq: []string{"❯"}}
	deps, _ := bootSmokeDeps(tmux)
	deps.SandboxWrap = func(req SandboxWrapRequest) ([]string, bool) {
		return []string{"sandbox-exec", "-f", "/tmp/x.sb"}, true
	}
	rc, _ := BootSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws, Worktree: wt, Agent: "build"}, deps)
	if rc != ExitOK {
		t.Fatalf("rc = %d, want ExitOK (%d)", rc, ExitOK)
	}
	if !tmux.sentContains("sandbox-exec") {
		t.Errorf("sandboxed boot must prepend the sandbox prefix to the launch cmd; sent=%v", tmux.sentKeys)
	}
}

func TestScrollbackTail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"empty", "", 3, ""},
		{"fewer than n", "a\nb", 5, "a\nb"},
		{"trims blank lines", "a\n\n  \nb\n\n", 3, "a\nb"},
		{"keeps last n non-empty", "1\n2\n3\n4\n5", 2, "4\n5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScrollbackTail(tc.in, tc.n); got != tc.want {
				t.Fatalf("ScrollbackTail(%q,%d)=%q want %q", tc.in, tc.n, got, tc.want)
			}
		})
	}
}
