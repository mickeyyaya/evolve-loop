package ciwatch

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	gateTestJob  = "go / build + test (Go 1.23, ubuntu, every package and every gate)"
	gateTestStep = "api-coverage enforce (apicover; per-package hard-fail, Phase 5)"
)

func jobsReply(conclusion string) string {
	return `{"jobs":[{"databaseId":7,"name":"` + gateTestJob + `","steps":[{"name":"build","conclusion":"success"},{"name":"` + gateTestStep + `","conclusion":"` + conclusion + `"}]}]}`
}

func TestWithGateRows_AttemptZeroOmitsTheFlagAndNamesAttemptOneInTheCommand(t *testing.T) {
	calls := fakeGH(t, func(args []string) ([]byte, error) { return []byte(jobsReply("failure")), nil })
	st := RunStatus{RunID: 5, FailingTests: []FailingTest{{Step: "make apicover-check", Jobs: []string{gateTestJob}}}}

	got := ghClassifySource{root: "."}.withGateRows(context.Background(), st)

	if want := []string{"run", "view", "5", "--json", "jobs"}; !slices.Equal((*calls)[0], want) {
		t.Errorf("gh args %v, want %v", (*calls)[0], want)
	}
	want := FailingTest{
		Step: "make apicover-check", JobID: 7, Attempt: 1, Jobs: []string{gateTestJob},
		Detail: `step "` + gateTestStep + `" failed; the message was not read; next: gh run view --job 7 --attempt 1 --log-failed | tail -n 2000 (the end of the job log)`,
	}
	if len(got.FailingTests) != 1 || !equalRow(got.FailingTests[0], want) {
		t.Errorf("rows %+v, want [%+v]", got.FailingTests, want)
	}
}

func equalRow(a, b FailingTest) bool {
	return a.Package == b.Package && a.Test == b.Test && a.Step == b.Step && a.Detail == b.Detail &&
		a.JobID == b.JobID && a.Attempt == b.Attempt && slices.Equal(a.Jobs, b.Jobs)
}

func TestWithGateRows_OnlyAStepWithTheConclusionFailureMakesAGateRow(t *testing.T) {
	for _, conclusion := range []string{"cancelled", "skipped", "success", "timed_out", ""} {
		fakeGH(t, func([]string) ([]byte, error) { return []byte(jobsReply(conclusion)), nil })
		rows := []FailingTest{{Test: "TestX", Jobs: []string{gateTestJob}}}

		got := ghClassifySource{root: "."}.withGateRows(context.Background(), RunStatus{RunID: 5, Attempt: 2, FailingTests: rows})

		if len(got.FailingTests) != 1 || got.FailingTests[0].Test != "TestX" || len(got.Warnings) != 0 {
			t.Errorf("conclusion %q: rows %+v warnings %v, want the text rows untouched", conclusion, got.FailingTests, got.Warnings)
		}
	}
}

func TestWithGateRows_AFailedReadKeepsTheRowsAndAddsOneWarningForTheRun(t *testing.T) {
	calls := fakeGH(t, func([]string) ([]byte, error) { return nil, errors.New("HTTP 502") })
	rows := []FailingTest{{Test: "TestX", Jobs: []string{"j"}}}

	got := ghClassifySource{root: "."}.withGateRows(context.Background(), RunStatus{RunID: 5, Attempt: 3, FailingTests: rows})

	if want := []string{"run", "view", "5", "--attempt", "3", "--json", "jobs"}; !slices.Equal((*calls)[0], want) {
		t.Errorf("gh args %v, want %v", (*calls)[0], want)
	}
	if len(got.FailingTests) != 1 || len(got.Warnings) != 1 || !strings.HasPrefix(got.Warnings[0], "run 5: the job API was not read, so gate failures keep the text rows: ") || !strings.Contains(got.Warnings[0], "HTTP 502") {
		t.Errorf("rows %+v warnings %q", got.FailingTests, got.Warnings)
	}
}

func TestReplaceJobRows_TakesOnlyTheGatedJobOutOfASharedRowAndDropsARowLeftWithNoJob(t *testing.T) {
	rows := []FailingTest{
		{Package: "p", Test: "TestShared", Jobs: []string{"a", "b"}},
		{Package: "p", Test: "TestOnlyA", Jobs: []string{"a"}},
		{Step: "make cover-changed", Jobs: []string{"c"}},
	}

	got := replaceJobRows(rows, map[string]FailingTest{"a": {Step: "make cover-changed", Detail: "d"}, "z": {Step: "make other"}})

	if len(got) != 3 {
		t.Fatalf("rows %+v, want 3", got)
	}
	if got[0].Step != "make cover-changed" || !slices.Equal(got[0].Jobs, []string{"a", "c"}) || got[0].Detail != "" {
		t.Errorf("merged row %+v, want the gate row and the row of the same key to share jobs a and c", got[0])
	}
	if got[1].Step != "make other" || !slices.Equal(got[1].Jobs, []string{"z"}) {
		t.Errorf("second row %+v", got[1])
	}
	if got[2].Test != "TestShared" || !slices.Equal(got[2].Jobs, []string{"b"}) {
		t.Errorf("shared row %+v, want only job b left", got[2])
	}
}

func TestReplaceJobRows_NoGatedJobReturnsTheRowsUnchanged(t *testing.T) {
	rows := []FailingTest{{Test: "TestX", Jobs: []string{"a"}}, {Test: "TestNoJob"}}
	if got := replaceJobRows(rows, nil); !slices.EqualFunc(got, rows, equalRow) {
		t.Errorf("rows %+v, want %+v", got, rows)
	}
}

func TestClassifyTarget_CollectsTheWarningsOfEveryRunItReadInOrder(t *testing.T) {
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	row := FailingTest{Package: "m/a", Test: "TestA", Jobs: []string{"j"}}
	head := failed(9, "head", at, row)
	head.Warnings = []string{"head"}
	base := failed(8, "base", at.Add(-time.Hour), row)
	base.Warnings = []string{"base"}
	onMain := failed(7, "older", at.Add(-2*time.Hour), row)
	onMain.Warnings = []string{"main"}
	newer := failed(9, "head", at, row)
	newer.Attempt = 2
	newer.Warnings = []string{"rerun"}
	src := &fakeSource{change: Change{HeadSHA: "head", BaseSHA: "base", Run: head}, byCommit: map[string]RunStatus{"base": base}, main: []RunStatus{onMain}, rerun: newer}

	rep, err := ClassifyTarget(context.Background(), src, Target{Kind: TargetRun, Value: "9"}, ClassifyOptions{MainWindow: 5, Rerun: true})

	if err != nil || !slices.Equal(rep.Warnings, []string{"head", "base", "main", "rerun"}) {
		t.Errorf("err %v warnings %q, want head, base, main, rerun", err, rep.Warnings)
	}
}

func TestWithGateRows_EachOfTheFiveGatesPrintsItsWholeDetail(t *testing.T) {
	cases := []struct{ job, step, gate, bound string }{
		{"go / build + test (Go 1.23, ubuntu, every package and every gate)", "changed functions at 100% (cover-changed; hard-fail)", "make cover-changed", "200"},
		{"go / build + test (Go 1.23, ubuntu, every package and every gate)", "api-coverage enforce (apicover; per-package hard-fail, Phase 5)", "make apicover-check", "2000"},
		{"go / build + test (Go 1.23, ubuntu, every package and every gate)", "line-coverage strict (go/.cover-strict; per-package hard-fail, ADR-0101)", "make cover-strict", "1200"},
		{"go / build + test (Go 1.23, macOS, the darwin package set)", "changed functions of the darwin set at 100% (cover-changed; hard-fail)", "make cover-changed-darwin", "200"},
		{"go / build + test (Go 1.23, macOS, the darwin package set)", "line-coverage strict of the darwin set (go/.cover-strict floors of go/.darwin-set packages; hard-fail)", "make cover-strict-darwin", "1200"},
	}
	for _, tc := range cases {
		reply := `{"jobs":[{"databaseId":42,"name":"` + tc.job + `","steps":[{"name":"` + tc.step + `","conclusion":"failure"}]}]}`
		fakeGH(t, func([]string) ([]byte, error) { return []byte(reply), nil })

		got := ghClassifySource{root: "."}.withGateRows(context.Background(), RunStatus{RunID: 5, Attempt: 2})

		want := FailingTest{
			Step: tc.gate, JobID: 42, Attempt: 2, Jobs: []string{tc.job},
			Detail: `step "` + tc.step + `" failed; the message was not read; next: gh run view --job 42 --attempt 2 --log-failed | tail -n ` + tc.bound + ` (the end of the job log)`,
		}
		if len(got.FailingTests) != 1 || !equalRow(got.FailingTests[0], want) {
			t.Errorf("%s: rows %+v, want [%+v]", tc.gate, got.FailingTests, want)
		}
	}
}

func TestClassifyReport_JSONOmitsTheGateFieldsOfAPlainRowAndOfAReportWithNoWarning(t *testing.T) {
	rep := ClassifyReport{Failures: []ClassifiedFailure{{FailingTest: FailingTest{Package: "m/a", Test: "TestA", Jobs: []string{"j"}}}}}

	raw, err := json.Marshal(rep)

	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"detail"`, `"job_id"`, `"attempt"`, `"warnings"`} {
		if strings.Contains(string(raw), key) {
			t.Errorf("json %s holds %s, want it omitted", raw, key)
		}
	}
}

func TestClassifyReport_JSONKeepsTheGateFieldsOfAGateRowAndOfAWarning(t *testing.T) {
	row := FailingTest{Step: "make cover-changed", Detail: "d", JobID: 7, Attempt: 1}
	rep := ClassifyReport{Failures: []ClassifiedFailure{{FailingTest: row}}, Warnings: []string{"w"}}

	raw, err := json.Marshal(rep)

	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"detail":"d"`, `"job_id":7`, `"attempt":1`, `"warnings":["w"]`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("json %s lacks %s", raw, want)
		}
	}
}

func TestClassifyTarget_ARunOfTheHeadCommitOnMainGivesNoWarning(t *testing.T) {
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	row := FailingTest{Package: "m/a", Test: "TestA", Jobs: []string{"j"}}
	head := failed(9, "head", at, row)
	sameHead := failed(7, "head", at.Add(-time.Hour), row)
	sameHead.Warnings = []string{"same head"}
	older := failed(6, "older", at.Add(-2*time.Hour), row)
	older.Warnings = []string{"older"}
	src := &fakeSource{change: Change{HeadSHA: "head", Run: head}, main: []RunStatus{sameHead, older}}

	rep, err := ClassifyTarget(context.Background(), src, Target{Kind: TargetRun, Value: "9"}, ClassifyOptions{MainWindow: 5})

	if err != nil || !slices.Equal(rep.Warnings, []string{"older"}) {
		t.Errorf("err %v warnings %q, want only the warning of the run that gave a fact", err, rep.Warnings)
	}
}
