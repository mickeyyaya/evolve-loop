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
	return liveSeatHomeAt(t, false, nil)
}

func liveSeatHomeAt(t *testing.T, symlinked bool, setup func(home string)) (home, token string, prefix []string) {
	t.Helper()
	probe := requireNativeSandbox(t)
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if symlinked {
		real := filepath.Join(resolved, "real")
		if err := os.MkdirAll(real, 0o700); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(resolved, "link")
		if err := os.Symlink(real, link); err != nil {
			t.Fatal(err)
		}
		resolved = link
	}
	t.Setenv("HOME", resolved)
	token = paths.SeatTokenPath(resolved)
	if err := os.MkdirAll(filepath.Dir(token), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(token, []byte(liveHolderCanary+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if setup != nil {
		setup(resolved)
	}
	req := SandboxWrapRequest{
		Phase: "itest", CLI: "claude-tmux", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
		HostIsolated: true, SeatRoot: paths.SeatRoot(resolved),
	}
	prefix, ok := defaultSandboxWrapWithProbe(Deps{SandboxMode: "on"}, func() sandbox.ProbeResult { return probe })(req)
	if !ok.Applied {
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
	t.Run("the stored login of a plain home", func(t *testing.T) { requireStoredLoginDenied(t, false) })
	t.Run("the stored login of a symlinked home", func(t *testing.T) { requireStoredLoginDenied(t, true) })
	t.Run("the holder", requireHolderDenied)
}

func requireHolderDenied(t *testing.T) {
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

const liveStoredLoginBody = "fixture-stored-login-not-real"

func plantStoredLogin(t *testing.T, home string) (credentials, claudeJSON string) {
	t.Helper()
	credentials = filepath.Join(home, ".claude", ".credentials.json")
	claudeJSON = filepath.Join(home, ".claude.json")
	if err := os.MkdirAll(filepath.Dir(credentials), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{credentials: liveStoredLoginBody, claudeJSON: liveStoredLoginBody} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return credentials, claudeJSON
}

func requireStoredLoginDenied(t *testing.T, symlinked bool) {
	t.Helper()
	var credentials, claudeJSON string
	_, _, prefix := liveSeatHomeAt(t, symlinked, func(home string) { credentials, claudeJSON = plantStoredLogin(t, home) })
	for _, target := range []string{credentials, claudeJSON} {
		for _, name := range pathSpellings(target) {
			for verb, script := range map[string]string{
				"read":      "cat " + name,
				"overwrite": "printf planted > " + name,
				"append":    "printf planted >> " + name,
				"remove":    "rm -f " + name,
			} {
				if out, err := runInSeat(t, prefix, script); err == nil {
					t.Errorf("%s of %s was allowed: %q", verb, name, out)
				}
				body, readErr := os.ReadFile(target)
				if readErr != nil || string(body) != liveStoredLoginBody {
					t.Fatalf("%s of %s changed the stored login: %v", verb, name, readErr)
				}
			}
		}
	}
}
