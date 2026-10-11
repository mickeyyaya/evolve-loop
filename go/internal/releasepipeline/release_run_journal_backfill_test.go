package releasepipeline

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func unwritableJournalPath(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(file, "journal.json")
}

func directRun(t *testing.T, journalPath string, dryRun bool, steps Steps) (*releaseRun, *bytes.Buffer) {
	t.Helper()
	var log bytes.Buffer
	return &releaseRun{
		opts:        Options{Target: "1.2.3", RepoRoot: t.TempDir(), DryRun: dryRun},
		steps:       steps,
		now:         fixedNow(t),
		logf:        releaseLogf(&log),
		journal:     &Journal{},
		journalPath: journalPath,
		res:         Result{Target: "1.2.3"},
	}, &log
}

func TestReleaseRunShip_DryRunJournalFailureIsAPrePublishFailure(t *testing.T) {
	shipCalls := 0
	steps := allOkSteps()
	steps.Ship = func(string, string, string) (string, error) { shipCalls++; return "x", nil }
	r, log := directRun(t, unwritableJournalPath(t), true, steps)

	shipped, err := r.ship()

	if shipped || !errors.Is(err, ErrPrePublishFailed) || !strings.Contains(err.Error(), "ship") {
		t.Fatalf("ship() = (%v, %v), want (false, ErrPrePublishFailed naming ship)", shipped, err)
	}
	if shipCalls != 0 {
		t.Errorf("Ship ran %d time(s) in a dry run", shipCalls)
	}
	if strings.Contains(log.String(), "DRY RUN COMPLETE") {
		t.Errorf("log = %q, must not claim a completed dry run after the journal failed", log.String())
	}
}

func TestReleaseRunShip_DryRunRecordsTheSkipAndShipsNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.json")
	r, log := directRun(t, path, true, allOkSteps())

	shipped, err := r.ship()

	if shipped || err != nil {
		t.Fatalf("ship() = (%v, %v), want (false, nil)", shipped, err)
	}
	if n := len(r.journal.Steps); n != 1 || r.journal.Steps[0] != (StepRecord{Step: "ship", Status: "skipped-dry-run", Timestamp: r.journal.Steps[0].Timestamp}) {
		t.Errorf("journal steps = %+v, want one skipped-dry-run ship step", r.journal.Steps)
	}
	if !strings.Contains(log.String(), "commit msg: release: v1.2.3") {
		t.Errorf("log = %q, want the would-be commit message", log.String())
	}
}

func TestReleaseRunShip_CommitSHAJournalFailureIsAPostPublishFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.json")
	steps := allOkSteps()
	steps.Ship = func(string, string, string) (string, error) { return "cafef00d", nil }
	r, _ := directRun(t, path, false, steps)
	prev := writeJournal
	t.Cleanup(func() { writeJournal = prev })
	writes := 0
	writeJournal = func(j *Journal, p string) error {
		writes++
		if writes == 2 {
			return errors.New("disk full")
		}
		return prev(j, p)
	}

	shipped, err := r.ship()

	if shipped || !errors.Is(err, ErrPostPublishFailed) || !strings.Contains(err.Error(), "ship: journal") || !strings.Contains(err.Error(), "field commit_sha: disk full") {
		t.Fatalf("ship() = (%v, %v), want (false, ErrPostPublishFailed for the commit_sha field)", shipped, err)
	}
	if r.res.NewCommitSHA != "cafef00d" || len(r.res.StepsCompleted) != 1 || r.res.StepsCompleted[0] != "ship" {
		t.Errorf("res = %+v, want the shipped SHA recorded even though the journal field failed", r.res)
	}
}

func TestReleaseRunComplete(t *testing.T) {
	t.Run("journal failure is a post-publish failure", func(t *testing.T) {
		r, log := directRun(t, unwritableJournalPath(t), false, allOkSteps())

		err := r.complete()

		if !errors.Is(err, ErrPostPublishFailed) || !strings.Contains(err.Error(), "complete: journal") {
			t.Fatalf("complete() = %v, want ErrPostPublishFailed: complete", err)
		}
		if strings.Contains(log.String(), "DONE:") {
			t.Errorf("log = %q, must not announce DONE after the journal failed", log.String())
		}
	})
	t.Run("success stamps completed_at and announces DONE", func(t *testing.T) {
		r, log := directRun(t, filepath.Join(t.TempDir(), "journal.json"), false, allOkSteps())

		err := r.complete()

		if err != nil {
			t.Fatalf("complete() = %v", err)
		}
		if _, perr := time.Parse(time.RFC3339, r.journal.CompletedAt); perr != nil {
			t.Errorf("CompletedAt = %q, want RFC3339", r.journal.CompletedAt)
		}
		if !strings.Contains(log.String(), "DONE: v1.2.3 shipped, propagated, and verified") {
			t.Errorf("log = %q, want the DONE line", log.String())
		}
	})
}
