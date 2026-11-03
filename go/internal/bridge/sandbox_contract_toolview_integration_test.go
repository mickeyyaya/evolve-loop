//go:build integration

package bridge

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const fakeGhScript = `#!/bin/sh
dir="${GH_CONFIG_DIR:-$HOME/.config/gh}"
echo "config-dir=$dir"
if [ -n "$GH_TOKEN$GITHUB_TOKEN" ] || [ -s "$dir/hosts.yml" ]; then
  echo "Logged in to github.com"
  exit 0
fi
echo "You are not logged into any GitHub hosts."
exit 1
`

type toolViewHost struct {
	home, workspace string
}

func newToolViewHost(t *testing.T) toolViewHost {
	t.Helper()
	h := toolViewHost{home: t.TempDir(), workspace: t.TempDir()}
	fixtures := map[string]string{
		".config/gh/hosts.yml": "github.com:\n    oauth_token: fixture-gh-token\n",
		".gitconfig":           "[credential]\n\thelper = store\n",
		".git-credentials":     "https://fixture:fixture-git-token@github.com\n",
		"system-gitconfig":     "[credential]\n\thelper = store\n",
		"bin/gh":               fakeGhScript,
	}
	for rel, body := range fixtures {
		path := filepath.Join(h.home, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", h.home)
	t.Setenv("PATH", filepath.Join(h.home, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_TOKEN", "fixture-launcher-token")
	return h
}

func (h toolViewHost) cell(t *testing.T, stdin string, name string, args ...string) (string, error) {
	t.Helper()
	cfg := seatConfig("claude-tmux")
	cfg.Workspace = h.workspace
	env, hermetic := Deps{EnvStage: "enforce"}.cellEnv(cfg)
	if !hermetic {
		t.Fatal("the contract runs at enforce")
	}
	return h.run(env, stdin, name, args...)
}

func (h toolViewHost) host(name string, args ...string) (string, error) {
	return h.run(os.Environ(), "", name, args...)
}

func (h toolViewHost) run(env []string, stdin string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = h.workspace
	cmd.Env = append(slices.Clone(env), "GIT_CONFIG_SYSTEM="+filepath.Join(h.home, "system-gitconfig"), "GIT_TRACE=1")
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestAgentSandboxContract_N8b_ACellGhCallSaysNotLoggedInAndReadsNoHostConfig(t *testing.T) {
	h := newToolViewHost(t)
	if out, err := h.host("gh", "auth", "status"); err != nil || !strings.Contains(out, "Logged in to github.com") {
		t.Fatalf("the host fixture must be logged in, or the row proves nothing: err=%v out=%q", err, out)
	}
	out, err := h.cell(t, "", "gh", "auth", "status")
	if err == nil || !strings.Contains(out, "You are not logged into any GitHub hosts.") {
		t.Fatalf("a cell gh call must say not logged in: err=%v out=%q", err, out)
	}
	if want := "config-dir=" + filepath.Join(h.workspace, ".cell", "gh"); !strings.Contains(out, want) {
		t.Fatalf("a cell gh call must read the empty cell config dir, not the host one: want %q in %q", want, out)
	}
	if strings.Contains(out, filepath.Join(h.home, ".config", "gh")) {
		t.Fatalf("a cell gh call read the host config: %q", out)
	}
}

func TestAgentSandboxContract_N8c_ACellGitHasNoCredentialHelper(t *testing.T) {
	h := newToolViewHost(t)
	showHelpers := []string{"config", "--show-origin", "--get-all", "credential.helper"}
	if out, err := h.host("git", showHelpers...); err != nil || !strings.Contains(out, "\tstore") {
		t.Fatalf("the host fixture must have the store helper, or the row proves nothing: err=%v out=%q", err, out)
	}
	out, _ := h.cell(t, "", "git", showHelpers...)
	if want := "file:" + filepath.Join(h.workspace, ".cell", "gitconfig") + "\t\n"; !strings.Contains(out, want) {
		t.Fatalf("a cell git must read only the empty helper of the cell gitconfig: want %q in %q", want, out)
	}
	if strings.Contains(out, "store") || strings.Contains(out, ".gitconfig") {
		t.Fatalf("a cell git has a host credential helper: %q", out)
	}
	out, err := h.cell(t, "protocol=https\nhost=github.com\n\n", "git", "credential", "fill")
	if err == nil || strings.Contains(out, "fixture-git-token") {
		t.Fatalf("a cell git credential fill must find no credential and prompt for none: err=%v out=%q", err, out)
	}
	trace, _ := h.cell(t, "", "git", "-c", "core.sshCommand=ssh -i "+filepath.Join(h.home, ".ssh", "id_fixture"), "ls-remote", "ssh://git@127.0.0.1:1/x")
	if !strings.Contains(trace, "IdentityFile=/dev/null") || !strings.Contains(trace, "BatchMode=yes") || strings.Contains(trace, "id_fixture") {
		t.Fatalf("a cell git ssh call must read no key: %q", trace)
	}
}
