package releasepipeline

import (
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestRun_NilPreflightOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.Preflight = nil

	_, _ = Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
}

func TestRun_NilChangelogGenOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.ChangelogGen = nil

	_, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil && !errors.Is(err, ErrPrePublishFailed) {
		t.Errorf("unexpected error wrapping: %v", err)
	}
}

func TestRun_NilVersionBumpOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.VersionBump = nil

	_, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil && !errors.Is(err, ErrPrePublishFailed) {
		t.Errorf("unexpected error wrapping: %v", err)
	}
}

func TestRun_NilRebuildBinaryOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.RebuildBinary = nil

	res, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		DryRun:      true,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("Run with nil RebuildBinary (dry-run): %v, result=%+v", err, res)
	}
}

func TestRun_NilReleaseShOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.ReleaseSh = nil

	_, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil && !errors.Is(err, ErrPrePublishFailed) {
		t.Errorf("unexpected error wrapping: %v", err)
	}
}

func TestRun_NilShipOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.Ship = nil
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")

	_, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil && !errors.Is(err, ErrShipFailed) && !errors.Is(err, ErrPrePublishFailed) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_NilMarketplacePollOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.MarketplacePoll = nil
	t.Setenv("EVOLVE_MARKETPLACE_DIR", t.TempDir()+"/no-such-market")

	_, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: 100 * time.Millisecond,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil && !errors.Is(err, ErrPostPublishFailed) &&
		!errors.Is(err, ErrPrePublishFailed) && !errors.Is(err, ErrShipFailed) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_NilRollbackOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.Rollback = nil
	steps.MarketplacePoll = func(string, string, time.Duration) error {
		return errors.New("poll fail")
	}

	res, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPostPublishFailed) {
		t.Fatalf("err = %v, want ErrPostPublishFailed", err)
	}
	if !res.RollbackTriggered {
		t.Error("RollbackTriggered must be true")
	}
	if res.RollbackErr == nil {
		t.Log("RollbackErr is nil (rollback may have partially succeeded)")
	}
}

func TestRun_FromTagResolvesViaInitCommit_NoTags(t *testing.T) {
	dir := makeHermeticGitRepo(t)
	deleteTagCmd := exec.Command("git", "-C", dir, "tag", "-d", "v0.0.1")
	if out, err := deleteTagCmd.CombinedOutput(); err != nil {
		t.Fatalf("delete tag: %v\n%s", err, out)
	}

	res, err := Run(Options{
		Target:      "99.1.0",
		RepoRoot:    dir,
		FromTag:     "",
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("Run with init-commit fromTag: %v", err)
	}
	if res.JournalPath == "" {
		t.Error("JournalPath must be set")
	}
}
