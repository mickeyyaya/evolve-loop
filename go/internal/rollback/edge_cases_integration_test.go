//go:build integration

// edge_cases_integration_test.go — subprocess-spawning tests from edge_cases_test.go.
//
// Every test here forks at least one real process (git, gh, or a fake shell
// script that replaces them). Run with:
//
//	go test -tags integration -race -count=1 ./internal/rollback/
//
// Coverage targets (integration tier):
//   - Run: nil-step fallback wiring (defaults shell out)
//   - defaultGhDeleteRelease: fake gh scripts (view success/fail, delete success/fail)
//   - defaultDeleteRemoteTag: fake git scripts (tag present + push success/fail)
//   - defaultRevertAndShip: fake git + evolve scripts (revert ok, ship ok/fail/absent)
package rollback

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Run: nil-step fallback wiring
// ---------------------------------------------------------------------------

// TestRun_NilStepsGetDefaultsWired — when Steps fields are nil and DryRun=false,
// Run wires defaults before executing. The defaults will shell out but in a
// non-git temp dir each will return a non-success status rather than panic.
// The key behavior: nil fields are replaced (no nil-deref panic).
func TestRun_NilStepsGetDefaultsWired(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	// Provide completely nil Steps — all three fields are nil.
	// defaultGhDeleteRelease: gh likely not in PATH → "skipped"
	// defaultDeleteRemoteTag: ls-remote on a non-git dir → "not-present"
	// defaultRevertAndShip: git revert on non-git dir → "failed"
	// The test must not panic, and must return ErrPartial (revert="failed").
	// Also neutralize EVOLVE_GO_BIN so resolveEvolveBinForRollback stays in test context.
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "/nonexistent-bin-for-nil-steps-test")

	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Reason:      "nil steps test",
		Steps:       Steps{}, // all three funcs are nil → wired to defaults
		DryRun:      false,
	})
	// With PATH neutered and a non-git repo, the wired defaults must reach
	// definite terminal outcomes — not just any non-empty string. git revert
	// in a non-git dir fails, so Revert="failed" and Run returns ErrPartial.
	if !errors.Is(err, ErrPartial) {
		t.Errorf("revert fails in non-git dir → want ErrPartial, got %v", err)
	}
	if res.Revert != "failed" {
		t.Errorf("Revert: want %q, got %q", "failed", res.Revert)
	}
	// gh/git-tag defaults still resolve to a terminal outcome (not blank).
	if res.ReleaseDelete == "" || res.TagDelete == "" {
		t.Errorf("default steps must set terminal outcomes: ReleaseDelete=%q TagDelete=%q",
			res.ReleaseDelete, res.TagDelete)
	}
}

// TestRun_NilGhDeleteRelease_DefaultIsWired — only GhDeleteRelease is nil;
// others are provided. Verifies that the partial-nil path is also wired.
func TestRun_NilGhDeleteRelease_DefaultIsWired(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	t.Setenv("PATH", "/nonexistent-bin-nil-gh-test")

	steps := Steps{
		GhDeleteRelease: nil, // should be wired to defaultGhDeleteRelease → "skipped"
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
	// defaultGhDeleteRelease with no gh in PATH → "skipped" (legitimate, not "failed")
	if res.ReleaseDelete != "skipped" {
		t.Errorf("ReleaseDelete = %q, want 'skipped' (gh not in PATH)", res.ReleaseDelete)
	}
	if !res.OverallSucceeded {
		t.Error("OverallSucceeded should be true when skipped+deleted+reverted")
	}
}

// TestRun_NilDeleteRemoteTag_DefaultIsWired — only DeleteRemoteTag is nil.
// ls-remote on a non-git temp dir returns empty output → "not-present".
func TestRun_NilDeleteRemoteTag_DefaultIsWired(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	steps := Steps{
		GhDeleteRelease: func(string) string { return "deleted" },
		DeleteRemoteTag: nil, // wired to defaultDeleteRemoteTag → "not-present"
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
	// "not-present" is legitimate (tag already gone), so overall succeeds.
	if res.TagDelete != "not-present" {
		t.Errorf("TagDelete = %q, want 'not-present'", res.TagDelete)
	}
	if !res.OverallSucceeded {
		t.Error("OverallSucceeded should be true when tag not-present + revert succeeded")
	}
}

// TestRun_NilRevertAndShip_DefaultIsWired — only RevertAndShip is nil.
// defaultRevertAndShip on a non-git dir → "failed" → ErrPartial.
func TestRun_NilRevertAndShip_DefaultIsWired(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	t.Setenv("EVOLVE_GO_BIN", "")

	steps := Steps{
		GhDeleteRelease: func(string) string { return "deleted" },
		DeleteRemoteTag: func(string, string) string { return "deleted" },
		RevertAndShip:   nil, // wired to defaultRevertAndShip → "failed" on non-git dir
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

// ---------------------------------------------------------------------------
// defaultGhDeleteRelease: gh in PATH, release present then delete success
// ---------------------------------------------------------------------------

// TestDefaultGhDeleteRelease_ViewSucceeds_DeleteSucceeds — exercises the success
// path: `gh release view` succeeds (exit 0) AND `gh release delete` succeeds.
// We use a fake `gh` script to control both outcomes.
func TestDefaultGhDeleteRelease_ViewSucceeds_DeleteSucceeds(t *testing.T) {
	dir := t.TempDir()
	// Fake gh: always exits 0 regardless of sub-command.
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

// TestDefaultGhDeleteRelease_ViewSucceeds_DeleteFails — `gh release view`
// succeeds (release present) but `gh release delete` fails (exit 1).
func TestDefaultGhDeleteRelease_ViewSucceeds_DeleteFails(t *testing.T) {
	dir := t.TempDir()
	// Fake gh: `view` exits 0; `delete` exits 1.
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

// TestDefaultGhDeleteRelease_ViewFails_IsNotPresent — `gh release view` exits
// non-zero → release does not exist → "not-present".
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

// ---------------------------------------------------------------------------
// defaultDeleteRemoteTag: remote tag present but push fails/succeeds
// ---------------------------------------------------------------------------

// TestDefaultDeleteRemoteTag_TagPresent_PushFails — ls-remote reports the tag
// exists but `git push origin :refs/tags/...` fails → "failed".
//
// We inject a fake git that: ls-remote echos the tag, push exits 1.
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

// TestDefaultDeleteRemoteTag_TagPresent_PushSucceeds — ls-remote reports tag,
// push succeeds → "deleted". Also exercises the local tag cleanup best-effort call.
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

// ---------------------------------------------------------------------------
// defaultRevertAndShip: revert succeeds paths
// ---------------------------------------------------------------------------

// TestDefaultRevertAndShip_RevertSucceeds_NoBin_LocalOnly — git revert succeeds
// but no evolve binary exists → "local-only".
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

// TestDefaultRevertAndShip_RevertSucceeds_BinPresent_ShipSucceeds —
// git revert exits 0 AND evolve binary exits 0 → "reverted".
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

// TestDefaultRevertAndShip_RevertSucceeds_BinPresent_ShipFails —
// git revert exits 0 BUT evolve ship exits 1 → "local-only".
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
