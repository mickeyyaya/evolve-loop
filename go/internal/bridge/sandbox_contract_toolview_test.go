//go:build integration && darwin && !notmux

package bridge

import (
	"context"
	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	fakeGhHost       = "fake-gh-host.example.test"
	fakeHelperMarker = "fake-credential-helper-marker"
	fakeXDGMarker    = "fake-xdg-helper-marker"
)

func toolViewHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".config/gh/hosts.yml", fakeGhHost+":\n    user: fakeuser\n    oauth_token: fake-not-a-token\n")
	write(".gitconfig", "[credential]\n\thelper = "+fakeHelperMarker+"\n[user]\n\tname = Fake Home\n")
	write(".config/git/config", "[credential]\n\thelper = "+fakeXDGMarker+"\n")
	return home
}

func (h contractHost) toolViewEnv(t *testing.T, home string) []string {
	t.Helper()
	cfg := &Config{CLI: "claude-tmux", Agent: "builder", Workspace: h.run, Realization: RealizeFor("claude-tmux", LaunchIntent{})}
	env, hermetic := Deps{EnvStage: "enforce", Env: map[string]string{}, Stderr: io.Discard, HostGitConfig: fakeHostGit(nil)}.cellEnv(cfg)
	if !hermetic {
		t.Fatal("the cell env must be hermetic at enforce")
	}
	return env
}

func (h contractHost) inCell(t *testing.T, env []string, script string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := append(append([]string{}, h.prefix[1:]...), "/bin/sh", "-c", script)
	cmd := exec.CommandContext(ctx, h.prefix[0], args...)
	cmd.WaitDelay = contractAgentWaitDelay
	cmd.Dir = h.worktree
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("inconclusive: the cell command timed out: %q", out)
	}
	return strings.TrimSpace(string(out)), err
}

func controlEnv(home string) []string {
	return []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home}
}

func TestAgentSandboxContract_N8b_ACellGhCallSaysNotLoggedInAndReadsNoHostConfig(t *testing.T) {
	testhost.Tool(t, "gh")
	h := newContractHost(t)
	home := toolViewHome(t)
	t.Setenv("HOME", home)
	control, _ := h.inCell(t, controlEnv(home), "gh auth status 2>&1")
	if !strings.Contains(control, fakeGhHost) {
		t.Fatalf("the control run must read the fake host config, so the proof means something: %q", control)
	}
	out, err := h.inCell(t, h.toolViewEnv(t, home), "gh auth status 2>&1")
	if err == nil || !strings.Contains(out, "not logged") {
		t.Fatalf("a cell gh call must say not logged in and fail: err=%v out=%q", err, out)
	}
	if strings.Contains(out, fakeGhHost) {
		t.Fatalf("a cell gh call read the host gh config: %q", out)
	}
}

func TestAgentSandboxContract_N8c_ACellGitHasNoCredentialHelper(t *testing.T) {
	testhost.Tool(t, "git")
	h := newContractHost(t)
	home := toolViewHome(t)
	t.Setenv("HOME", home)
	control, _ := h.inCell(t, controlEnv(home), "git config --get-all credential.helper 2>&1")
	if !strings.Contains(control, fakeHelperMarker) {
		t.Fatalf("the control run must see the fake helper, so the proof means something: %q", control)
	}
	env := h.toolViewEnv(t, home)
	global, err := h.inCell(t, env, "git config --global --list 2>&1")
	if err != nil {
		t.Fatalf("git config --global --list: %v: %q", err, global)
	}
	for _, line := range strings.Split(global, "\n") {
		if !strings.HasPrefix(line, "user.name=") && !strings.HasPrefix(line, "user.email=") {
			t.Errorf("the global config of a cell holds %q, want only the identity", line)
		}
	}
	if !strings.Contains(global, "user.name="+hostIdentityName) {
		t.Errorf("the global config of a cell must be the cell config: %q", global)
	}
	all, _ := h.inCell(t, env, "git config --show-origin --list 2>&1")
	for _, bad := range []string{fakeHelperMarker, fakeXDGMarker, home} {
		if strings.Contains(all, bad) {
			t.Errorf("a cell git config lists %q: %q", bad, all)
		}
	}
	helpers, herr := h.inCell(t, env, "git config --get-all credential.helper 2>&1")
	if herr == nil || helpers != "" {
		t.Errorf("a cell git has no credential helper: err=%v out=%q", herr, helpers)
	}
}
