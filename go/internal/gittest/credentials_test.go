package gittest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

const (
	plantedHostToken    = "gho_plantedHostLoginToken0123456789ab"
	ghNotLoggedInPhrase = "not logged into any GitHub hosts"
)

func unsetForTest(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
}

func ghAuthStatus(t *testing.T) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func plantHostsFileLogin(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	ghDir := filepath.Join(home, ".config", "gh")
	if err := os.MkdirAll(ghDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hosts := "github.com:\n    oauth_token: " + plantedHostToken + "\n    user: hostuser\n    git_protocol: https\n"
	if err := os.WriteFile(filepath.Join(ghDir, "hosts.yml"), []byte(hosts), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	unsetForTest(t, "XDG_CONFIG_HOME")
	unsetForTest(t, "GH_CONFIG_DIR")
	unsetForTest(t, "GH_TOKEN")
	unsetForTest(t, "GITHUB_TOKEN")
}

func plantTokenVariableLogin(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	unsetForTest(t, "XDG_CONFIG_HOME")
	unsetForTest(t, "GH_CONFIG_DIR")
	t.Setenv("GH_TOKEN", plantedHostToken)
	t.Setenv("GITHUB_TOKEN", plantedHostToken)
}

func TestIsolateHostCredentials_ARealGhInATestSeesNoLogin(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh is not on PATH")
	}
	for name, plant := range map[string]func(*testing.T){
		"hosts file":     plantHostsFileLogin,
		"token variable": plantTokenVariableLogin,
	} {
		t.Run(name, func(t *testing.T) {
			plant(t)
			before, _ := ghAuthStatus(t)
			if strings.Contains(before, ghNotLoggedInPhrase) {
				t.Fatalf("the planted host login is invisible to gh before isolation, so the test proves nothing:\n%s", before)
			}

			restore, err := IsolateHostCredentials()
			if err != nil {
				t.Fatalf("IsolateHostCredentials: %v", err)
			}
			t.Cleanup(func() {
				if err := restore(); err != nil {
					t.Errorf("restore: %v", err)
				}
			})

			after, err := ghAuthStatus(t)
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("gh auth status after isolation: err = %v, want a non-zero exit\n%s", err, after)
			}
			if !strings.Contains(after, ghNotLoggedInPhrase) {
				t.Errorf("gh auth status after isolation does not say %q:\n%s", ghNotLoggedInPhrase, after)
			}
			if strings.Contains(after, plantedHostToken) {
				t.Errorf("the host token leaked into gh output after isolation:\n%s", after)
			}
		})
	}
}

func gitConfigGet(t *testing.T, key string) (string, error) {
	t.Helper()
	out, err := exec.Command("git", "config", "--get", key).Output()
	return strings.TrimSpace(string(out)), err
}

func TestIsolateHostCredentials_SetsTheHermeticValuesAndDropsHostTokens(t *testing.T) {
	home := t.TempDir()
	hostGitConfig := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(hostGitConfig, []byte("[credential]\n\thelper = hostcredhelper\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	unsetForTest(t, "GIT_CONFIG_GLOBAL")
	unsetForTest(t, "GIT_CONFIG_NOSYSTEM")
	t.Setenv("GH_TOKEN", plantedHostToken)
	t.Setenv("GITHUB_TOKEN", plantedHostToken)
	if got, _ := gitConfigGet(t, "credential.helper"); !strings.Contains(got, "hostcredhelper") {
		t.Fatalf("precondition: git sees credential.helper = %q, want the planted host helper", got)
	}

	restore, err := IsolateHostCredentials()
	if err != nil {
		t.Fatalf("IsolateHostCredentials: %v", err)
	}
	t.Cleanup(func() {
		if err := restore(); err != nil {
			t.Errorf("restore: %v", err)
		}
	})

	if got := os.Getenv("GIT_CONFIG_GLOBAL"); got != os.DevNull {
		t.Errorf("GIT_CONFIG_GLOBAL = %q, want %q", got, os.DevNull)
	}
	if got := os.Getenv("GIT_CONFIG_NOSYSTEM"); got != "1" {
		t.Errorf("GIT_CONFIG_NOSYSTEM = %q, want 1", got)
	}
	if got := os.Getenv("GIT_TERMINAL_PROMPT"); got != "0" {
		t.Errorf("GIT_TERMINAL_PROMPT = %q, want 0", got)
	}
	if got := os.Getenv("GIT_SSH_COMMAND"); !strings.Contains(got, "BatchMode=yes") {
		t.Errorf("GIT_SSH_COMMAND = %q, want a command that never prompts (BatchMode=yes)", got)
	}
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v, ok := os.LookupEnv(key); ok {
			t.Errorf("%s is still set (%q) after isolation", key, v)
		}
	}
	configDir := os.Getenv("GH_CONFIG_DIR")
	if configDir == "" {
		t.Fatal("GH_CONFIG_DIR is empty after isolation")
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		t.Fatalf("GH_CONFIG_DIR %s: %v", configDir, err)
	}
	if len(entries) != 0 {
		t.Errorf("GH_CONFIG_DIR %s holds %d entries, want an empty directory", configDir, len(entries))
	}
	if filepath.Dir(configDir) == filepath.Join(home, ".config") {
		t.Errorf("GH_CONFIG_DIR %s is the host config location", configDir)
	}
	if got, err := gitConfigGet(t, "credential.helper"); err == nil || got != "" {
		t.Errorf("git still sees credential.helper = %q (err %v) after isolation", got, err)
	}
}

func TestIsolateHostCredentials_RestoreReinstatesThePriorEnvironment(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/prior/gitconfig")
	t.Setenv("GH_TOKEN", "prior-token")
	unsetForTest(t, "GIT_SSH_COMMAND")
	unsetForTest(t, "GH_CONFIG_DIR")

	restore, err := IsolateHostCredentials()
	if err != nil {
		t.Fatalf("IsolateHostCredentials: %v", err)
	}
	configDir := os.Getenv("GH_CONFIG_DIR")
	if _, err := os.Stat(configDir); err != nil {
		t.Fatalf("GH_CONFIG_DIR %s does not exist while isolated: %v", configDir, err)
	}
	if err := restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}

	if got := os.Getenv("GIT_CONFIG_GLOBAL"); got != "/prior/gitconfig" {
		t.Errorf("GIT_CONFIG_GLOBAL = %q after restore, want the prior value", got)
	}
	if got := os.Getenv("GH_TOKEN"); got != "prior-token" {
		t.Errorf("GH_TOKEN = %q after restore, want the prior value", got)
	}
	for _, key := range []string{"GIT_SSH_COMMAND", "GH_CONFIG_DIR"} {
		if v, ok := os.LookupEnv(key); ok {
			t.Errorf("%s = %q after restore, want it unset as before", key, v)
		}
	}
	if _, err := os.Stat(configDir); !os.IsNotExist(err) {
		t.Errorf("GH_CONFIG_DIR %s survives restore (stat err %v)", configDir, err)
	}
}

func TestIsolateHostCredentials_AConfigDirFailureIsReportedAndChangesNothing(t *testing.T) {
	boom := errors.New("disk full")
	saved := mkdirTemp
	mkdirTemp = func(string, string) (string, error) { return "", boom }
	t.Cleanup(func() { mkdirTemp = saved })
	t.Setenv("GIT_CONFIG_GLOBAL", "/prior/gitconfig")
	unsetForTest(t, "GH_CONFIG_DIR")

	restore, err := IsolateHostCredentials()

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap %v", err, boom)
	}
	if restore != nil {
		t.Error("restore is non-nil after a failure, want nil")
	}
	if got := os.Getenv("GIT_CONFIG_GLOBAL"); got != "/prior/gitconfig" {
		t.Errorf("GIT_CONFIG_GLOBAL = %q after a failure, want the prior value", got)
	}
	if v, ok := os.LookupEnv("GH_CONFIG_DIR"); ok {
		t.Errorf("GH_CONFIG_DIR = %q after a failure, want it unset", v)
	}
}

func TestIsolateHostCredentials_AFailedRemovalIsReportedAndTheEnvironmentIsStillRestored(t *testing.T) {
	boom := errors.New("directory is busy")
	saved := removeAll
	removeAll = func(string) error { return boom }
	t.Cleanup(func() { removeAll = saved })
	t.Setenv("GIT_CONFIG_GLOBAL", "/prior/gitconfig")
	unsetForTest(t, "GH_CONFIG_DIR")

	restore, err := IsolateHostCredentials()
	if err != nil {
		t.Fatalf("IsolateHostCredentials: %v", err)
	}
	configDir := os.Getenv("GH_CONFIG_DIR")
	t.Cleanup(func() {
		if err := os.RemoveAll(configDir); err != nil {
			t.Errorf("remove %s: %v", configDir, err)
		}
	})

	if err := restore(); !errors.Is(err, boom) {
		t.Fatalf("restore err = %v, want it to wrap %v", err, boom)
	}
	if got := os.Getenv("GIT_CONFIG_GLOBAL"); got != "/prior/gitconfig" {
		t.Errorf("GIT_CONFIG_GLOBAL = %q after a failed removal, want the prior value", got)
	}
	if v, ok := os.LookupEnv("GH_CONFIG_DIR"); ok {
		t.Errorf("GH_CONFIG_DIR = %q after a failed removal, want it unset", v)
	}
}

func TestCredentialIsolation_IsTheOneSet(t *testing.T) {
	restore, err := IsolateHostCredentials()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restore() })
	want := ipcenv.CredentialIsolation(os.Getenv("GH_CONFIG_DIR"), os.DevNull)
	for k, v := range want.Set {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q from ipcenv.CredentialIsolation", k, got, v)
		}
	}
	for _, k := range want.Unset {
		if v, ok := os.LookupEnv(k); ok {
			t.Errorf("%s is set (%q), want it unset", k, v)
		}
	}
}

func TestIsolateHostCredentials_AFixtureKeepsItsRepoLocalIdentity(t *testing.T) {
	restore, err := IsolateHostCredentials()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restore() })
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		if v, ok := os.LookupEnv(k); ok {
			t.Errorf("%s = %q beats the repo-local identity", k, v)
		}
	}
	r := Fixture(t)
	r.Write("f", "x")
	r.Git("add", "f")
	r.Git("commit", "-q", "-m", "work")
	if got := r.Git("log", "-1", "--format=%an <%ae>|%cn <%ce>"); got != "gittest <gittest@example.com>|gittest <gittest@example.com>" {
		t.Errorf("identity = %q, want the repo-local one", got)
	}
}
