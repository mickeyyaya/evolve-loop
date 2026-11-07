package releasepreflight

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
)

func TestNeedsContentReview_OnlyAMinorOrMajorBump(t *testing.T) {
	cases := []struct {
		current, target string
		want            bool
	}{
		{"23.2.1", "23.2.2", false},
		{"23.2.1", "23.3.0", true},
		{"23.2.1", "24.0.0", true},
		{"bad", "23.3.0", false},
	}
	for _, c := range cases {
		if got := needsContentReview(c.current, c.target); got != c.want {
			t.Errorf("needsContentReview(%q,%q)=%v, want %v", c.current, c.target, got, c.want)
		}
	}
}

func TestRun_MinorBumpWithoutRecordIsRefusedAndNamesTheRecordPath(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	opts := stubOpts(r, "1.1.0")

	_, err := Run(opts)

	if !errors.Is(err, ErrCheckFailed) || !strings.Contains(err.Error(), ContentReviewRecordPath) {
		t.Fatalf("want ErrCheckFailed naming %s, got %v", ContentReviewRecordPath, err)
	}
}

func TestRun_SkipContentReviewPassesAndLogsTheReason(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	var buf bytes.Buffer
	opts := stubOpts(r, "1.1.0")
	opts.Stderr = &buf
	opts.SkipContentReview = "models down"

	if _, err := Run(opts); err != nil {
		t.Fatalf("bypass must pass: %v", err)
	}
	if !strings.Contains(buf.String(), "models down") {
		t.Errorf("log must carry the reason, got %s", buf.String())
	}
}

func TestContentReviewSeat_IsTheDocsReviewSeat(t *testing.T) {
	if ContentReviewSeat != "docs-review" {
		t.Errorf("ContentReviewSeat=%q", ContentReviewSeat)
	}
}

type reviewFixture struct {
	repo   *gittest.Repo
	runTS  string
	runID  string
	record string
}

func newReviewFixture(t *testing.T) *reviewFixture {
	t.Helper()
	r := gittest.Fixture(t)
	r.Write("README.md", "seed\n")
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "seed")
	r.Git("tag", "v1.0.0")
	return &reviewFixture{
		repo:  r,
		runTS: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		runID: "agent-docs-review-20261107T000000Z-0001",
	}
}

func (f *reviewFixture) commitRecord(t *testing.T, body string) {
	t.Helper()
	f.repo.Write(ContentReviewRecordPath, body)
	f.repo.Git("add", "-A")
	f.repo.Git("commit", "-q", "-m", "record")
}

func (f *reviewFixture) writeRun(t *testing.T, profile, verdict, ts string) {
	t.Helper()
	rec := launchrecord.Record{RunID: f.runID, Profile: profile, Verdict: verdict, TS: ts}
	if _, err := launchrecord.Write(filepath.Join(f.repo.Dir, ".evolve"), rec); err != nil {
		t.Fatalf("launchrecord.Write: %v", err)
	}
}

func (f *reviewFixture) recordFor(release string) string {
	return fmt.Sprintf(`{"release":%q,"run_id":%q}`, release, f.runID)
}

func TestVerifyContentReview_RefusesEachDefectOfTheRecordOrItsRun(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, f *reviewFixture)
		wantErr string
	}{
		{"no committed record", func(t *testing.T, f *reviewFixture) {}, "no committed"},
		{"record is not JSON", func(t *testing.T, f *reviewFixture) { f.commitRecord(t, "{") }, "not valid JSON"},
		{"record reviews another release", func(t *testing.T, f *reviewFixture) {
			f.commitRecord(t, f.recordFor("1.0.0"))
		}, "not 1.1.0"},
		{"record names no real run", func(t *testing.T, f *reviewFixture) {
			f.commitRecord(t, f.recordFor("1.1.0"))
		}, "no real run"},
		{"run is another seat", func(t *testing.T, f *reviewFixture) {
			f.commitRecord(t, f.recordFor("1.1.0"))
			f.writeRun(t, "go-reviewer", "PASS", f.runTS)
		}, "not docs-review"},
		{"run verdict is not PASS", func(t *testing.T, f *reviewFixture) {
			f.commitRecord(t, f.recordFor("1.1.0"))
			f.writeRun(t, ContentReviewSeat, "FAIL", f.runTS)
		}, "not PASS"},
		{"run predates the previous tag", func(t *testing.T, f *reviewFixture) {
			f.commitRecord(t, f.recordFor("1.1.0"))
			f.writeRun(t, ContentReviewSeat, "PASS", "2000-01-01T00:00:00Z")
		}, "earlier release"},
		{"run has no valid ts", func(t *testing.T, f *reviewFixture) {
			f.commitRecord(t, f.recordFor("1.1.0"))
			f.writeRun(t, ContentReviewSeat, "PASS", "yesterday")
		}, "no valid ts"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newReviewFixture(t)
			c.setup(t, f)

			err := verifyContentReview(f.repo.Dir, "1.1.0")

			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("want error containing %q, got %v", c.wantErr, err)
			}
		})
	}
}

func TestVerifyContentReview_AcceptsAFreshPassingRunOfTheSeat(t *testing.T) {
	f := newReviewFixture(t)
	f.commitRecord(t, f.recordFor("1.1.0"))
	f.writeRun(t, ContentReviewSeat, "PASS", f.runTS)

	if err := verifyContentReview(f.repo.Dir, "1.1.0"); err != nil {
		t.Fatalf("a fresh passing run must be accepted: %v", err)
	}
}

func TestVerifyContentReview_AcceptsARunWhenNoTagExistsYet(t *testing.T) {
	r := gittest.Fixture(t)
	r.Write(ContentReviewRecordPath, `{"release":"0.1.0","run_id":"agent-docs-review-20261107T000000Z-0002"}`)
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "record")
	rec := launchrecord.Record{RunID: "agent-docs-review-20261107T000000Z-0002", Profile: ContentReviewSeat, Verdict: "PASS", TS: "2000-01-01T00:00:00Z"}
	if _, err := launchrecord.Write(filepath.Join(r.Dir, ".evolve"), rec); err != nil {
		t.Fatal(err)
	}

	if err := verifyContentReview(r.Dir, "0.1.0"); err != nil {
		t.Fatalf("with no previous tag there is no earlier release to replay: %v", err)
	}
}

func TestGateContentReview_RecordsSuccessForAValidReview(t *testing.T) {
	f := newReviewFixture(t)
	f.commitRecord(t, f.recordFor("1.1.0"))
	f.writeRun(t, ContentReviewSeat, "PASS", f.runTS)
	var buf bytes.Buffer
	p := &preflightRun{
		o:    resolved{target: "1.1.0", repoRoot: f.repo.Dir},
		res:  Result{CurrentVersion: "1.0.0"},
		logf: func(format string, args ...any) { fmt.Fprintf(&buf, format+"\n", args...) },
	}

	if err := p.gateContentReview(); err != nil {
		t.Fatalf("gateContentReview: %v", err)
	}
	if !strings.Contains(buf.String(), "content review recorded for 1.1.0") {
		t.Errorf("log lacks the success line: %s", buf.String())
	}
}

func fakeGit(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestRequireRunAfterPreviousTag_RefusesWhenGitCannotDateTheTag(t *testing.T) {
	cases := []struct {
		name, script, wantErr string
	}{
		{"log fails", `case "$*" in *describe*) echo v1;; *) exit 1;; esac`, "cannot read the commit date"},
		{"date is not RFC3339", `case "$*" in *describe*) echo v1;; *) echo notadate;; esac`, "not RFC3339"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeGit(t, c.script)

			err := requireRunAfterPreviousTag(t.TempDir(), launchrecord.Record{RunID: "r", TS: "2026-11-07T00:00:00Z"})

			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("want error containing %q, got %v", c.wantErr, err)
			}
		})
	}
}
