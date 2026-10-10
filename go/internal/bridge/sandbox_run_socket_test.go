package bridge

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func wrapDarwinBuild(t *testing.T, stderr io.Writer) ([]string, bool) {
	t.Helper()
	deps := Deps{Env: map[string]string{envSandboxMode: config.SandboxModeOn}, Stderr: stderr}
	return defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))(SandboxWrapRequest{
		Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(), AllowNetwork: true,
	})
}

func readSBPL(t *testing.T, prefix []string, ok bool) string {
	t.Helper()
	if !ok {
		t.Fatal("sandbox wrapper did not return a prefix")
	}
	raw, err := os.ReadFile(prefix[2])
	if err != nil {
		t.Fatalf("read SBPL: %v", err)
	}
	return string(raw)
}

func TestDefaultSandboxWrap_GuardsTheRunAndDefaultTmuxSockets(t *testing.T) {
	tmpdir := t.TempDir()
	t.Setenv("TMUX_TMPDIR", tmpdir)
	t.Setenv(TmuxSocketEnv, "evolve-bridge-p4242")
	real, err := filepath.EvalSymlinks(tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(real, fmt.Sprintf("tmux-%d", os.Getuid()))
	run, def := filepath.Join(dir, "evolve-bridge-p4242"), filepath.Join(dir, "default")

	prefix, ok := wrapDarwinBuild(t, &bytes.Buffer{})
	sbpl := readSBPL(t, prefix, ok)

	for _, rule := range []string{
		fmt.Sprintf("(deny network-outbound (remote unix-socket (path-literal %q)))", run),
		fmt.Sprintf("(deny network-outbound (remote unix-socket (path-literal %q)))", def),
		fmt.Sprintf("(deny file-write* (literal %q))", dir),
		fmt.Sprintf("(deny file-write* (literal %q))", run),
		fmt.Sprintf("(deny file-write* (literal %q))", def),
	} {
		if !strings.Contains(sbpl, rule) {
			t.Fatalf("the agent profile must carry %s at the resolved socket paths:\n%s", rule, sbpl)
		}
	}
	if strings.Contains(sbpl, fmt.Sprintf("(deny file-write* (subpath %q))", dir)) {
		t.Fatalf("the socket directory must stay writable below it, so the agent can make its own -L sockets:\n%s", sbpl)
	}
}

func TestDefaultSandboxWrap_RefusesASocketGuardItCannotResolve(t *testing.T) {
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	for _, tc := range []struct {
		name, tmpdir, want string
		stderr             io.Writer
	}{
		{"an unreadable socket directory", filepath.Join(locked, "inner"), fmt.Sprintf("inner/tmux-%d", os.Getuid()), &bytes.Buffer{}},
		{"a relative TMUX_TMPDIR", "relative/tmux", "is not absolute", &bytes.Buffer{}},
		{"without a diagnostic sink", "relative/tmux", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TMUX_TMPDIR", tc.tmpdir)

			prefix, ok := wrapDarwinBuild(t, tc.stderr)

			if ok || prefix != nil {
				t.Fatalf("a socket guard the bridge cannot resolve must refuse the wrap, not confine the agent without it; got %v", prefix)
			}
			if buf, isBuf := tc.stderr.(*bytes.Buffer); isBuf && (!strings.Contains(buf.String(), "tmux socket guard unavailable") || !strings.Contains(buf.String(), tc.want)) {
				t.Fatalf("stderr = %q, want the guard refusal naming %q", buf.String(), tc.want)
			}
		})
	}
}

func TestOSSandboxPrefix_AnUnknownSystemGetsNoPrefix(t *testing.T) {
	prefix, ok := osSandboxPrefix("plan9", Deps{}, SandboxWrapRequest{Phase: "build"}, sandbox.Config{})

	if ok || prefix != nil {
		t.Fatalf("osSandboxPrefix(plan9) = (%v, %v), want no prefix", prefix, ok)
	}
}
