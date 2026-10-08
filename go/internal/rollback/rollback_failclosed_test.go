package rollback

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

type fakeTools struct{ pwdFile string }

func installFakeTools(t *testing.T, ghBody, gitBody string) fakeTools {
	t.Helper()
	dir := t.TempDir()
	fakeclitest.Install(t, filepath.Join(dir, "gh"), "#!/bin/sh\n"+ghBody)
	fakeclitest.Install(t, filepath.Join(dir, "git"), "#!/bin/sh\n"+gitBody)
	pwdFile := filepath.Join(t.TempDir(), "gh-pwd.txt")
	t.Setenv("FAKE_GH_PWD", pwdFile)
	t.Setenv("PATH", dir)
	return fakeTools{pwdFile: pwdFile}
}

func (f fakeTools) ghWorkingDirs(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(f.pwdFile)
	if err != nil {
		return nil
	}
	return strings.Fields(string(b))
}

func runWithDefaultLookups(t *testing.T, repo, jp string) Result {
	t.Helper()
	res, _ := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Steps: Steps{
			RevertAndShip: func(string, string, string, string) string { return "reverted" },
		},
		Now: func() time.Time { return time.Unix(0, 0) },
	})
	return res
}

func TestRun_LsRemoteFails_TagDeleteIsFailedNotAbsent(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	installFakeTools(t, "exit 0\n",
		"case \"$1\" in ls-remote) echo 'fatal: unable to access origin' >&2; exit 128;; esac\nexit 0\n")
	res := runWithDefaultLookups(t, repo, jp)
	if res.TagDelete != "failed" {
		t.Errorf("TagDelete = %q, want failed when ls-remote exits 128", res.TagDelete)
	}
	if res.OverallSucceeded {
		t.Error("rollback must not report success when the remote tag lookup failed")
	}
}

func TestRun_LsRemoteListsTag_PushesDeletion(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	installFakeTools(t, "exit 0\n",
		"case \"$1\" in ls-remote) echo 'abc\trefs/tags/v1.2.3';; esac\nexit 0\n")
	res := runWithDefaultLookups(t, repo, jp)
	if res.TagDelete != "deleted" {
		t.Errorf("TagDelete = %q, want deleted when the remote lists the tag", res.TagDelete)
	}
}

func TestRun_LsRemoteEmptySuccess_TagNotPresent(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	installFakeTools(t, "exit 0\n", "exit 0\n")
	res := runWithDefaultLookups(t, repo, jp)
	if res.TagDelete != "not-present" {
		t.Errorf("TagDelete = %q, want not-present when ls-remote succeeds with no match", res.TagDelete)
	}
}

func TestRun_GhViewAuthFailure_ReleaseDeleteIsFailed(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	installFakeTools(t,
		"case \"$2\" in view) echo 'HTTP 401: Bad credentials' >&2; exit 1;; esac\nexit 0\n", "exit 0\n")
	res := runWithDefaultLookups(t, repo, jp)
	if res.ReleaseDelete != "failed" {
		t.Errorf("ReleaseDelete = %q, want failed on an auth error", res.ReleaseDelete)
	}
	if res.OverallSucceeded {
		t.Error("rollback must not report success when the release lookup failed")
	}
}

func TestRun_GhViewNotFound_ReleaseDeleteIsNotPresent(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	installFakeTools(t,
		"case \"$2\" in view) echo 'release not found' >&2; exit 1;; esac\nexit 0\n", "exit 0\n")
	res := runWithDefaultLookups(t, repo, jp)
	if res.ReleaseDelete != "not-present" {
		t.Errorf("ReleaseDelete = %q, want not-present when gh says release not found", res.ReleaseDelete)
	}
}

func TestRun_GhRunsInRepoRoot(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	fakes := installFakeTools(t,
		"pwd >> \"$FAKE_GH_PWD\"\nexit 0\n", "exit 0\n")
	origin, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origin) })
	runWithDefaultLookups(t, repo, jp)
	dirs := fakes.ghWorkingDirs(t)
	if len(dirs) == 0 {
		t.Fatal("fake gh was never invoked")
	}
	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		got, err := filepath.EvalSymlinks(d)
		if err != nil || got != want {
			t.Errorf("gh ran in %q, want RepoRoot %q", d, want)
		}
	}
}
