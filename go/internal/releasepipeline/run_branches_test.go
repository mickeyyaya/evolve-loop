package releasepipeline

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun_NilNowUsesRealClock(t *testing.T) {
	res, err := Run(Options{
		Target:      "2.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.9.9",
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
		Now:         nil,
	})
	if err != nil {
		t.Fatalf("Run with nil Now: %v", err)
	}
	if res.JournalPath == "" {
		t.Error("JournalPath must be set even with nil Now")
	}
}

func TestRun_NilFullDryRunPreflightNotCalled(t *testing.T) {
	steps := allOkSteps()
	steps.FullDryRunPreflight = nil

	res, err := Run(Options{
		Target:      "3.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v2.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("Run with nil FullDryRunPreflight (unused) = %v", err)
	}
	if !contains(res.StepsCompleted, "preflight") {
		t.Errorf("StepsCompleted = %v, want 'preflight'", res.StepsCompleted)
	}
}

func TestRun_FromTagAutoResolved_ValidRepo(t *testing.T) {
	res, err := Run(Options{
		Target:      "99.0.0",
		RepoRoot:    makeHermeticGitRepo(t),
		FromTag:     "",
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("Run with auto-resolved FromTag: %v", err)
	}
	if res.JournalPath == "" {
		t.Error("JournalPath not set")
	}
}

func TestRun_FromTagAutoResolved_NonGitDir(t *testing.T) {
	dir := t.TempDir()

	res, err := Run(Options{
		Target:      "99.0.0",
		RepoRoot:    dir,
		FromTag:     "",
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("Run with unresolvable fromTag: %v", err)
	}
	if res.JournalPath == "" {
		t.Error("JournalPath not set")
	}
}

func TestRun_ChangelogGenFails(t *testing.T) {
	steps := allOkSteps()
	steps.ChangelogGen = func(string, string, string, string, bool) error {
		return errors.New("simulated changelog error")
	}
	versionBumpCalled := 0
	steps.VersionBump = func(string, string, bool) error {
		versionBumpCalled++
		return nil
	}

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed", err)
	}
	if versionBumpCalled != 0 {
		t.Errorf("VersionBump called %d times after changelog-gen failure (want 0)", versionBumpCalled)
	}
	if !contains(res.StepsFailed, "changelog-gen") {
		t.Errorf("StepsFailed = %v, want contains 'changelog-gen'", res.StepsFailed)
	}
}

func TestRun_VersionBumpFails(t *testing.T) {
	steps := allOkSteps()
	steps.VersionBump = func(string, string, bool) error {
		return errors.New("simulated version-bump error")
	}
	rebuildCalled := 0
	steps.RebuildBinary = func(string, string, bool) error {
		rebuildCalled++
		return nil
	}

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed", err)
	}
	if rebuildCalled != 0 {
		t.Errorf("RebuildBinary called %d times after version-bump failure (want 0)", rebuildCalled)
	}
	if !contains(res.StepsFailed, "version-bump") {
		t.Errorf("StepsFailed = %v, want contains 'version-bump'", res.StepsFailed)
	}
}

func TestRun_ReleaseShFails(t *testing.T) {
	steps := allOkSteps()
	steps.ReleaseSh = func(string, string) error {
		return errors.New("consistency check failed")
	}
	shipCalled := 0
	steps.Ship = func(string, string, string) (string, error) {
		shipCalled++
		return "sha", nil
	}

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed", err)
	}
	if shipCalled != 0 {
		t.Errorf("Ship called %d times after release-sh failure (want 0)", shipCalled)
	}
	if !contains(res.StepsFailed, "release-sh-check") {
		t.Errorf("StepsFailed = %v, want contains 'release-sh-check'", res.StepsFailed)
	}
}

func TestRun_JournalInitFails_UnwritableDir(t *testing.T) {
	dir := t.TempDir()
	blockingFile := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	impossibleDir := filepath.Join(blockingFile, "release-journal")

	_, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    dir,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		JournalDir:  impossibleDir,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed on journal init failure", err)
	}
	if !strings.Contains(err.Error(), "journal init") {
		t.Errorf("err = %q, want contains 'journal init'", err.Error())
	}
}

func TestRun_DryRun_JournalHasDryRunSteps(t *testing.T) {
	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		DryRun:      true,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("dry-run err = %v", err)
	}
	body, err := os.ReadFile(res.JournalPath)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	if !strings.Contains(string(body), "skipped-dry-run") {
		t.Error("dry-run journal must contain 'skipped-dry-run' status records")
	}
}
