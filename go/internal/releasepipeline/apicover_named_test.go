package releasepipeline

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResult_RunBadTargetReturnsSeededResult(t *testing.T) {
	var got Result
	got, err := Run(Options{Target: "not-semver"})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err=%v, want ErrPrePublishFailed", err)
	}
	if got.Target != "not-semver" {
		t.Errorf("Result.Target=%q, want %q", got.Target, "not-semver")
	}
	if len(got.StepsCompleted) != 0 {
		t.Errorf("StepsCompleted=%v, want none (failed before any step)", got.StepsCompleted)
	}
	if got.RollbackTriggered {
		t.Error("RollbackTriggered=true, want false on pre-publish validation failure")
	}
}

func TestOptions_SkipContentReviewJournalsItsReasonBeforePreflight(t *testing.T) {
	var preflightRan bool
	res, err := Run(Options{
		Target:            "1.2.3",
		RepoRoot:          t.TempDir(),
		JournalDir:        t.TempDir(),
		DryRun:            true,
		SkipContentReview: "models down",
		Steps: Steps{
			Preflight:    func(string, string, bool, bool) error { preflightRan = true; return nil },
			ChangelogGen: func(string, string, string, string, bool) error { return nil },
			VersionBump:  func(string, string, bool) error { return nil },
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !preflightRan {
		t.Error("preflight must still run")
	}
	raw, rerr := os.ReadFile(res.JournalPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if !strings.Contains(string(raw), `"content-review-bypass"`) || !strings.Contains(string(raw), "models down") {
		t.Errorf("journal lacks the bypass step with its reason: %s", raw)
	}
}

func TestOptions_SkipContentReviewFailsWhenTheJournalCannotRecordTheBypass(t *testing.T) {
	journalDir := t.TempDir()
	journalFile := filepath.Join(journalDir, "release-pipeline-dryrun-1.2.3.json")
	calls := 0
	now := func() time.Time {
		calls++
		if calls == 2 {
			if err := os.Remove(journalFile); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(journalFile, "x"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		return time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	}

	_, err := Run(Options{
		Target:            "1.2.3",
		RepoRoot:          t.TempDir(),
		JournalDir:        journalDir,
		DryRun:            true,
		Now:               now,
		SkipContentReview: "models down",
		Steps:             allOkSteps(),
	})

	if !errors.Is(err, ErrPrePublishFailed) || !strings.Contains(err.Error(), "content-review-bypass") {
		t.Fatalf("want ErrPrePublishFailed naming content-review-bypass, got %v", err)
	}
}
