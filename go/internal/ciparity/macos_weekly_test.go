package ciparity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const macosWeeklyWorkflow = "macos-weekly.yml"

type macosWeeklyContract struct {
	On struct {
		Schedule         []struct{ Cron string }
		WorkflowDispatch yaml.Node `yaml:"workflow_dispatch"`
		PullRequest      yaml.Node `yaml:"pull_request"`
		Push             yaml.Node `yaml:"push"`
		WorkflowCall     yaml.Node `yaml:"workflow_call"`
	}
	Permissions map[string]string
	Concurrency struct {
		Group            string
		CancelInProgress string `yaml:"cancel-in-progress"`
	}
	Jobs map[string]workflowJobContract
}

func readMacosWeekly(t *testing.T) (macosWeeklyContract, string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(workflowRepoRoot(t), ".github/workflows", macosWeeklyWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	var w macosWeeklyContract
	if err := yaml.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	return w, string(b)
}

func TestMacosWeekly_RunsWeeklyOnMainAndIsNotAPullRequestCheck(t *testing.T) {
	t.Parallel()
	w, _ := readMacosWeekly(t)
	if len(w.On.Schedule) != 1 {
		t.Fatalf("want one schedule, got %d", len(w.On.Schedule))
	}
	fields := strings.Fields(w.On.Schedule[0].Cron)
	if len(fields) != 5 || fields[2] != "*" || fields[3] != "*" || strings.ContainsAny(fields[4], "*,-/") {
		t.Errorf("cron %q must fire on one fixed weekday", w.On.Schedule[0].Cron)
	}
	if w.On.WorkflowDispatch.IsZero() {
		t.Error("workflow_dispatch must exist for a manual run")
	}
	if !w.On.PullRequest.IsZero() || !w.On.Push.IsZero() {
		t.Error("a weekly backstop must not run on push or pull_request")
	}
	job := w.Jobs["full-suite"]
	var ref string
	for _, s := range job.Steps {
		if strings.HasPrefix(s.Uses, "actions/checkout@") {
			ref = s.With["ref"]
		}
	}
	if ref != "main" {
		t.Errorf("the run must check out main whatever ref started it, got %q", ref)
	}
}

func TestMacosWeekly_RunsTheFullSuiteThroughTheMakeTargets(t *testing.T) {
	t.Parallel()
	w, _ := readMacosWeekly(t)
	job := w.Jobs["full-suite"]
	if job.RunsOn != "macos-latest" {
		t.Errorf("runs-on = %q; want macos-latest", job.RunsOn)
	}
	var runs []string
	var goVersion string
	for _, s := range job.Steps {
		runs = append(runs, s.Run)
		if strings.HasPrefix(s.Uses, "actions/setup-go@") {
			goVersion = s.With["go-version"]
		}
	}
	all := strings.Join(runs, "\n")
	for _, want := range []string{"make test-integration", "make test-e2e"} {
		if !strings.Contains(all, want) {
			t.Errorf("the run must call %q", want)
		}
	}
	for _, bad := range []string{"go test", "test-integration-darwin"} {
		if strings.Contains(all, bad) {
			t.Errorf("the run must use the full-suite make targets, found %q", bad)
		}
	}
	if goVersion != "1.23" {
		t.Errorf("go-version = %q; want 1.23", goVersion)
	}
	var upload bool
	for _, st := range job.Steps {
		switch {
		case strings.Contains(st.Run, "make test-integration"):
			if st.Env["TEST_JSON"] != "test-integration.json" {
				t.Errorf("integration step TEST_JSON = %q", st.Env["TEST_JSON"])
			}
		case strings.Contains(st.Run, "make test-e2e"):
			if st.Env["E2E_JSON"] != "test-e2e.json" {
				t.Errorf("e2e step E2E_JSON = %q", st.Env["E2E_JSON"])
			}
		case strings.HasPrefix(st.Uses, "actions/upload-artifact@"):
			upload = true
		}
	}
	if !upload {
		t.Error("the json streams must be uploaded")
	}
}

func TestMacosWeekly_IsASoftGateOutsideTheRequiredResult(t *testing.T) {
	t.Parallel()
	w, _ := readMacosWeekly(t)
	required := readWorkflowContract(t, RequiredWorkflow)
	_, requiredText := readRequiredText(t)
	if strings.Contains(requiredText, macosWeeklyWorkflow) {
		t.Error("required.yml must not call the weekly run")
	}
	var needs []string
	requiredJob := required.Jobs["required"]
	if err := requiredJob.Needs.Decode(&needs); err != nil {
		t.Fatal(err)
	}
	for _, n := range needs {
		if strings.Contains(n, "macos") {
			t.Errorf("CI required depends on %q", n)
		}
	}
	if !w.On.WorkflowCall.IsZero() {
		t.Error("no other workflow may call the weekly run")
	}
	c := w.Concurrency
	if c.Group != "macos-weekly" || c.CancelInProgress != "false" {
		t.Errorf("own concurrency group, never cancelled: %+v", c)
	}
}

func TestMacosWeekly_OnlyTheReportJobCanWriteIssues(t *testing.T) {
	t.Parallel()
	w, _ := readMacosWeekly(t)
	if len(w.Permissions) != 1 || w.Permissions["contents"] != "read" {
		t.Errorf("workflow permissions = %v; want only contents: read", w.Permissions)
	}
	if got := w.Jobs["full-suite"].Permissions; len(got) != 0 {
		t.Errorf("full-suite must inherit contents: read, got %v", got)
	}
	report := w.Jobs["report"]
	if len(report.Permissions) != 1 || report.Permissions["issues"] != "write" {
		t.Errorf("report permissions = %v; want only issues: write", report.Permissions)
	}
	if report.Needs.Value != "full-suite" {
		t.Errorf("report must need full-suite, got %q", report.Needs.Value)
	}
	if !strings.Contains(report.If, "failure()") || !strings.Contains(report.If, "needs.full-suite.result == 'failure'") {
		t.Errorf("report must run only when the suite failed: %q", report.If)
	}
	if report.Env["ISSUE_LABEL"] != "ci-macos-weekly" || report.Env["ISSUE_TITLE"] == "" {
		t.Errorf("report env = %v; want the ci-macos-weekly label and a title", report.Env)
	}
	var run string
	for _, s := range report.Steps {
		run += s.Run
	}
	for _, want := range []string{"gh issue list", "gh issue create", "gh issue comment", "--label \"$ISSUE_LABEL\"", "select(.title =="} {
		if !strings.Contains(run, want) {
			t.Errorf("the issue step must contain %q (deduplicate by title, label, comment on repeat)", want)
		}
	}
}

func readRequiredText(t *testing.T) (string, string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(workflowRepoRoot(t), ".github/workflows", RequiredWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	return RequiredWorkflow, string(b)
}
