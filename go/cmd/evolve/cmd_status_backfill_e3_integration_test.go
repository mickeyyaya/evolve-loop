//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

func TestRunStatus_ReadsOpenPRsAndTheFailingMainRunFromTheRealGHBinary(t *testing.T) {
	root := e3StatusRoot(t)
	record := bfStub(t, "gh", `case "$1 $2" in
"pr list") echo '[{"number":7,"title":"feat: x","headRefName":"feat/x","url":"u7","statusCheckRollup":[{"name":"unit","status":"COMPLETED","conclusion":"FAILURE"}]}]' ;;
"run list") echo '[{"status":"completed","conclusion":"failure","url":"https://ci/runs/5","databaseId":5}]' ;;
"run view") printf 'test (ubuntu)\tstep\t--- FAIL: TestX (0.1s)\nlint\tstep\tboom\n' ;;
*) exit 9 ;;
esac`)
	var text, js, errb bytes.Buffer

	textCode := runStatus([]string{"--project-root", root}, nil, &text, &errb)
	jsonCode := runStatus([]string{"--json", "--project-root", root}, nil, &js, &errb)

	if textCode != 0 || jsonCode != 0 {
		t.Fatalf("codes %d/%d stderr %q", textCode, jsonCode, errb.String())
	}
	for _, want := range []string{
		"prs:     1 open\n  #7 feat: x [feat/x] checks failing: unit\n",
		"ci:      " + ciparity.RequiredWorkflow + " on main: completed failure https://ci/runs/5\n         failing jobs: test (ubuntu), lint\n",
	} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text lacks %q:\n%s", want, text.String())
		}
	}
	var report struct {
		CI struct {
			Available bool          `json:"available"`
			Items     []statusCIRun `json:"items"`
		} `json:"ci"`
	}
	if err := json.Unmarshal(js.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.CI.Available || len(report.CI.Items) != 1 || report.CI.Items[0].Branch != "main" || report.CI.Items[0].Workflow != ciparity.RequiredWorkflow || report.CI.Items[0].Conclusion != "failure" {
		t.Errorf("ci = %+v", report.CI)
	}
	if !strings.Contains(bfCalls(t, record), "run list --workflow "+ciparity.RequiredWorkflow+" --branch main --limit 1") {
		t.Errorf("gh calls:\n%s", bfCalls(t, record))
	}
}
