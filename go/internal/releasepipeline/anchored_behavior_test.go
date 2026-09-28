package releasepipeline

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func stampScript(version string, exitCode int) string {
	return fmt.Sprintf("#!/bin/sh\necho \"evolve version %s\"\nexit %d\n", version, exitCode)
}

func writeTrackedBinary(t *testing.T, repo, script string) {
	t.Helper()
	bin := filepath.Join(repo, "go", "evolve")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func committedBinaryRepo(t *testing.T, script string) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeTrackedBinary(t, repo.Dir, script)
	repo.Git("add", "go/evolve")
	repo.Git("commit", "-m", "release")
	return repo, repo.Git("rev-parse", "HEAD")
}

func writeStateFile(t *testing.T, repo, body string) string {
	t.Helper()
	path := filepath.Join(repo, ".evolve", "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertFileBytes(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s rewritten:\n%s", path, got)
	}
}

func TestDefaultReleaseVerify_DiskBinaryMustMatchCommittedBlob(t *testing.T) {
	repo, commit := committedBinaryRepo(t, stampScript("1.2.3", 0))
	writeTrackedBinary(t, repo.Dir, stampScript("1.2.3", 0)+"# rebuilt\n")
	err := defaultReleaseVerify(repo.Dir, "1.2.3", commit)
	if err == nil || !strings.Contains(err.Error(), "not what was committed") {
		t.Fatalf("err = %v", err)
	}
}

func TestDefaultReleaseVerify_ReadsAndTagsTheReleaseCommitNotHEAD(t *testing.T) {
	script := stampScript("1.2.3", 0)
	repo, release := committedBinaryRepo(t, script)
	writeTrackedBinary(t, repo.Dir, script+"# next\n")
	repo.Git("commit", "-am", "next")
	writeTrackedBinary(t, repo.Dir, script)
	if err := defaultReleaseVerify(repo.Dir, "1.2.3", release); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got := repo.Git("rev-parse", "v1.2.3^{commit}"); got != release {
		t.Fatalf("tag v1.2.3 at %s, want the release commit %s", got, release)
	}
}

func TestDefaultReleaseVerify_AlreadyPinnedStateIsLeftAlone(t *testing.T) {
	script := stampScript("1.2.3", 0)
	repo, commit := committedBinaryRepo(t, script)
	body := fmt.Sprintf(`{"expected_ship_sha":"%x","expected_ship_version":"0.9.0"}`, sha256.Sum256([]byte(script)))
	path := writeStateFile(t, repo.Dir, body)
	if err := defaultReleaseVerify(repo.Dir, "1.2.3", commit); err != nil {
		t.Fatalf("verify: %v", err)
	}
	assertFileBytes(t, path, body)
}

func TestDefaultReleaseVerify_MalformedStateIsLeftAlone(t *testing.T) {
	repo, commit := committedBinaryRepo(t, stampScript("1.2.3", 0))
	path := writeStateFile(t, repo.Dir, "{not json")
	if err := defaultReleaseVerify(repo.Dir, "1.2.3", commit); err != nil {
		t.Fatalf("verify: %v", err)
	}
	assertFileBytes(t, path, "{not json")
}

func TestDefaultReleaseVerify_VersionStampMustNameTarget(t *testing.T) {
	repo, commit := committedBinaryRepo(t, stampScript("9.9.9", 0))
	err := defaultReleaseVerify(repo.Dir, "1.2.3", commit)
	if err == nil || !strings.Contains(err.Error(), "does not report target") {
		t.Fatalf("err = %v", err)
	}
}

func TestDefaultReleaseVerify_FailingVersionRunIsAnError(t *testing.T) {
	repo, commit := committedBinaryRepo(t, stampScript("1.2.3", 3))
	err := defaultReleaseVerify(repo.Dir, "1.2.3", commit)
	if err == nil || !strings.Contains(err.Error(), "--version failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestDefaultReleaseVerify_ExistingLocalTagIsKept(t *testing.T) {
	repo, commit := committedBinaryRepo(t, stampScript("1.2.3", 0))
	repo.Git("tag", "v1.2.3", commit)
	if err := defaultReleaseVerify(repo.Dir, "1.2.3", commit); err != nil {
		t.Fatalf("verify with the tag already present: %v", err)
	}
}

func TestDefaultReleaseVerify_TagCreationFailureIsAnError(t *testing.T) {
	repo, commit := committedBinaryRepo(t, stampScript("1..2", 0))
	err := defaultReleaseVerify(repo.Dir, "1..2", commit)
	if err == nil || !strings.Contains(err.Error(), "creation failed") {
		t.Fatalf("err = %v", err)
	}
}

type stepArgs struct {
	preflight       [2]bool
	changelogFrom   string
	changelogDry    bool
	bumpDry         bool
	rebuildDry      bool
	releaseShTarget string
}

func recordingSteps(c *stepArgs) Steps {
	s := allOkSteps()
	s.Preflight = func(_, _ string, dryRun, skipTests bool) error {
		c.preflight = [2]bool{dryRun, skipTests}
		return nil
	}
	s.ChangelogGen = func(_, from, _, _ string, dryRun bool) error {
		c.changelogFrom, c.changelogDry = from, dryRun
		return nil
	}
	s.VersionBump = func(_, _ string, dryRun bool) error {
		c.bumpDry = dryRun
		return nil
	}
	s.RebuildBinary = func(_, _ string, dryRun bool) error {
		c.rebuildDry = dryRun
		return nil
	}
	s.ReleaseSh = func(_, target string) error {
		c.releaseShTarget = target
		return nil
	}
	return s
}

func runRecorded(t *testing.T, opts Options) (Result, *stepArgs, string) {
	t.Helper()
	var c stepArgs
	var buf bytes.Buffer
	opts.Target = "1.2.3"
	opts.Stderr = &buf
	opts.Steps = recordingSteps(&c)
	opts.MaxPollWait = time.Second
	if opts.Now == nil {
		opts.Now = fixedNow(t)
	}
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, buf.String())
	}
	return res, &c, buf.String()
}

func TestPrePublish_DryRunIsPassedThrough(t *testing.T) {
	_, c, _ := runRecorded(t, Options{RepoRoot: t.TempDir(), FromTag: "v1.0.0", DryRun: true})
	if c.preflight != [2]bool{true, false} || !c.changelogDry || !c.bumpDry {
		t.Fatalf("calls = %+v", *c)
	}
}

func TestPrePublish_LiveRunArguments(t *testing.T) {
	_, c, _ := runRecorded(t, Options{RepoRoot: t.TempDir(), FromTag: "v1.0.0", SkipTests: true, JournalDir: t.TempDir()})
	if c.preflight != [2]bool{false, true} || c.changelogDry || c.bumpDry || c.rebuildDry || c.releaseShTarget != "1.2.3" {
		t.Fatalf("calls = %+v", *c)
	}
}

func TestNewReleaseRun_GivenFromTagWins(t *testing.T) {
	_, c, _ := runRecorded(t, Options{RepoRoot: initTempRepoWithTag(t, "v9.9.9"), FromTag: "v0.0.1", DryRun: true})
	if c.changelogFrom != "v0.0.1" {
		t.Fatalf("changelog from %q, want v0.0.1", c.changelogFrom)
	}
}

func TestNewReleaseRun_PreviousTagStartsTheRange(t *testing.T) {
	_, c, _ := runRecorded(t, Options{RepoRoot: initTempRepoWithTag(t, "v1.0.0"), DryRun: true})
	if c.changelogFrom != "v1.0.0" {
		t.Fatalf("changelog from %q, want v1.0.0", c.changelogFrom)
	}
}

func TestNewReleaseRun_TaglessRepoStartsAtInitialCommit(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-m", "initial")
	_, c, log := runRecorded(t, Options{RepoRoot: repo.Dir, DryRun: true})
	if want := repo.Git("rev-parse", "HEAD"); c.changelogFrom != want {
		t.Fatalf("changelog from %q, want the initial commit %s", c.changelogFrom, want)
	}
	if !strings.Contains(log, "[release-pipeline] WARN: no previous tag found; changelog range will start from initial commit\n") {
		t.Fatalf("missing no-tag warning:\n%s", log)
	}
}

func TestNewReleaseRun_BannerReportsTheRun(t *testing.T) {
	res, _, log := runRecorded(t, Options{RepoRoot: t.TempDir(), FromTag: "v1.0.0", DryRun: true, NoRollback: true})
	for _, want := range []string{
		"[release-pipeline] target: v1.2.3\n",
		"[release-pipeline] changelog range: v1.0.0..HEAD\n",
		"[release-pipeline] dry-run: true | no-rollback: true | skip-tests: false\n",
		"[release-pipeline] journal: " + res.JournalPath + "\n",
		"…version=1.2.3 …",
		"[release-pipeline] step: release.sh-check (DRY-RUN — skipping; markers not actually bumped)\n",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
}

func TestNewReleaseRun_NowSeamStampsTheJournal(t *testing.T) {
	res, _, _ := runRecorded(t, Options{RepoRoot: t.TempDir(), FromTag: "v1.0.0", JournalDir: t.TempDir()})
	if got := filepath.Base(res.JournalPath); got != "1.2.3-20260524T120000Z.json" {
		t.Fatalf("journal file %q", got)
	}
}
