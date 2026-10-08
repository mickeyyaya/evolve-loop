//go:build acs

package cycle1839

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/rollback"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const journalJSON = `{"version":"1.2.3","tag":"v1.2.3","commit_sha":"deadbeef","branch":"main"}`

func rollbackWithFakes(t *testing.T, ghBody, gitBody string) rollback.Result {
	res, _ := rollbackInRepo(t, ghBody, gitBody)
	return res
}

func rollbackInRepo(t *testing.T, ghBody, gitBody string) (rollback.Result, string) {
	t.Helper()
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "gh"), "#!/bin/sh\n"+ghBody)
	fakeclitest.Install(t, filepath.Join(bin, "git"), "#!/bin/sh\n"+gitBody)
	t.Setenv("PATH", bin)
	repo := t.TempDir()
	journal := filepath.Join(repo, "journal.json")
	if err := os.WriteFile(journal, []byte(journalJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	res, _ := rollback.Run(rollback.Options{
		JournalPath: journal,
		RepoRoot:    repo,
		Steps: rollback.Steps{
			RevertAndShip: func(string, string, string, string) string { return "reverted" },
		},
		Now: func() time.Time { return time.Unix(0, 0) },
	})
	return res, repo
}

func TestC1839_001_FailingLsRemoteIsFailedNotAbsent(t *testing.T) {
	res := rollbackWithFakes(t, "exit 0\n",
		"case \"$1\" in ls-remote) echo 'fatal: no origin' >&2; exit 128;; esac\nexit 0\n")
	if res.TagDelete != "failed" {
		t.Errorf("RED: TagDelete = %q, want failed", res.TagDelete)
	}
	if res.OverallSucceeded {
		t.Error("RED: rollback reported success after a failed remote tag lookup")
	}
}

func TestC1839_002_GhAuthFailureIsFailedAndNotFoundIsAbsent(t *testing.T) {
	auth := rollbackWithFakes(t,
		"case \"$2\" in view) echo 'HTTP 401: Bad credentials' >&2; exit 1;; esac\nexit 0\n", "exit 0\n")
	if auth.ReleaseDelete != "failed" {
		t.Errorf("RED: auth failure ReleaseDelete = %q, want failed", auth.ReleaseDelete)
	}
	missing := rollbackWithFakes(t,
		"case \"$2\" in view) echo 'release not found' >&2; exit 1;; esac\nexit 0\n", "exit 0\n")
	if missing.ReleaseDelete != "not-present" {
		t.Errorf("not-found ReleaseDelete = %q, want not-present", missing.ReleaseDelete)
	}
}

func TestC1839_003_DefaultGhRunsInRepoRoot(t *testing.T) {
	pwdFile := filepath.Join(t.TempDir(), "pwd.txt")
	t.Setenv("FAKE_GH_PWD", pwdFile)
	origin, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origin) })
	_, repo := rollbackInRepo(t, "pwd >> \"$FAKE_GH_PWD\"\nexit 0\n", "exit 0\n")
	b, err := os.ReadFile(pwdFile)
	if err != nil {
		t.Fatalf("fake gh never ran: %v", err)
	}
	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range strings.Fields(string(b)) {
		if got, err := filepath.EvalSymlinks(d); err != nil || got != want {
			t.Errorf("RED: gh ran in %q, want RepoRoot %q", d, want)
		}
	}
}

func TestC1839_004_LsRemoteListingTagStillDeletes(t *testing.T) {
	res := rollbackWithFakes(t, "exit 0\n",
		"case \"$1\" in ls-remote) echo 'abc\trefs/tags/v1.2.3';; esac\nexit 0\n")
	if res.TagDelete != "deleted" {
		t.Errorf("TagDelete = %q, want deleted", res.TagDelete)
	}
}

func TestC1839_005_RollbackIntegrationTierEncodesFailClosed(t *testing.T) {
	goModule := filepath.Join(acsassert.RepoRoot(t), "go")
	out, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-C", goModule,
		"-tags", "integration", "-count=1", "-v",
		"-run", "^(TestRun_NilDeleteRemoteTag_|TestDefaultGhDeleteRelease_ViewFails)", "./internal/rollback")
	if err != nil || code != 0 {
		t.Fatalf("RED: rollback integration tier exit=%d err=%v\n%s%s", code, err, out, stderr)
	}
	for _, mustRun := range []string{
		"--- PASS: TestRun_NilDeleteRemoteTag_LsRemoteFails_IsPartial",
		"--- PASS: TestDefaultGhDeleteRelease_ViewFails/auth_failure_is_failed",
		"--- PASS: TestDefaultGhDeleteRelease_ViewFails/silent_non-zero_exit_is_failed",
		"--- PASS: TestDefaultGhDeleteRelease_ViewFails/release_not_found_is_not-present",
	} {
		if !strings.Contains(out, mustRun) {
			t.Errorf("RED: integration tier did not pass %q", mustRun)
		}
	}
	if strings.Contains(out, "TestDefaultGhDeleteRelease_ViewFails_IsNotPresent") {
		t.Error("RED: the fail-open integration test that maps every gh view failure to not-present still runs")
	}
}
