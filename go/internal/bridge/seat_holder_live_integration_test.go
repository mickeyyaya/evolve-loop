//go:build integration && darwin

package bridge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const liveHolderCanary = "sk-ant-oat01-LIVE-HOLDER-CANARY"

func liveSeatHome(t *testing.T) (home, token string, prefix []string) {
	t.Helper()
	probe := requireNativeSandbox(t)
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", resolved)
	token = paths.SeatTokenPath(resolved)
	if err := os.MkdirAll(filepath.Dir(token), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(token, []byte(liveHolderCanary+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := SandboxWrapRequest{
		Phase: "itest", CLI: "claude-tmux", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
		HostIsolated: true, SeatRoot: paths.SeatRoot(resolved),
	}
	prefix, ok := defaultSandboxWrapWithProbe(Deps{SandboxMode: "on"}, func() sandbox.ProbeResult { return probe })(req)
	if !ok {
		t.Fatal("the wrap was refused")
	}
	return resolved, token, prefix
}

func runInSeat(t *testing.T, prefix []string, script string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	args := append(append([]string{}, prefix[1:]...), "/bin/sh", "-c", script)
	out, err := exec.CommandContext(ctx, prefix[0], args...).CombinedOutput()
	return string(out), err
}

func TestSeatHolder_ALiveSeatCannotReadOrChangeTheHolder(t *testing.T) {
	home, token, prefix := liveSeatHome(t)
	holder := filepath.Dir(token)
	root := paths.SeatRoot(home)
	for name, script := range map[string]string{
		"read":              "cat " + token,
		"chmod":             "chmod 644 " + token,
		"overwrite":         "printf planted > " + token,
		"append":            "printf planted >> " + token,
		"remove":            "rm -f " + token,
		"rename the token":  "mv " + token + " " + token + ".moved",
		"plant a sibling":   "printf planted > " + holder + "/other",
		"rename the holder": "mv " + holder + " " + root + "/moved",
		"rename the root":   "mv " + root + " " + root + ".moved",
		"list the holder":   "ls " + holder,
	} {
		out, err := runInSeat(t, prefix, script)
		if err == nil {
			t.Errorf("%s: the seat was allowed: %q", name, out)
		}
		body, readErr := os.ReadFile(token)
		if readErr != nil || string(body) != liveHolderCanary+"\n" {
			t.Fatalf("%s changed the holder: err=%v", name, readErr)
		}
	}
	info, err := os.Stat(token)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the holder mode changed: %v %v", info, err)
	}
}

func TestSeatHolder_ALiveSeatCanWriteItsOwnDirectory(t *testing.T) {
	home, _, prefix := liveSeatHome(t)
	target := filepath.Join(paths.SeatRoot(home), "projects", "a-transcript.jsonl")
	out, err := runInSeat(t, prefix, "mkdir -p "+filepath.Dir(target)+" && printf ok > "+target)
	if err != nil {
		t.Fatalf("the seat must write its own directory: %v: %q", err, out)
	}
	if body, err := os.ReadFile(target); err != nil || string(body) != "ok" {
		t.Fatalf("got %q %v", body, err)
	}
}
