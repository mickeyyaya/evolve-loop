package releasepipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stepFailure(failing string, boom error) Steps {
	s := allOkSteps()
	switch failing {
	case "full-dry-run-preflight":
		s.FullDryRunPreflight = func(string, string) error { return boom }
	case "preflight":
		s.Preflight = func(string, string, bool, bool) error { return boom }
	case "changelog-gen":
		s.ChangelogGen = func(string, string, string, string, bool) error { return boom }
	case "version-bump":
		s.VersionBump = func(string, string, bool) error { return boom }
	case "rebuild-binary":
		s.RebuildBinary = func(string, string, bool) error { return boom }
	case "release-sh-check":
		s.ReleaseSh = func(string, string) error { return boom }
	}
	return s
}

func readJournalAt(t *testing.T, path string) Journal {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	var j Journal
	if err := json.Unmarshal(body, &j); err != nil {
		t.Fatalf("unmarshal journal: %v", err)
	}
	return j
}

func TestRun_PrePublishStepFailure_WrapsLabelAndJournals(t *testing.T) {
	boom := errors.New("simulated")
	for _, tc := range []struct {
		journalName   string
		wantErr       string
		wantCompleted []string
	}{
		{
			journalName:   "full-dry-run-preflight",
			wantErr:       "releasepipeline: pre-publish step failed: full-dry-run preflight: simulated",
			wantCompleted: nil,
		},
		{
			journalName:   "preflight",
			wantErr:       "releasepipeline: pre-publish step failed: preflight: simulated",
			wantCompleted: []string{"full-dry-run-preflight"},
		},
		{
			journalName:   "changelog-gen",
			wantErr:       "releasepipeline: pre-publish step failed: changelog-gen: simulated",
			wantCompleted: []string{"full-dry-run-preflight", "preflight"},
		},
		{
			journalName:   "version-bump",
			wantErr:       "releasepipeline: pre-publish step failed: version-bump: simulated",
			wantCompleted: []string{"full-dry-run-preflight", "preflight", "changelog-gen"},
		},
		{
			journalName:   "rebuild-binary",
			wantErr:       "releasepipeline: pre-publish step failed: rebuild-binary: simulated",
			wantCompleted: []string{"full-dry-run-preflight", "preflight", "changelog-gen", "version-bump"},
		},
		{
			journalName:   "release-sh-check",
			wantErr:       "releasepipeline: pre-publish step failed: release.sh consistency: simulated",
			wantCompleted: []string{"full-dry-run-preflight", "preflight", "changelog-gen", "version-bump", "rebuild-binary"},
		},
	} {
		t.Run(tc.journalName, func(t *testing.T) {
			res, err := Run(Options{
				Target:           "1.2.3",
				RepoRoot:         t.TempDir(),
				FromTag:          "v1.2.2",
				RequirePreflight: true,
				MaxPollWait:      time.Second,
				Now:              fixedNow(t),
				Steps:            stepFailure(tc.journalName, boom),
			})
			if err == nil {
				t.Fatalf("want %s to fail, got nil", tc.journalName)
			}
			if err.Error() != tc.wantErr {
				t.Errorf("err = %q\nwant %q", err.Error(), tc.wantErr)
			}
			if !errors.Is(err, ErrPrePublishFailed) {
				t.Errorf("err must wrap ErrPrePublishFailed (the CLI maps it to exit 1)")
			}
			if !equalStrings(res.StepsFailed, []string{tc.journalName}) {
				t.Errorf("StepsFailed = %v, want [%s]", res.StepsFailed, tc.journalName)
			}
			if !equalStrings(res.StepsCompleted, tc.wantCompleted) {
				t.Errorf("StepsCompleted = %v, want %v", res.StepsCompleted, tc.wantCompleted)
			}

			j := readJournalAt(t, res.JournalPath)
			if len(j.Steps) == 0 {
				t.Fatal("journal has no step records")
			}
			last := j.Steps[len(j.Steps)-1]
			if last.Step != tc.journalName {
				t.Errorf("journal last step = %q, want %q", last.Step, tc.journalName)
			}
			if last.Status != "fail" {
				t.Errorf("journal last status = %q, want fail", last.Status)
			}
			if last.Note != "simulated" {
				t.Errorf("journal note = %q, want the step's own error %q", last.Note, "simulated")
			}
			if j.CompletedAt != "" {
				t.Errorf("completed_at = %q, want empty on a failed run", j.CompletedAt)
			}
		})
	}
}

func TestRun_DryRun_StepLedgerExact(t *testing.T) {
	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		JournalDir:  t.TempDir(),
		FromTag:     "v1.2.2",
		DryRun:      true,
		MaxPollWait: time.Second,
		Now:         fixedNow(t),
		Steps:       allOkSteps(),
	})
	if err != nil {
		t.Fatalf("dry run err = %v", err)
	}
	if want := []string{"preflight", "changelog-gen", "version-bump"}; !equalStrings(res.StepsCompleted, want) {
		t.Errorf("StepsCompleted = %v, want %v — a skipped-dry-run step must NOT count as completed", res.StepsCompleted, want)
	}
	if len(res.StepsFailed) != 0 {
		t.Errorf("StepsFailed = %v, want none", res.StepsFailed)
	}
	if res.NewCommitSHA != "" {
		t.Errorf("NewCommitSHA = %q, want empty — a dry run ships nothing", res.NewCommitSHA)
	}

	j := readJournalAt(t, res.JournalPath)
	type rec struct{ step, status string }
	var got []rec
	for _, s := range j.Steps {
		got = append(got, rec{s.Step, s.Status})
	}
	want := []rec{
		{"preflight", "ok"},
		{"changelog-gen", "ok"},
		{"version-bump", "ok"},
		{"rebuild-binary", "skipped-dry-run"},
		{"release-sh-check", "skipped-dry-run"},
		{"ship", "skipped-dry-run"},
	}
	if len(got) != len(want) {
		t.Fatalf("journal steps = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("journal step %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if j.CompletedAt != "" {
		t.Errorf("completed_at = %q, want empty — a dry run returns before completion is stamped", j.CompletedAt)
	}
}

func TestReleaseNotes_BannerOnlyWhenNotesExist(t *testing.T) {
	t.Run("no changelog entry yields no notes and no banner", func(t *testing.T) {
		r := &releaseRun{
			opts: Options{RepoRoot: t.TempDir(), Target: "9.9.9"},
			logf: func(string, ...any) {},
		}
		if notes := r.releaseNotes(); notes != "" {
			t.Errorf("releaseNotes = %q, want empty — no entry means no banner either", notes)
		}
	})

	t.Run("a real entry is prefixed with the release-class banner", func(t *testing.T) {
		repo := t.TempDir()
		body := "# Changelog\n\n## [1.2.3] - 2026-05-24\n\n### Added\n\n- Feature A\n"
		if err := os.WriteFile(filepath.Join(repo, "CHANGELOG.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		var logged []string
		r := &releaseRun{
			opts:    Options{RepoRoot: repo, Target: "1.2.3"},
			fromTag: "v1.2.2",
			logf:    func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) },
		}
		notes := r.releaseNotes()
		if !strings.Contains(notes, "Feature A") {
			t.Errorf("releaseNotes lost the changelog body: %q", notes)
		}
		if !strings.HasPrefix(notes, "**Release class:") {
			t.Errorf("notes not prefixed by a release-class banner: %q", notes)
		}
		var warned bool
		for _, l := range logged {
			if strings.Contains(l, "release-class classification failed") {
				warned = true
			}
		}
		if !warned {
			t.Errorf("a failed classification must be logged, not silently dropped; logs=%v", logged)
		}
	})
}
