//go:build integration

package publishmirror

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// setupPrivateRepo builds a temp repo mimicking the private tree: a tracked
// binary, a commit-prefix-scope with a chore(build) entry, a README, a clean
// doc, and (optionally) a doc that leaks a macOS home path.
func setupPrivateRepo(t *testing.T, withLeak bool) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "Test")

	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go/evolve", "\x7fELF\x00fake binary\x00")
	// A legitimate tracked binary (image) that is published as-is but must be
	// skipped by the text sanitizer (it is not text).
	write("assets/logo.png", "\x89PNG\r\n\x00\x00binary\x00image\x00data")
	write(".evolve/commit-prefix-scope.json",
		`{"feat":{"description":"f"},"chore(build)":{"required_paths":["go/evolve","go/bin/**"],"description":"b"}}`)
	write("README.md", "# evolveloop\n\nThe full private README.\n")
	write("docs/clean.md", "Clean: `~/ai/claude/evolve-loop/go`, user@example.com.\n")
	if withLeak {
		write("docs/leak.md", "oops /Users/danleemh/secret/path here\n")
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

func TestRun_BadRef_Errors(t *testing.T) {
	repo := setupPrivateRepo(t, false)
	if _, err := Run(context.Background(), Options{RepoDir: repo, Ref: "no-such-ref-xyz", Push: false}); err == nil {
		t.Fatal("an unresolvable ref should error")
	}
}

func TestRun_DryRun_Clean(t *testing.T) {
	repo := setupPrivateRepo(t, false)
	res, err := Run(context.Background(), Options{RepoDir: repo, Push: false})
	if err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}
	if res.Pushed {
		t.Error("dry-run must not push")
	}
	if len(res.Violations) != 0 {
		t.Errorf("clean tree should have no violations: %+v", res.Violations)
	}
	if res.StagedFiles == 0 {
		t.Error("expected staged files > 0")
	}
	if len(res.Dropped) != 1 || res.Dropped[0] != "go/evolve" {
		t.Errorf("expected go/evolve dropped, got %v", res.Dropped)
	}
}

func TestRun_DryRun_SanitizerCatchesLeak(t *testing.T) {
	repo := setupPrivateRepo(t, true)
	res, err := Run(context.Background(), Options{RepoDir: repo, Push: false})
	if err == nil {
		t.Fatal("expected sanitizer error on a leaking tree")
	}
	if res.Pushed {
		t.Error("must not push when sanitizer fails")
	}
	if len(res.Violations) == 0 {
		t.Fatal("expected at least one violation")
	}
	found := false
	for _, v := range res.Violations {
		if v.File == "docs/leak.md" && v.Rule == "macos-home-path" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected macos-home-path violation in docs/leak.md, got %+v", res.Violations)
	}
}

func TestRun_DryRun_DenylistCatchesUsername(t *testing.T) {
	repo := setupPrivateRepo(t, false)
	// README contains "private"; use a denylist term present in the tree.
	_, err := Run(context.Background(), Options{RepoDir: repo, Push: false, Denylist: []string{"evolveloop"}})
	if err == nil {
		t.Fatal("denylist term present in tree should fail the sanitizer")
	}
}

func TestRun_Push_PublishesSquashedMirror(t *testing.T) {
	repo := setupPrivateRepo(t, false)
	bare := t.TempDir()
	git(t, bare, "init", "--bare", "-q")

	res, err := Run(context.Background(), Options{
		RepoDir: repo, Remote: bare, Push: true, Tag: "v9.9.9", Message: "Release v9.9.9",
	})
	if err != nil {
		t.Fatalf("Run push: %v", err)
	}
	if !res.Pushed || res.PublicRef != "main" || res.Tag != "v9.9.9" {
		t.Fatalf("unexpected push result: %+v", res)
	}

	// Clone the mirror and verify the published tree.
	clone := t.TempDir()
	git(t, filepath.Dir(clone), "clone", "-q", bare, clone)

	// (1) History is severed — exactly one commit.
	if n := strings.TrimSpace(git(t, clone, "rev-list", "--count", "HEAD")); n != "1" {
		t.Errorf("mirror must be a single squashed commit, got %s commits", n)
	}
	// (2) The tracked binary is gone.
	if _, err := os.Stat(filepath.Join(clone, "go", "evolve")); !os.IsNotExist(err) {
		t.Error("go/evolve must not be in the published mirror")
	}
	// (3) The chore(build) prefix entry is gone.
	scope, err := os.ReadFile(filepath.Join(clone, ".evolve", "commit-prefix-scope.json"))
	if err != nil {
		t.Fatalf("read mirror commit-prefix-scope: %v", err)
	}
	if strings.Contains(string(scope), "chore(build)") {
		t.Error("chore(build) entry must be removed from the mirror")
	}
	// (4) The README is present.
	if _, err := os.Stat(filepath.Join(clone, "README.md")); err != nil {
		t.Errorf("README.md missing from mirror: %v", err)
	}
	// (5) The tag exists on the mirror.
	if tags := git(t, clone, "tag", "--list"); !strings.Contains(tags, "v9.9.9") {
		t.Errorf("tag v9.9.9 missing from mirror, got %q", tags)
	}
}

func TestRun_Push_BadRemote_Errors(t *testing.T) {
	repo := setupPrivateRepo(t, false)
	// A clean tree (sanitizer passes) but an unreachable remote: the commit
	// succeeds locally, the push fails, and Run reports the error without
	// claiming success.
	res, err := Run(context.Background(), Options{
		RepoDir: repo, Remote: filepath.Join(t.TempDir(), "does-not-exist"), Push: true, Message: "Release",
	})
	if err == nil {
		t.Fatal("push to an unreachable remote should error")
	}
	if res != nil && res.Pushed {
		t.Error("Pushed must be false when the push fails")
	}
}

func TestRun_Push_ReadmeSwap(t *testing.T) {
	repo := setupPrivateRepo(t, false)
	bare := t.TempDir()
	git(t, bare, "init", "--bare", "-q")

	pub := filepath.Join(t.TempDir(), "README.public.md")
	if err := os.WriteFile(pub, []byte("# evolveloop (condensed public pitch)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), Options{
		RepoDir: repo, Remote: bare, Push: true, PublicReadme: pub, Message: "Release",
	}); err != nil {
		t.Fatalf("Run push with README swap: %v", err)
	}
	clone := t.TempDir()
	git(t, filepath.Dir(clone), "clone", "-q", bare, clone)
	got, err := os.ReadFile(filepath.Join(clone, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "condensed public pitch") {
		t.Errorf("public README was not swapped in: %q", got)
	}
}
