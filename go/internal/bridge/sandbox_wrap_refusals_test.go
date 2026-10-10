package bridge

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func lockedDir(t *testing.T) string {
	t.Helper()
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	return locked
}

func TestDefaultSandboxWrap_RefusesAProfileItCannotBuildSafely(t *testing.T) {
	for _, tc := range []struct {
		name  string
		os    string
		mkdir func(string, string) (string, error)
		req   func(t *testing.T) SandboxWrapRequest
		want  string
	}{
		{
			name: "a terminal that is not an assigned tty", os: "darwin",
			req: func(t *testing.T) SandboxWrapRequest {
				return SandboxWrapRequest{Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), TerminalPath: "/dev/null"}
			},
			want: "sandbox terminal unavailable",
		},
		{
			name: "a write grant that is not a path", os: "darwin",
			req: func(t *testing.T) SandboxWrapRequest {
				return SandboxWrapRequest{Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), WriteSubpaths: []string{""}}
			},
			want: "sandbox write grant unavailable",
		},
		{
			name: "a repository root the bridge cannot resolve", os: "darwin",
			req: func(t *testing.T) SandboxWrapRequest {
				return SandboxWrapRequest{Phase: "build", Workspace: t.TempDir(), RepoRoot: filepath.Join(lockedDir(t), "repo")}
			},
			want: "sandbox path resolution failed",
		},
		{
			name: "a Linux denial target that does not exist", os: "linux",
			req: func(t *testing.T) SandboxWrapRequest {
				return SandboxWrapRequest{Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), DenyPaths: []string{filepath.Join(t.TempDir(), "absent")}}
			},
			want: "Linux denial target",
		},
		{
			name: "a profile file the bridge cannot write", os: "darwin",
			mkdir: func(string, string) (string, error) { return filepath.Join(os.DevNull, "no-dir"), nil },
			req: func(t *testing.T) SandboxWrapRequest {
				return SandboxWrapRequest{Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir()}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			deps := Deps{Env: map[string]string{envSandboxMode: config.SandboxModeOn}, Stderr: &stderr, MkScratchDir: tc.mkdir}

			prefix, ok := defaultSandboxWrapWithProbe(deps, fakeProbe(tc.os, true))(tc.req(t))

			if ok || prefix != nil {
				t.Fatalf("the wrapper must refuse rather than confine the agent with a wrong profile; got %v", prefix)
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr = %q, want it to name %q", stderr.String(), tc.want)
			}
		})
	}
}
