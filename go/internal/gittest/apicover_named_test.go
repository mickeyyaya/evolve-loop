package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// rawConfig reads a key the way production code in a fixture sees it: a plain
// git in the repo, without the helper.
func rawConfig(t *testing.T, dir, key string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "config", "--get", key).Output()
	if err != nil {
		t.Fatalf("git config --get %s in %s: %v", key, dir, err)
	}
	return strings.TrimSpace(string(out))
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
	if got := rawConfig(t, r.Dir, "maintenance.auto"); got != "false" {
		t.Errorf("maintenance.auto = %q in the repo's own config, want false", got)
	}
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
		if got := rawConfig(t, r.Dir, "maintenance.auto"); got != "false" {
			t.Errorf("%s: maintenance.auto = %q, want false", r.Dir, got)
		}
	}
	if got := rawConfig(t, clone.Dir, "user.email"); got != "gittest@example.com" {
		t.Errorf("clone identity = %q, want the fixture identity", got)
	}
}
