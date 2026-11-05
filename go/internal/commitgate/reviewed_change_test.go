package commitgate

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cloudlaunch"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
)

const againNext = "then run record-pr again."

func recordOf(att Attestation, launches ...launchrecord.Record) ReviewRecord {
	att.TreeStateSHA = stagedTree()
	return ReviewRecord{Attestation: att, LaunchRecords: launches}
}

func TestCheckReviewedChangeJudgesTheRecordWithNoDisk(t *testing.T) {
	t.Parallel()
	code := []string{"a.go"}
	both := []launchrecord.Record{reviewRun("r-simp", "code-simplifier"), reviewRun("r-go", "go-reviewer")}
	other := reviewRun("r-other", "code-reviewer")
	other.TreeAfter = "changed"
	wholly := reviewRun("r-simp", "code-simplifier")
	wholly.TreeBefore, wholly.TreeAfter = "elsewhere", "elsewhere"
	cloud := reviewRun("r-cloud", "code-reviewer")
	cloud.Cloud = &cloudlaunch.Session{}
	cases := map[string]struct {
		rec   ReviewRecord
		paths []string
		want  string
	}{
		"a run named twice":           {recordOf(Attestation{ReviewersRun: []string{"code-simplifier", "go-reviewer"}, ReviewRuns: []ReviewRun{{RunID: "r-simp"}, {RunID: "r-go"}, {RunID: "r-go"}}}, both...), code, ""},
		"a run by list only":          {recordOf(Attestation{ReviewRuns: []ReviewRun{{RunID: "r-simp"}, {RunID: "r-go"}}}, both...), code, ""},
		"a docs change by a docs run": {recordOf(Attestation{ReviewersRun: []string{"docs-review"}, ReviewRuns: []ReviewRun{{RunID: "r-docs"}}}, reviewRun("r-docs", "docs-review")), []string{"docs/a.md"}, ""},
		"a waiver": {
			recordOf(Attestation{ReviewWaiver: "comment-only", ReviewRuns: []ReviewRun{{RunID: "r-simp"}}}, both...), code,
			`the record of change 4d5e6f7 holds the waiver "comment-only", and a waiver is not a review. Next: run the review seats and evolve commit-gate run on this change, ` + againNext,
		},
		"no run": {
			recordOf(Attestation{}), code,
			"the record of change 4d5e6f7 names no review run. Next: run evolve commit-gate run with --reviewers run:<label>, " + againNext,
		},
		"a name with no run": {
			recordOf(Attestation{ReviewersRun: []string{"code-reviewer"}}), code,
			"the record of change 4d5e6f7 names no review run. Next: run evolve commit-gate run with --reviewers run:<label>, " + againNext,
		},
		"a run with no copy": {
			recordOf(Attestation{ReviewRuns: []ReviewRun{{RunID: "ghost"}}}, both...), code,
			"the record of change 4d5e6f7 names review run ghost: no launch record of the run is copied in the record. Next: fix the cause, run the review seats and evolve commit-gate run again, " + againNext,
		},
		"a run of another tree": {
			recordOf(Attestation{ReviewRuns: []ReviewRun{{RunID: "r-other"}}}, other), code,
			"the record of change 4d5e6f7 names review run r-other: the review did not see the staged tree (the tree changed during or after the review). Next: fix the cause, run the review seats and evolve commit-gate run again, " + againNext,
		},
		"a run bound wholly to another tree": {
			recordOf(Attestation{ReviewRuns: []ReviewRun{{RunID: "r-simp"}}}, wholly), code,
			"the record of change 4d5e6f7 names review run r-simp: the review did not see the staged tree (the tree changed during or after the review). Next: fix the cause, run the review seats and evolve commit-gate run again, " + againNext,
		},
		"a cloud run": {
			recordOf(Attestation{ReviewRuns: []ReviewRun{{RunID: "r-cloud"}}}, cloud), code,
			"the record of change 4d5e6f7 names review run r-cloud: the run launched a cloud session, so no reviewer read the staged tree. Next: fix the cause, run the review seats and evolve commit-gate run again, " + againNext,
		},
		"one bad run among good": {
			recordOf(Attestation{ReviewRuns: []ReviewRun{{RunID: "r-simp"}, {RunID: "ghost"}}}, both...), code,
			"the record of change 4d5e6f7 names review run ghost: no launch record of the run is copied in the record. Next: fix the cause, run the review seats and evolve commit-gate run again, " + againNext,
		},
		"a simplifier alone proves no review": {
			recordOf(Attestation{ReviewersRun: []string{"code-simplifier", "go-reviewer"}, ReviewRuns: []ReviewRun{{RunID: "r-simp"}}}, both[0]), code,
			"the review runs of change 4d5e6f7 prove no review (profiles: code-simplifier). Next: run a review seat with that capability and evolve commit-gate run with --reviewers run:<label>, " + againNext,
		},
		"a reviewer alone proves no simplify": {
			recordOf(Attestation{ReviewRuns: []ReviewRun{{RunID: "r-go"}}}, both[1]), code,
			"the review runs of change 4d5e6f7 prove no simplify (profiles: go-reviewer). Next: run a review seat with that capability and evolve commit-gate run with --reviewers run:<label>, " + againNext,
		},
		"a go reviewer does not review python": {
			recordOf(Attestation{ReviewRuns: []ReviewRun{{RunID: "r-simp"}, {RunID: "r-go"}}}, both...), []string{"a.py"},
			"the review runs of change 4d5e6f7 prove no review (profiles: go-reviewer, code-simplifier). Next: run a review seat with that capability and evolve commit-gate run with --reviewers run:<label>, " + againNext,
		},
		"a code run does not review docs": {
			recordOf(Attestation{ReviewRuns: []ReviewRun{{RunID: "r-simp"}}}, both[0]), []string{"docs/a.md"},
			"the review runs of change 4d5e6f7 prove no docs review (profiles: code-simplifier). Next: run a review seat with that capability and evolve commit-gate run with --reviewers run:<label>, " + againNext,
		},
	}
	for name, c := range cases {
		if got := CheckReviewedChange(c.rec, "4d5e6f7", c.paths); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", name, got, c.want)
		}
	}
}
