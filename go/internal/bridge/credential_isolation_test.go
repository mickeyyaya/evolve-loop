package bridge

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

const (
	hostIdentityName  = "Fake Host"
	hostIdentityEmail = "fake-host@example.test"
)

func fakeHostGit(calls *[]string) func(string) string {
	return func(key string) string {
		if calls != nil {
			*calls = append(*calls, key)
		}
		switch key {
		case "user.name":
			return hostIdentityName
		case "user.email":
			return hostIdentityEmail
		}
		return ""
	}
}

func lastEnvValue(env []string, key string) (string, bool) {
	val, found := "", false
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			val, found = v, true
		}
	}
	return val, found
}

func isolationParent(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "y")
	t.Setenv("GIT_CONFIG_GLOBAL", "/host/gitconfig")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "0")
	t.Setenv("GH_CONFIG_DIR", "/host/gh")
}

func assertIsolated(t *testing.T, where string, env []string, ghDir, gitCfg string) {
	t.Helper()
	want := ipcenv.CredentialIsolation(ghDir, gitCfg)
	for k, v := range want.Set {
		if got, ok := lastEnvValue(env, k); !ok || got != v {
			t.Errorf("%s: %s = %q (set %v), want %q", where, k, got, ok, v)
		}
	}
	for _, k := range want.Unset {
		if v, ok := lastEnvValue(env, k); ok && v != "" {
			t.Errorf("%s: %s = %q, want it absent", where, k, v)
		}
	}
}

func isolationConfig(t *testing.T) *Config {
	t.Helper()
	return &Config{CLI: "claude-tmux", Agent: "build", Workspace: t.TempDir(), Realization: RealizeFor("claude-tmux", LaunchIntent{})}
}

func TestCellEnv_IsolatesHostCredentialsOnEachCarrier(t *testing.T) {
	t.Setenv("GH_TOKEN", "x")
	isolationParent(t)
	for _, stage := range []string{"off", "shadow", "enforce"} {
		cfg := isolationConfig(t)
		ghDir := filepath.Join(cfg.Workspace, "gh-config")
		gitCfg := filepath.Join(cfg.Workspace, "gitconfig")
		deps := Deps{EnvStage: stage, Stderr: io.Discard, HostGitConfig: fakeHostGit(nil)}
		env, _ := deps.cellEnv(cfg)
		assertIsolated(t, "cellEnv "+stage, env, ghDir, gitCfg)
		assertIsolated(t, "headless "+stage, dispatchEnv(deps, cfg), ghDir, gitCfg)
		if fi, err := os.Stat(ghDir); err != nil || !fi.IsDir() {
			t.Errorf("%s: the gh config dir %s must be an empty dir of the workspace: %v", stage, ghDir, err)
		}
	}

	seatCfg := isolationConfig(t)
	seatCfg.Decision.SeatToken = "tok"
	seatEnv, hermetic := Deps{EnvStage: "off", Stderr: io.Discard, HostGitConfig: fakeHostGit(nil)}.cellEnv(seatCfg)
	if !hermetic {
		t.Fatal("a seat dispatch runs at enforce")
	}
	assertIsolated(t, "seat dispatch", seatEnv, filepath.Join(seatCfg.Workspace, "gh-config"), filepath.Join(seatCfg.Workspace, "gitconfig"))

	tm := &fakeTmux{}
	paneCfg := isolationConfig(t)
	if err := primePaneShell(context.Background(), Deps{Tmux: tm, Stderr: io.Discard, HostGitConfig: fakeHostGit(nil)}, paneCfg, "s", "/wt"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"export GIT_CONFIG_GLOBAL=" + shellQuotePOSIX(filepath.Join(paneCfg.Workspace, "gitconfig")), "export GIT_CONFIG_NOSYSTEM=1", "export GIT_TERMINAL_PROMPT=0",
		"export GIT_SSH_COMMAND='ssh -o BatchMode=yes'", "export GH_CONFIG_DIR=" + shellQuotePOSIX(filepath.Join(paneCfg.Workspace, "gh-config")),
		"unset GH_TOKEN", "unset GITHUB_TOKEN",
	} {
		if !slices.Contains(tm.sentKeys, want) {
			t.Errorf("the pane keys lack %q: %q", want, tm.sentKeys)
		}
	}
	marker := slices.Index(tm.sentKeys, "export "+ipcenv.PhaseKey+"=build")
	if marker < 0 || slices.Index(tm.sentKeys, "unset GH_TOKEN") < marker {
		t.Errorf("the credential lines follow the cell markers: %q", tm.sentKeys)
	}
	for _, k := range ipcenv.CellMarkers() {
		if _, ok := ipcenv.CredentialIsolation("x", "y").Set[k]; ok {
			t.Errorf("%s is in the marker registry", k)
		}
	}
}

func TestCellEnv_NoWorkspaceGivesTheGhConfigNoDirAndNoIdentityFile(t *testing.T) {
	t.Setenv("GH_TOKEN", "x")
	isolationParent(t)
	var calls []string
	env, _ := Deps{EnvStage: "shadow", Stderr: io.Discard, HostGitConfig: fakeHostGit(&calls)}.cellEnv(&Config{CLI: "claude-tmux", Agent: "probe"})
	assertIsolated(t, "no workspace", env, os.DevNull, os.DevNull)
	if len(calls) != 0 {
		t.Errorf("a cell with no workspace has no file to hold an identity, yet git config was read: %v", calls)
	}
}

func TestCellEnv_AnUnreadableHostGitWritesNoUserSection(t *testing.T) {
	t.Parallel()
	cfg := isolationConfig(t)
	env, _ := Deps{EnvStage: "shadow", Stderr: io.Discard, HostGitConfig: func(string) string { return "" }}.cellEnv(cfg)
	assertIsolated(t, "no identity", env, filepath.Join(cfg.Workspace, "gh-config"), os.DevNull)
	if _, err := os.Stat(filepath.Join(cfg.Workspace, "gitconfig")); err == nil {
		t.Error("an empty identity must write no gitconfig file")
	}
}

func TestCellEnv_AnUnwritableWorkspaceWarnsAndFallsBackToTheNullConfigs(t *testing.T) {
	t.Parallel()
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	env, _ := Deps{EnvStage: "shadow", Stderr: &stderr, HostGitConfig: fakeHostGit(nil)}.cellEnv(&Config{CLI: "claude-tmux", Agent: "probe", Workspace: blocked})
	assertIsolated(t, "unwritable workspace", env, filepath.Join(blocked, "gh-config"), os.DevNull)
	for _, want := range []string{"gh config dir", "git config"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("a failed write must warn about %q: %q", want, stderr.String())
		}
	}
	env, _ = Deps{EnvStage: "shadow", HostGitConfig: fakeHostGit(nil)}.cellEnv(&Config{CLI: "claude-tmux", Agent: "probe", Workspace: blocked})
	assertIsolated(t, "unwritable workspace without a log", env, filepath.Join(blocked, "gh-config"), os.DevNull)
}

func TestDeps_HostGitConfigNilResolvesToTheRealReader(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Real Host\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	if got := (Deps{}).hostGitConfig("user.name"); got != "Real Host" {
		t.Errorf("the nil reader gave %q", got)
	}
	if got := (Deps{HostGitConfig: func(string) string { return "fake" }}).hostGitConfig("user.name"); got != "fake" {
		t.Errorf("the set reader gave %q", got)
	}
}

func TestCellEnv_ReadsTheHostIdentityOncePerLaunch(t *testing.T) {
	t.Parallel()
	cfg := isolationConfig(t)
	var calls []string
	deps := Deps{EnvStage: "enforce", Stderr: io.Discard, Tmux: &fakeTmux{}, HostGitConfig: fakeHostGit(&calls)}
	deps.cellEnv(cfg)
	dispatchEnv(deps, cfg)
	if err := primePaneShell(context.Background(), deps, cfg, "s", "/wt"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []string{"user.name", "user.email"}) {
		t.Errorf("the host identity must be read once for the launch: %v", calls)
	}
}

func TestCellEnv_TheIdentityFileHoldsTheUserSectionAndNoCredentialKey(t *testing.T) {
	t.Parallel()
	cfg := isolationConfig(t)
	Deps{EnvStage: "enforce", Stderr: io.Discard, HostGitConfig: fakeHostGit(nil)}.cellEnv(cfg)
	body, err := os.ReadFile(filepath.Join(cfg.Workspace, "gitconfig"))
	if err != nil {
		t.Fatal(err)
	}
	want := "[user]\n\tname = \"" + hostIdentityName + "\"\n\temail = \"" + hostIdentityEmail + "\"\n"
	if string(body) != want {
		t.Errorf("gitconfig = %q, want %q", body, want)
	}
	if strings.Contains(string(body), "credential") {
		t.Error("the cell gitconfig holds a credential key")
	}
}

func TestReadHostGitConfig_ReadsTheHostConfigAndAMissingGitGivesNothing(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Real Host\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	if _, err := exec.LookPath("git"); err == nil {
		if got := readHostGitConfig("user.name"); got != "Real Host" {
			t.Errorf("user.name = %q", got)
		}
		if got := readHostGitConfig("user.nosuchkey"); got != "" {
			t.Errorf("an unset key must read as empty, got %q", got)
		}
	}
	t.Setenv("PATH", t.TempDir())
	if got := readHostGitConfig("user.name"); got != "" {
		t.Errorf("a missing git must read as empty, got %q", got)
	}
}

func gitCmd(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = dir, env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCellEnv_BuilderCommitGetsTheHostIdentityAndARepoLocalIdentityWins(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	isolationParent(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	hostLine := hostIdentityName + " <" + hostIdentityEmail + ">"
	for _, stage := range []string{"off", "shadow", "enforce"} {
		cfg := isolationConfig(t)
		deps := Deps{EnvStage: stage, Stderr: io.Discard, HostGitConfig: fakeHostGit(nil)}
		env, _ := deps.cellEnv(cfg)
		wtRepo := gittest.Fixture(t)
		wt := wtRepo.Dir
		gitCmd(t, wt, env, "config", "--local", "--unset", "user.name")
		gitCmd(t, wt, env, "config", "--local", "--unset", "user.email")
		wtRepo.Write("f", "x")
		gitCmd(t, wt, env, "add", "f")
		gitCmd(t, wt, env, "commit", "-q", "-m", "work")
		if got := gitCmd(t, wt, env, "log", "-1", "--format=%an <%ae>|%cn <%ce>"); got != hostLine+"|"+hostLine {
			t.Errorf("%s: the worktree commit identity = %q, want the host identity", stage, got)
		}

		fixture := gittest.Fixture(t)
		fixture.Write("g", "y")
		gitCmd(t, fixture.Dir, env, "add", "g")
		gitCmd(t, fixture.Dir, env, "commit", "-q", "-m", "fixture work")
		if got := gitCmd(t, fixture.Dir, env, "log", "-1", "--format=%an <%ae>|%cn <%ce>"); got != "gittest <gittest@example.com>|gittest <gittest@example.com>" {
			t.Errorf("%s: the fixture commit identity = %q, want the repo-local one", stage, got)
		}
	}
}

func TestCellEnv_DropsEachUnsetKeyFromEverySource(t *testing.T) {
	isolationParent(t)
	t.Setenv("GH_TOKEN", "parent")
	for _, stage := range []string{"off", "shadow", "enforce"} {
		cfg := isolationConfig(t)
		cfg.Realization.Env = map[string]string{"GH_TOKEN": "manifest", "GITHUB_TOKEN": "manifest", "KEEP": "1"}
		deps := Deps{EnvStage: stage, Stderr: io.Discard, HostGitConfig: fakeHostGit(nil), Env: map[string]string{"GH_TOKEN": "pipeline", "GITHUB_TOKEN": "pipeline"}}
		env, _ := deps.cellEnv(cfg)
		for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
			if v, ok := lastEnvValue(env, k); ok {
				t.Errorf("%s: %s = %q reached the child", stage, k, v)
			}
		}
		if v, _ := lastEnvValue(env, "KEEP"); v != "1" {
			t.Errorf("%s: an unrelated manifest key must stay: %q", stage, v)
		}
	}
	got := hermeticEnv(nil, nil, map[string]string{"GH_TOKEN": "m"}, map[string]string{"GH_TOKEN": "p"}, ipcenv.CredentialIsolation("g", "c"))
	if _, ok := lastEnvValue(got, "GH_TOKEN"); ok {
		t.Errorf("hermeticEnv kept GH_TOKEN: %q", got)
	}
}

func TestCellEnv_GitFindsNoCredentialHelperInTheCell(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	challenged := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		challenged++
		w.Header().Set("WWW-Authenticate", `Basic realm="x"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	home := t.TempDir()
	marker := filepath.Join(home, "helper-started")
	stub := filepath.Join(home, "stub-helper")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho started > "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[credential]\n\thelper = "+stub+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GH_TOKEN", "x")
	url := server.URL + "/x/y"

	control := exec.Command("git", "ls-remote", url)
	control.Env = os.Environ()
	_ = control.Run()
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("the control run must start the helper, so the proof means something: %v (challenges %d)", err, challenged)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}

	cfg := isolationConfig(t)
	env, _ := Deps{EnvStage: "enforce", Stderr: io.Discard}.cellEnv(cfg)
	cell := exec.Command("git", "ls-remote", url)
	cell.Env = env
	_ = cell.Run()
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the credential helper started inside the cell")
	}
}

func TestCredentialIsolation_IsTheOneSet(t *testing.T) {
	t.Setenv("GH_TOKEN", "x")
	isolationParent(t)
	restore, err := gittest.IsolateHostCredentials()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restore() })
	cfg := isolationConfig(t)
	env, _ := Deps{EnvStage: "enforce", Stderr: io.Discard, HostGitConfig: fakeHostGit(nil)}.cellEnv(cfg)
	want := ipcenv.CredentialIsolation("", os.DevNull)
	for k, v := range want.Set {
		if k == "GH_CONFIG_DIR" || k == "GIT_CONFIG_GLOBAL" {
			continue
		}
		cell, _ := lastEnvValue(env, k)
		if cell != v || os.Getenv(k) != v {
			t.Errorf("%s: cell %q, gittest %q, want %q in both", k, cell, os.Getenv(k), v)
		}
	}
	for _, k := range want.Unset {
		if _, ok := lastEnvValue(env, k); ok {
			t.Errorf("cell env holds %s", k)
		}
		if _, ok := os.LookupEnv(k); ok {
			t.Errorf("gittest left %s set", k)
		}
	}
}
