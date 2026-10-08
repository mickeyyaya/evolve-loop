//go:build integration

package rollback

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
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

func runWithDefaultTagDelete(t *testing.T, fakeGit string) (Result, error) {
	t.Helper()
	jp, repo := makeJournal(t, journalFull)
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "git"), fakeGit)
	t.Setenv("PATH", bin)
	return Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Steps: Steps{
			GhDeleteRelease: func(string) string { return "deleted" },
			DeleteRemoteTag: nil,
			RevertAndShip:   func(string, string, string, string) string { return "reverted" },
		},
	})
}

func TestRun_NilDeleteRemoteTag_DefaultIsWired(t *testing.T) {
	res, err := runWithDefaultTagDelete(t, "#!/bin/sh\nexit 0\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TagDelete != "not-present" {
		t.Errorf("TagDelete = %q, want 'not-present' when ls-remote succeeds with no match", res.TagDelete)
	}
	if !res.OverallSucceeded {
		t.Error("OverallSucceeded should be true when tag not-present + revert succeeded")
	}
}

func TestRun_NilDeleteRemoteTag_LsRemoteFails_IsPartial(t *testing.T) {
	res, err := runWithDefaultTagDelete(t,
		"#!/bin/sh\ncase \"$1\" in ls-remote) echo 'fatal: no origin' >&2; exit 128;; esac\nexit 0\n")
	if !errors.Is(err, ErrPartial) {
		t.Errorf("err = %v, want ErrPartial when ls-remote cannot reach origin", err)
	}
	if res.TagDelete != "failed" {
		t.Errorf("TagDelete = %q, want 'failed' when ls-remote exits 128", res.TagDelete)
	}
	if res.OverallSucceeded {
		t.Error("OverallSucceeded must be false after a failed remote tag lookup")
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
	fakeclitest.Install(t, ghScript, "#!/bin/sh\nexit 0\n")
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
	fakeclitest.Install(t, ghScript, script)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	got := defaultGhDeleteRelease("v9.9.9")
	if got != "failed" {
		t.Errorf("got %q, want 'failed' when delete sub-command exits 1", got)
	}
}

func TestDefaultGhDeleteRelease_ViewFails(t *testing.T) {
	cases := []struct {
		name       string
		viewStderr string
		want       string
	}{
		{"release not found is not-present", "release not found", "not-present"},
		{"HTTP 404 is not-present", "HTTP 404: Not Found", "not-present"},
		{"auth failure is failed", "HTTP 401: Bad credentials", "failed"},
		{"silent non-zero exit is failed", "", "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := "#!/bin/sh\ncase \"$2\" in\n  view) echo '" + tc.viewStderr + "' >&2; exit 1 ;;\n  *) exit 0 ;;\nesac\n"
			fakeclitest.Install(t, filepath.Join(dir, "gh"), script)
			t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

			if got := defaultGhDeleteRelease("v9.9.9"); got != tc.want {
				t.Errorf("view stderr %q: got %q, want %q", tc.viewStderr, got, tc.want)
			}
		})
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
	fakeclitest.Install(t, gitScript, script)
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
	fakeclitest.Install(t, gitScript, script)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	got := defaultDeleteRemoteTag(t.TempDir(), tag)
	if got != "deleted" {
		t.Errorf("got %q, want 'deleted' when push succeeds", got)
	}
}

func TestDefaultRevertAndShip_RevertSucceeds_NoBin_LocalOnly(t *testing.T) {
	dir := t.TempDir()
	gitScript := filepath.Join(dir, "git")
	fakeclitest.Install(t, gitScript, "#!/bin/sh\nexit 0\n")
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
	fakeclitest.Install(t, filepath.Join(dir, "git"), "#!/bin/sh\nexit 0\n")
	evolveBin := filepath.Join(dir, "fake-evolve")
	fakeclitest.Install(t, evolveBin, "#!/bin/sh\nexit 0\n")
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
	fakeclitest.Install(t, filepath.Join(dir, "git"), "#!/bin/sh\nexit 0\n")
	evolveBin := filepath.Join(dir, "fake-evolve-fail")
	fakeclitest.Install(t, evolveBin, "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("EVOLVE_GO_BIN", evolveBin)

	repoRoot := t.TempDir()
	got := defaultRevertAndShip(repoRoot, "deadbeef", "test reason", "9.9.9")
	if got != "local-only" {
		t.Errorf("got %q, want 'local-only' when revert ok but ship exits 1", got)
	}
}
