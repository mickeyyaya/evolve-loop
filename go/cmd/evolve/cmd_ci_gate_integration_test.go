//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
)

const (
	gateRunID        = 41826765563
	gateUbuntuJobID  = "126563650938"
	gateUbuntuJob    = "go / build + test (Go 1.23, ubuntu, every package and every gate)"
	gateCoverStep    = "changed functions at 100% (cover-changed; hard-fail)"
	gateNoFailTest   = "{\"Action\":\"pass\",\"Package\":\"github.com/x/y\"}\n"
	gateArtifactList = `{"total_count":1,"artifacts":[{"name":"coverage-ubuntu-latest-go1.23","expired":false}]}`
)

func gateFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "cigate", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func gateReplies(t *testing.T, run string) map[string]string {
	t.Helper()
	id, err := strconv.Atoi(run)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"run.json": ciRunDoc(id, "completed", "failure"), "compare.json": ciCompareDoc, "runs.json": "[]",
		"log.txt":   gateFixture(t, "run-"+run+"-subset.txt"),
		"jobs.json": gateFixture(t, "run-"+run+"-jobs.json"),
	}
}

func gateRows(out string) []string {
	var rows []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "real\t") || strings.HasPrefix(line, "unknown\t") || strings.HasPrefix(line, "pre-existing\t") || strings.HasPrefix(line, "flake-evidence\t") {
			rows = append(rows, line)
		}
	}
	return rows
}

func TestRunCIClassify_AFailedCoverChangedGateIsOneRowThatNamesTheStepAndTheBoundedCommand(t *testing.T) {
	newCIFakeGH(t, gateReplies(t, "41826765563"))
	root := ciProjectRoot(t, "", true)

	out, errOut, code := runCIIn("classify", "run:41826765563", "--project-root", root)

	want := "unknown\t-\tmake cover-changed: step \"" + gateCoverStep + "\" failed; the message was not read; next: gh run view --job " +
		gateUbuntuJobID + " --attempt 1 --log-failed | tail -n 200 (the end of the job log)\tdefault\tunknown\tunknown\tunknown\tunknown"
	rows := gateRows(out)
	found := 0
	for _, row := range rows {
		if row == want {
			found++
		}
		if strings.Contains(row, "make cover-changed") && row != want {
			t.Errorf("a second row names the gate:\n%q", row)
		}
	}
	if found != 1 || code != exitRefused || errOut != "" {
		t.Errorf("code=%d stderr=%q gate rows=%d, want 1 exact row; rows:\n%s", code, errOut, found, strings.Join(rows, "\n"))
	}
}

func gateClassify(t *testing.T, run string, mutate func(replies map[string]string), extra ...string) (string, string, int) {
	t.Helper()
	replies := gateReplies(t, run)
	if mutate != nil {
		mutate(replies)
	}
	gh := newCIFakeGH(t, replies)
	if replies["jobs.json"] == jobAPIFails {
		if err := os.Remove(filepath.Join(gh.dir, "jobs.json")); err != nil {
			t.Fatal(err)
		}
	}
	root := ciProjectRoot(t, "", true)
	return runCIIn(append([]string{"classify", "run:" + run, "--project-root", root}, extra...)...)
}

const jobAPIFails = "the job API route exits non-zero"

func failingJobAPI(replies map[string]string) {
	replies["jobs.json"] = jobAPIFails
}

func withoutJobAPI(replies map[string]string) {
	replies["jobs.json"] = `{"jobs":[]}`
}

func withTestJSONArtifact(replies map[string]string) {
	replies["artifacts.json"] = gateArtifactList
	replies["testjson.json"] = gateNoFailTest
}

func classifyJSON(t *testing.T, out string) ciwatch.ClassifyReport {
	t.Helper()
	var rep ciwatch.ClassifyReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
	return rep
}

func rowsOf(rep ciwatch.ClassifyReport, pred func(ciwatch.FailingTest) bool) []ciwatch.FailingTest {
	var rows []ciwatch.FailingTest
	for _, f := range rep.Failures {
		if pred(f.FailingTest) {
			rows = append(rows, f.FailingTest)
		}
	}
	return rows
}

func TestRunCIClassify_TheArtifactFormOfTheFaultGivesOneGateRowWithTodaysKey(t *testing.T) {
	before, _, _ := gateClassify(t, "41826765563", func(r map[string]string) { withTestJSONArtifact(r); withoutJobAPI(r) }, "--json")
	after, errOut, _ := gateClassify(t, "41826765563", withTestJSONArtifact, "--json")

	oldRows := rowsOf(classifyJSON(t, before), func(f ciwatch.FailingTest) bool { return f.Step == "make cover-changed" })
	newRows := rowsOf(classifyJSON(t, after), func(f ciwatch.FailingTest) bool { return f.Jobs[0] == gateUbuntuJob })
	if len(oldRows) != 1 || len(newRows) != 1 || newRows[0].Step != oldRows[0].Step || newRows[0].Package != "" || newRows[0].Test != "" || errOut != "" {
		t.Errorf("old gate rows %+v, new rows of the job %+v, stderr %q; want one row each with the same step", oldRows, newRows, errOut)
	}
	if newRows[0].JobID != 126563650938 || newRows[0].Attempt != 1 {
		t.Errorf("job id %d attempt %d, want 126563650938 and 1", newRows[0].JobID, newRows[0].Attempt)
	}
}

func TestRunCIClassify_ARunWithNoFailedGateStepKeepsTodaysOutputForEveryShape(t *testing.T) {
	cases := map[string]struct {
		run    string
		mutate func(map[string]string)
	}{
		"a cancelled step": {"41844945011", nil},
		"a cancelled job with no steps": {"41826765563", func(r map[string]string) {
			r["jobs.json"] = `{"jobs":[{"databaseId":126563650938,"name":"` + gateUbuntuJob + `","conclusion":"cancelled","steps":[]}]}`
		}},
		"test json and a passing gate step": {"41826765563", func(r map[string]string) {
			withTestJSONArtifact(r)
			r["jobs.json"] = strings.Replace(r["jobs.json"], `"conclusion":"failure","name":"`+gateCoverStep, `"conclusion":"success","name":"`+gateCoverStep, 1)
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			want, _, wantCode := gateClassify(t, tc.run, func(r map[string]string) {
				if tc.mutate != nil {
					tc.mutate(r)
				}
				withoutJobAPI(r)
			})
			got, errOut, code := gateClassify(t, tc.run, tc.mutate)
			if got != want || code != wantCode || errOut != "" {
				t.Errorf("code %d stderr %q, output differs from the no-job-API output:\n%s\nwant\n%s", code, errOut, got, want)
			}
			if strings.Contains(got, "failed; the message was not read") {
				t.Error("a gate row was added")
			}
		})
	}
}

func TestRunCIClassify_AFailedAcsStepKeepsItsTestRowsAndTheGoJobGetsItsGateRow(t *testing.T) {
	out, errOut, _ := gateClassify(t, "41793157151", nil, "--json")

	rep := classifyJSON(t, out)
	acs := rowsOf(rep, func(f ciwatch.FailingTest) bool {
		return f.Test != "" && slices.Contains(f.Jobs, "plugin and durable ACS / acs-durable")
	})
	gate := rowsOf(rep, func(f ciwatch.FailingTest) bool { return f.Step == "make apicover-check" })
	if len(acs) != 2 || len(gate) != 1 || !strings.HasSuffix(gate[0].Detail, "tail -n 2000 (the end of the job log)") || errOut != "" {
		t.Errorf("acs rows %+v, gate rows %+v, stderr %q; want 2 acs test rows and 1 apicover gate row with bound 2000", acs, gate, errOut)
	}
}

func TestRunCIClassify_AGateRowTakesOnlyItsOwnJobOutOfARowThatTwoJobsShare(t *testing.T) {
	const macStep = "changed functions of the darwin set at 100% (cover-changed; hard-fail)"
	renameMacStep := func(r map[string]string) {
		r["jobs.json"] = strings.Replace(r["jobs.json"], macStep, "a step outside the table", 1)
	}
	out, errOut, _ := gateClassify(t, "41473319266", renameMacStep, "--json")

	rep := classifyJSON(t, out)
	shared := rowsOf(rep, func(f ciwatch.FailingTest) bool { return f.Step == "" && slices.Contains(f.Jobs, jobDarwinName) })
	gate := rowsOf(rep, func(f ciwatch.FailingTest) bool { return f.Step == "make cover-changed" })
	ubuntuLeft := rowsOf(rep, func(f ciwatch.FailingTest) bool {
		return f.Step != "make cover-changed" && slices.Contains(f.Jobs, gateUbuntuJob)
	})
	if len(shared) == 0 || len(gate) != 1 || len(gate[0].Jobs) != 1 || len(ubuntuLeft) != 0 || errOut != "" {
		t.Errorf("rows kept by the darwin job %d, gate rows %+v, rows left on the ubuntu job %+v, stderr %q", len(shared), gate, ubuntuLeft, errOut)
	}
	for _, row := range shared {
		if len(row.Jobs) != 1 {
			t.Errorf("row %+v still names the replaced job", row)
		}
	}
}

const jobDarwinName = "go / build + test (Go 1.23, macOS, the darwin package set)"

func TestRunCIClassify_AFailedJobAPIReadKeepsTodaysRowsAndWarnsOnStderr(t *testing.T) {
	want, wantErr, wantCode := gateClassify(t, "41826765563", withoutJobAPI)
	got, errOut, code := gateClassify(t, "41826765563", failingJobAPI)

	if got != want || code != wantCode || wantErr != "" {
		t.Errorf("rows changed after a failed read, code %d vs %d:\n%s\nwant\n%s", code, wantCode, got, want)
	}
	if !strings.Contains(errOut, "evolve ci classify: WARN: run 41826765563: the job API was not read") {
		t.Errorf("stderr %q, want the warning", errOut)
	}
}

func TestRunCIWatch_AFailedJobAPIReadAddsTheWarningOnStderrAndTheGateRowOtherwise(t *testing.T) {
	run := func(mutate func(map[string]string)) (string, string) {
		replies := gateReplies(t, "41826765563")
		replies["commit.json"] = `{"sha":"` + ciFakeHead + `"}`
		replies["watch-required.yml.json"] = watchReply("failure", gateRunID)
		mutate(replies)
		gh := newCIFakeGH(t, replies)
		if replies["jobs.json"] == jobAPIFails {
			if err := os.Remove(filepath.Join(gh.dir, "jobs.json")); err != nil {
				t.Fatal(err)
			}
		}
		out, errOut, code := runCIIn("watch", "--sha", ciFakeHead, "--project-root", ciProjectRoot(t, "", true))
		if code != exitRefused {
			t.Fatalf("code=%d stderr=%q, want %d", code, errOut, exitRefused)
		}
		return out, errOut
	}

	failedOut, failedErr := run(failingJobAPI)
	okOut, okErr := run(func(map[string]string) {})

	if !strings.Contains(failedErr, "evolve ci watch: WARN: run 41826765563: the job API was not read") || strings.Contains(failedOut, "failed; the message was not read") {
		t.Errorf("failed read: stdout %q stderr %q, want the warning and no gate row", failedOut, failedErr)
	}
	if strings.Contains(okErr, "WARN") || !strings.Contains(okOut, "make cover-changed: step \""+gateCoverStep+"\" failed; the message was not read; next: gh run view --job "+gateUbuntuJobID+" --attempt 1 --log-failed | tail -n 200") {
		t.Errorf("read ok: stdout %q stderr %q, want the gate row and no warning", okOut, okErr)
	}
}

func TestRunCIClassify_EveryPrintedGateRowOfTheRecordedRunsIsPinnedWhole(t *testing.T) {
	const tail = "\tdefault\tunknown\tunknown\tunknown\tunknown"
	cases := []struct{ run, want string }{
		{"41793157151", "unknown\t-\tmake apicover-check: step \"api-coverage enforce (apicover; per-package hard-fail, Phase 5)\" failed; the message was not read; next: gh run view --job 126451175141 --attempt 1 --log-failed | tail -n 2000 (the end of the job log)" + tail},
		{"41473319266", "unknown\t-\tmake cover-changed: step \"changed functions at 100% (cover-changed; hard-fail)\" failed; the message was not read; next: gh run view --job 125372622610 --attempt 1 --log-failed | tail -n 200 (the end of the job log)" + tail},
		{"41473319266", "unknown\t-\tmake cover-changed-darwin: step \"changed functions of the darwin set at 100% (cover-changed; hard-fail)\" failed; the message was not read; next: gh run view --job 125372622666 --attempt 1 --log-failed | tail -n 200 (the end of the job log)" + tail},
	}
	for _, tc := range cases {
		out, _, _ := gateClassify(t, tc.run, nil)
		if !slices.Contains(gateRows(out), tc.want) {
			t.Errorf("run %s: no row equals\n%q\nrows:\n%s", tc.run, tc.want, strings.Join(gateRows(out), "\n"))
		}
	}
}
