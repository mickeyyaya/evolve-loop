package releasepreflight

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	ContentReviewRecordPath = "docs/operations/release-content-review.json"
	ContentReviewSeat       = "docs-review"
	contentReviewVerdict    = "PASS"
)

type contentReviewRecord struct {
	Release string `json:"release"`
	RunID   string `json:"run_id"`
}

func needsContentReview(current, target string) bool {
	cMaj, cMin, _, okC := ParseSemver(current)
	tMaj, tMin, _, okT := ParseSemver(target)
	return okC && okT && (cMaj != tMaj || cMin != tMin)
}

func (p *preflightRun) gateContentReview() error {
	p.logf("content review of this minor release recorded?")
	if !needsContentReview(p.res.CurrentVersion, p.o.target) {
		p.logf("OK: %s → %s is a patch release; no content review needed", p.res.CurrentVersion, p.o.target)
		return nil
	}
	if p.o.skipContentReview != "" {
		p.logf("OVERRIDE: content review skipped — reason: %s", p.o.skipContentReview)
		return nil
	}
	if err := verifyContentReview(p.o.repoRoot, p.o.target); err != nil {
		return fmt.Errorf("%w: content review gate: %v — run `evolve agent run docs-review`, commit %s, or pass --skip-content-review <reason> to evolve release",
			ErrCheckFailed, err, ContentReviewRecordPath)
	}
	p.logf("OK: content review recorded for %s", p.o.target)
	return nil
}

func verifyContentReview(repoRoot, target string) error {
	rec, err := readCommittedReviewRecord(repoRoot)
	if err != nil {
		return err
	}
	if rec.Release != target {
		return fmt.Errorf("the record reviews release %q, not %s", rec.Release, target)
	}
	run, err := launchrecord.Read(filepath.Join(repoRoot, ".evolve"), rec.RunID)
	if err != nil {
		return fmt.Errorf("the record names no real run: %v", err)
	}
	if run.Profile != ContentReviewSeat {
		return fmt.Errorf("run %s is seat %q, not %s", rec.RunID, run.Profile, ContentReviewSeat)
	}
	if run.Verdict != contentReviewVerdict {
		return fmt.Errorf("run %s verdict is %q, not %s", rec.RunID, run.Verdict, contentReviewVerdict)
	}
	return requireRunAfterPreviousTag(repoRoot, run)
}

func readCommittedReviewRecord(repoRoot string) (contentReviewRecord, error) {
	var rec contentReviewRecord
	out, err := sysexec.Command(context.Background(), "git", "-C", repoRoot, "show", "HEAD:"+ContentReviewRecordPath).Output()
	if err != nil {
		return rec, fmt.Errorf("no committed %s (%v)", ContentReviewRecordPath, err)
	}
	if err := json.Unmarshal(out, &rec); err != nil {
		return rec, fmt.Errorf("committed %s is not valid JSON: %v", ContentReviewRecordPath, err)
	}
	return rec, nil
}

func requireRunAfterPreviousTag(repoRoot string, run launchrecord.Record) error {
	tagOut, err := sysexec.Command(context.Background(), "git", "-C", repoRoot, "describe", "--tags", "--abbrev=0").Output()
	if err != nil {
		return nil
	}
	tag := strings.TrimSpace(string(tagOut))
	dateOut, err := sysexec.Command(context.Background(), "git", "-C", repoRoot, "log", "-1", "--format=%cI", tag).Output()
	if err != nil {
		return fmt.Errorf("cannot read the commit date of %s: %v", tag, err)
	}
	tagDate, err := time.Parse(time.RFC3339, strings.TrimSpace(string(dateOut)))
	if err != nil {
		return fmt.Errorf("commit date of %s is not RFC3339: %v", tag, err)
	}
	ts, err := time.Parse(time.RFC3339, run.TS)
	if err != nil {
		return fmt.Errorf("run %s has no valid ts: %v", run.RunID, err)
	}
	if !ts.After(tagDate) {
		return fmt.Errorf("run %s ran at %s, not after the previous tag %s (%s): it reviewed an earlier release",
			run.RunID, run.TS, tag, tagDate.UTC().Format(time.RFC3339))
	}
	return nil
}
