package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func rawConfig(t *testing.T, dir, key string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "config", "--local", "--get", key).Output()
	if err != nil {
		t.Fatalf("git config --local --get %s in %s: %v", key, dir, err)
	}
	return strings.TrimSpace(string(out))
}

func assertPersistsMaintenanceConfig(t *testing.T, dir string) {
	t.Helper()
	for _, kv := range MaintenanceConfig() {
		if got := rawConfig(t, dir, kv[0]); got != kv[1] {
			t.Errorf("%s: %s = %q in the repo's own config, want %q", dir, kv[0], got, kv[1])
		}
	}
}

// TestFixture_CommitsWithNoAmbientIdentity names Fixture, Repo, Repo.Dir and
// Repo.Git: with an empty global config a fixture still commits, and its own
// config keeps auto-maintenance off for any git run inside it.
func TestFixture_CommitsWithNoAmbientIdentity(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", empty)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	var r *Repo = Fixture(t)
	if !filepath.IsAbs(r.Dir) {
		t.Errorf("Repo.Dir %q is not absolute", r.Dir)
	}
	r.Git("commit", "--allow-empty", "-q", "-m", "first")
	if got := r.Git("log", "-1", "--format=%s %ae"); got != "first gittest@example.com" {
		t.Errorf("HEAD = %q, want the fixture identity's commit", got)
	}
	if got := r.Git("symbolic-ref", "--short", "HEAD"); got != "main" {
		t.Errorf("branch = %q, want main", got)
	}
	assertPersistsMaintenanceConfig(t, r.Dir)
}

// TestBareAndClone_CarryTheFixtureConfig names Bare and Clone: a clone of a
// bare origin tracks its main and persists the fixture config itself, since
// production fetches in it never pass through Repo.Git.
func TestBareAndClone_CarryTheFixtureConfig(t *testing.T) {
	origin := Bare(t)
	if got := origin.Git("rev-parse", "--is-bare-repository"); got != "true" {
		t.Fatalf("Bare: --is-bare-repository = %q", got)
	}
	seed := Fixture(t)
	seed.Git("commit", "--allow-empty", "-q", "-m", "base")
	seed.Git("push", "-q", origin.Dir, "main")

	clone := Clone(t, origin.Dir)
	if got, want := clone.Git("rev-parse", "HEAD"), seed.Git("rev-parse", "HEAD"); got != want {
		t.Errorf("clone HEAD = %s, want the pushed %s", got, want)
	}
	for _, r := range []*Repo{origin, clone} {
		assertPersistsMaintenanceConfig(t, r.Dir)
	}
	if got := rawConfig(t, clone.Dir, "user.email"); got != "gittest@example.com" {
		t.Errorf("clone identity = %q, want the fixture identity", got)
	}
}

func TestMaintenanceConfig_TurnsBackgroundMaintenanceOff(t *testing.T) {
	got := map[string]string{}
	for _, kv := range MaintenanceConfig() {
		got[kv[0]] = kv[1]
	}
	for key, want := range map[string]string{"maintenance.auto": "false", "gc.auto": "0"} {
		if got[key] != want {
			t.Errorf("MaintenanceConfig %s = %q, want %q", key, got[key], want)
		}
	}
}

func TestMaintenanceConfig_ReturnsACopyCallersCannotMutate(t *testing.T) {
	want := slices.Clone(MaintenanceConfig())
	first := MaintenanceConfig()
	first[0][1] = "mutated"
	if second := MaintenanceConfig(); !slices.Equal(second, want) {
		t.Errorf("MaintenanceConfig() = %v after a caller mutated an earlier result, want %v", second, want)
	}
}

func TestConfigEnv_ReplacesAmbientCommandScopeWithMaintenanceConfigAndExtras(t *testing.T) {
	extra := [2]string{"core.hooksPath", "/nonexistent/hooks"}
	tableLen := len(MaintenanceConfig()) + 1
	stray := strconv.Itoa(tableLen)
	t.Setenv("GIT_CONFIG_COUNT", strconv.Itoa(tableLen+1))
	t.Setenv("GIT_CONFIG_KEY_"+stray, "ambient.stray")
	t.Setenv("GIT_CONFIG_VALUE_"+stray, "leaked")
	env := append(os.Environ(), ConfigEnv(extra)...)

	for _, kv := range append(MaintenanceConfig(), extra) {
		if got, err := commandScopeConfig(t, env, kv[0]); err != nil || got != kv[1] {
			t.Errorf("git under ConfigEnv: %s = %q (%v), want %q", kv[0], got, err, kv[1])
		}
	}
	if got, err := commandScopeConfig(t, env, "ambient.stray"); err == nil {
		t.Errorf("ConfigEnv let ambient command-scope config through: ambient.stray = %q", got)
	}
}

func commandScopeConfig(t *testing.T, env []string, key string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", "-C", t.TempDir(), "config", "--get", key)
	cmd.Env = env
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
