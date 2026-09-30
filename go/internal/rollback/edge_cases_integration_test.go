//go:build integration

package rollback

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_NilStepsGetDefaultsWired(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "/nonexistent-bin-for-nil-steps-test")

	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Reason:      "nil steps test",
		Steps:       Steps{},
		DryRun:      false,
	})
	if !errors.Is(err, ErrPartial) {
		t.Errorf("revert fails in non-git dir → want ErrPartial, got %v", err)
	}
	if res.Revert != "failed" {
		t.Errorf("Revert: want %q, got %q", "failed", res.Revert)
	}
	if res.ReleaseDelete == "" || res.TagDelete == "" {
		t.Errorf("default steps must set terminal outcomes: ReleaseDelete=%q TagDelete=%q",
			res.ReleaseDelete, res.TagDelete)
	}
}

func TestRun_NilGhDeleteRelease_DefaultIsWired(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	t.Setenv("PATH", "/nonexistent-bin-nil-gh-test")

	steps := Steps{
		GhDeleteRelease: nil,
		DeleteRemoteTag: func(string, string) string { return "deleted" },
		RevertAndShip:   func(string, string, string, string) string { return "reverted" },
	}
	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Steps:       steps,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ReleaseDelete != "skipped" {
		t.Errorf("ReleaseDelete = %q, want 'skipped' (gh not in PATH)", res.ReleaseDelete)
	}
	if !res.OverallSucceeded {
		t.Error("OverallSucceeded should be true when skipped+deleted+reverted")
	}
}

func TestRun_NilDeleteRemoteTag_DefaultIsWired(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	steps := Steps{
		GhDeleteRelease: func(string) string { return "deleted" },
		DeleteRemoteTag: nil,
		RevertAndShip:   func(string, string, string, string) string { return "reverted" },
	}
	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Steps:       steps,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TagDelete != "not-present" {
		t.Errorf("TagDelete = %q, want 'not-present'", res.TagDelete)
	}
	if !res.OverallSucceeded {
		t.Error("OverallSucceeded should be true when tag not-present + revert succeeded")
	}
}

func TestRun_NilRevertAndShip_DefaultIsWired(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	t.Setenv("EVOLVE_GO_BIN", "")

	steps := Steps{
		GhDeleteRelease: func(string) string { return "deleted" },
		DeleteRemoteTag: func(string, string) string { return "deleted" },
		RevertAndShip:   nil,
	}
	_, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Steps:       steps,
	})
	if !errors.Is(err, ErrPartial) {
		t.Errorf("err = %v, want ErrPartial (defaultRevertAndShip fails on non-git dir)", err)
	}
}

func TestDefaultGhDeleteRelease_ViewSucceeds_DeleteSucceeds(t *testing.T) {
	dir := t.TempDir()
	ghScript := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	got := defaultGhDeleteRelease("v9.9.9")
	if got != "deleted" {
		t.Errorf("got %q, want 'deleted' when gh view+delete both succeed", got)
	}
}

func TestDefaultGhDeleteRelease_ViewSucceeds_DeleteFails(t *testing.T) {
	dir := t.TempDir()
	ghScript := filepath.Join(dir, "gh")
	script := `#!/bin/sh
case "$2" in
  view)   exit 0 ;;
  delete) exit 1 ;;
  *)      exit 0 ;;
esac
`
	if err := os.WriteFile(ghScript, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	got := defaultGhDeleteRelease("v9.9.9")
	if got != "failed" {
		t.Errorf("got %q, want 'failed' when delete sub-command exits 1", got)
	}
}

func TestDefaultGhDeleteRelease_ViewFails_IsNotPresent(t *testing.T) {
	dir := t.TempDir()
	ghScript := filepath.Join(dir, "gh")
	script := `#!/bin/sh
case "$2" in
  view)   exit 1 ;;
  *)      exit 0 ;;
esac
`
	if err := os.WriteFile(ghScript, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	got := defaultGhDeleteRelease("v9.9.9")
	if got != "not-present" {
		t.Errorf("got %q, want 'not-present' when view exits non-zero", got)
	}
}

func TestDefaultDeleteRemoteTag_TagPresent_PushFails(t *testing.T) {
	dir := t.TempDir()
	tag := "v9.9.9"
	gitScript := filepath.Join(dir, "git")
	script := `#!/bin/sh
# Detect subcommand by scanning args.
for arg in "$@"; do
  case "$arg" in
    ls-remote) echo "refs/tags/v9.9.9"; exit 0 ;;
    push)      exit 1 ;;
  esac
done
exit 0
`
	if err := os.WriteFile(gitScript, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	got := defaultDeleteRemoteTag(t.TempDir(), tag)
	if got != "failed" {
		t.Errorf("got %q, want 'failed' when push exits 1", got)
	}
}

func TestDefaultDeleteRemoteTag_TagPresent_PushSucceeds(t *testing.T) {
	dir := t.TempDir()
	tag := "v9.9.9"
	gitScript := filepath.Join(dir, "git")
	script := `#!/bin/sh
for arg in "$@"; do
  case "$arg" in
    ls-remote) echo "refs/tags/v9.9.9"; exit 0 ;;
    push)      exit 0 ;;
  esac
done
exit 0
`
	if err := os.WriteFile(gitScript, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	got := defaultDeleteRemoteTag(t.TempDir(), tag)
	if got != "deleted" {
		t.Errorf("got %q, want 'deleted' when push succeeds", got)
	}
}

func TestDefaultRevertAndShip_RevertSucceeds_NoBin_LocalOnly(t *testing.T) {
	dir := t.TempDir()
	gitScript := filepath.Join(dir, "git")
	if err := os.WriteFile(gitScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("EVOLVE_GO_BIN", "")

	repoRoot := t.TempDir()
	got := defaultRevertAndShip(repoRoot, "deadbeef", "test reason", "9.9.9")
	if got != "local-only" {
		t.Errorf("got %q, want 'local-only' when revert ok but no evolve binary", got)
	}
}

func TestDefaultRevertAndShip_RevertSucceeds_BinPresent_ShipSucceeds(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	evolveBin := filepath.Join(dir, "fake-evolve")
	if err := os.WriteFile(evolveBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("EVOLVE_GO_BIN", evolveBin)

	repoRoot := t.TempDir()
	got := defaultRevertAndShip(repoRoot, "deadbeef", "test reason", "9.9.9")
	if got != "reverted" {
		t.Errorf("got %q, want 'reverted' when revert+ship both exit 0", got)
	}
}

func TestDefaultRevertAndShip_RevertSucceeds_BinPresent_ShipFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	evolveBin := filepath.Join(dir, "fake-evolve-fail")
	if err := os.WriteFile(evolveBin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("EVOLVE_GO_BIN", evolveBin)

	repoRoot := t.TempDir()
	got := defaultRevertAndShip(repoRoot, "deadbeef", "test reason", "9.9.9")
	if got != "local-only" {
		t.Errorf("got %q, want 'local-only' when revert ok but ship exits 1", got)
	}
}
