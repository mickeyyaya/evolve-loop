package releasepipeline

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRun_StepFailureWithBrokenJournalKeepsBothErrors(t *testing.T) {
	stepErr := errors.New("step blew up")
	cases := []struct {
		name            string
		failStepIn      func(s *Steps, breakJournal func())
		wantErrIs       error
		wantRollback    bool
		wantStepsFailed string
	}{
		{
			name: "preflight fails",
			failStepIn: func(s *Steps, breakJournal func()) {
				s.Preflight = func(string, string, bool, bool) error { breakJournal(); return stepErr }
			},
			wantErrIs:       ErrPrePublishFailed,
			wantStepsFailed: "preflight",
		},
		{
			name: "ship fails",
			failStepIn: func(s *Steps, breakJournal func()) {
				s.Ship = func(string, string, string) (string, error) { breakJournal(); return "", stepErr }
			},
			wantErrIs:       ErrShipFailed,
			wantStepsFailed: "ship",
		},
		{
			name: "marketplace-poll fails",
			failStepIn: func(s *Steps, breakJournal func()) {
				s.MarketplacePoll = func(string, string, time.Duration) error { breakJournal(); return stepErr }
			},
			wantErrIs:       ErrPostPublishFailed,
			wantRollback:    true,
			wantStepsFailed: "marketplace-poll",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			journalDir := filepath.Join(t.TempDir(), "journals")
			steps := allOkSteps()
			tc.failStepIn(&steps, func() { replaceDirWithFile(t, journalDir) })
			rollbackCalls := 0
			steps.Rollback = func(string, string, string) error { rollbackCalls++; return nil }

			res, err := Run(Options{
				Target:      "1.2.3",
				RepoRoot:    t.TempDir(),
				FromTag:     "v1.2.2",
				JournalDir:  journalDir,
				MaxPollWait: time.Second,
				Now:         fixedNow(t),
				Steps:       steps,
			})

			if !errors.Is(err, tc.wantErrIs) {
				t.Fatalf("Run error = %v, want it to wrap %v", err, tc.wantErrIs)
			}
			if !strings.Contains(err.Error(), stepErr.Error()) {
				t.Errorf("Run error %q lost the step failure %q", err, stepErr)
			}
			if !strings.Contains(err.Error(), journalDir) {
				t.Errorf("Run error %q does not name the broken journal location %s", err, journalDir)
			}
			if got := (rollbackCalls > 0); got != tc.wantRollback {
				t.Errorf("rollback ran = %v, want %v", got, tc.wantRollback)
			}
			if !slices.Contains(res.StepsFailed, tc.wantStepsFailed) {
				t.Errorf("StepsFailed = %v, want it to hold %q", res.StepsFailed, tc.wantStepsFailed)
			}
		})
	}
}

func TestRun_ShipJournalWriteFailureIsPostPublishAndKeepsTheCommit(t *testing.T) {
	journalDir := filepath.Join(t.TempDir(), "journals")
	steps := allOkSteps()
	steps.Ship = func(string, string, string) (string, error) {
		replaceDirWithFile(t, journalDir)
		return "deadbeef1234567890", nil
	}
	rollbackCalls := 0
	steps.Rollback = func(string, string, string) error { rollbackCalls++; return nil }

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.2.2",
		JournalDir:  journalDir,
		MaxPollWait: time.Second,
		Now:         fixedNow(t),
		Steps:       steps,
	})

	if !errors.Is(err, ErrPostPublishFailed) {
		t.Fatalf("Run error = %v, want ErrPostPublishFailed: the release is already pushed", err)
	}
	if res.NewCommitSHA != "deadbeef1234567890" {
		t.Errorf("NewCommitSHA = %q, want the pushed commit so the operator can remediate", res.NewCommitSHA)
	}
	if rollbackCalls != 0 {
		t.Errorf("rollback ran %d time(s) on a journal-only failure, want 0", rollbackCalls)
	}
}
