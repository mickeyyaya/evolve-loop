package releasepipeline

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func replaceDirWithFile(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove journal dir %s: %v", dir, err)
	}
	if err := os.WriteFile(dir, []byte("a file where the journal directory was"), 0o644); err != nil {
		t.Fatalf("put a file at the journal dir %s: %v", dir, err)
	}
}

type journalBreakCase struct {
	name           string
	breakJournalIn func(steps *Steps, breakJournal func())
	wantErrIs      error
	shipMustNotRun bool
}

func journalBreakCases() []journalBreakCase {
	return []journalBreakCase{
		{
			name:           "journal stays writable",
			breakJournalIn: func(*Steps, func()) {},
		},
		{
			name: "breaks during preflight",
			breakJournalIn: func(s *Steps, breakJournal func()) {
				s.Preflight = func(string, string, bool, bool) error { breakJournal(); return nil }
			},
			wantErrIs:      ErrPrePublishFailed,
			shipMustNotRun: true,
		},
		{
			name: "breaks during ship",
			breakJournalIn: func(s *Steps, breakJournal func()) {
				s.Ship = func(string, string, string) (string, error) { breakJournal(); return "deadbeef1234567890", nil }
			},
		},
		{
			name: "breaks during marketplace-poll",
			breakJournalIn: func(s *Steps, breakJournal func()) {
				s.MarketplacePoll = func(string, string, time.Duration) error { breakJournal(); return nil }
			},
		},
		{
			name: "breaks during release-verify",
			breakJournalIn: func(s *Steps, breakJournal func()) {
				s.ReleaseVerify = func(string, string, string) error { breakJournal(); return nil }
			},
		},
	}
}

func TestRun_JournalWriteError(t *testing.T) {
	for _, tc := range journalBreakCases() {
		t.Run(tc.name, func(t *testing.T) {
			journalDir := filepath.Join(t.TempDir(), "journals")
			isBroken := false
			steps := allOkSteps()
			tc.breakJournalIn(&steps, func() {
				replaceDirWithFile(t, journalDir)
				isBroken = true
			})
			shipCalls := 0
			ship := steps.Ship
			steps.Ship = func(root, msg, notes string) (string, error) {
				shipCalls++
				return ship(root, msg, notes)
			}

			res, err := Run(Options{
				Target:      "1.2.3",
				RepoRoot:    t.TempDir(),
				FromTag:     "v1.2.2",
				JournalDir:  journalDir,
				MaxPollWait: time.Second,
				Now:         fixedNow(t),
				Steps:       steps,
			})

			if !isBroken {
				if err != nil {
					t.Fatalf("Run with a writable journal = %v, want nil", err)
				}
				if res.JournalPath == "" {
					t.Fatal("Run with a writable journal reported no journal path")
				}
				return
			}
			if err == nil {
				t.Fatalf("Run = nil after the journal write failed; the journal that rollback reads is incomplete and nobody was told (completed %v)", res.StepsCompleted)
			}
			if !strings.Contains(err.Error(), journalDir) {
				t.Errorf("Run error %q does not name the journal location %s", err, journalDir)
			}
			if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
				t.Errorf("Run error = %v, want it to wrap %v", err, tc.wantErrIs)
			}
			if tc.shipMustNotRun && shipCalls != 0 {
				t.Errorf("Ship ran %d time(s) after a pre-publish journal write failed, want 0", shipCalls)
			}
		})
	}
}

func TestResolveInitCommit_EmptyGitOutputIsAnError(t *testing.T) {
	cases := []struct {
		name       string
		gitOutput  string
		wantCommit string
		wantErr    bool
	}{
		{name: "empty output", gitOutput: "", wantErr: true},
		{name: "whitespace only", gitOutput: "\n  \n", wantErr: true},
		{name: "one root commit", gitOutput: "abc1234\n", wantCommit: "abc1234"},
		{name: "two root commits", gitOutput: "abc1234\ndef5678\n", wantCommit: "abc1234"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeGitDir := t.TempDir()
			script := "#!/bin/sh\nprintf '%s' '" + tc.gitOutput + "'\n"
			if err := os.WriteFile(filepath.Join(fakeGitDir, "git"), []byte(script), 0o755); err != nil {
				t.Fatalf("write fake git: %v", err)
			}
			t.Setenv("PATH", fakeGitDir)

			commit, err := resolveInitCommit(t.TempDir())

			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveInitCommit with git output %q = (%q, nil), want an error", tc.gitOutput, commit)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveInitCommit with git output %q: %v", tc.gitOutput, err)
			}
			if commit != tc.wantCommit {
				t.Errorf("resolveInitCommit = %q, want %q", commit, tc.wantCommit)
			}
		})
	}
}
