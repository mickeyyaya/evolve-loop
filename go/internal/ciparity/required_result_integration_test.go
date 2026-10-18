//go:build integration

package ciparity

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRequiredResult_RejectsMissingOrUnsuccessfulWork(t *testing.T) {
	t.Parallel()
	step := workflowStep(t, RequiredWorkflow, "required", "result")
	bindings := map[string]string{
		"ROUTE_RESULT": "${{ needs.changes.result }}", "VALIDATE_RESULT": "${{ needs.validate.result }}",
		"GO_REQUIRED": "${{ needs.changes.outputs.go }}", "GO_RESULT": "${{ needs.go.result }}",
		"LANDING_REQUIRED": "${{ needs.changes.outputs.landing }}", "LANDING_RESULT": "${{ needs.landing.result }}",
		"INBOX_REQUIRED": "${{ needs.changes.outputs.inbox }}", "INBOX_RESULT": "${{ needs.inbox.result }}",
	}
	for key, want := range bindings {
		if step.Env[key] != want {
			t.Fatalf("result %s binding = %q; want actual dependency %q", key, step.Env[key], want)
		}
	}
	type resultCase struct {
		name string
		set  map[string]string
		pass bool
	}
	cases := []resultCase{
		{"all_required_green", nil, true},
		{"no_docs_change", map[string]string{"GO_REQUIRED": "false", "GO_RESULT": "skipped", "GO_SUITE_RESULT": "", "LANDING_REQUIRED": "false", "LANDING_RESULT": "skipped", "LANDING_SUITE_RESULT": ""}, true},
		{"docs_only", docsOnlyResult(nil), true},
		{"docs_only_lint_failed", docsOnlyResult(map[string]string{"STE_LINT_RESULT": "failure"}), false},
		{"docs_only_lint_missing", docsOnlyResult(map[string]string{"STE_LINT_RESULT": ""}), false},
		{"docs_only_lint_skipped", docsOnlyResult(map[string]string{"STE_LINT_RESULT": "skipped"}), false},
		{"docs_only_lint_cancelled", docsOnlyResult(map[string]string{"STE_LINT_RESULT": "cancelled"}), false},
		{"docs_only_validate_ran", docsOnlyResult(map[string]string{"VALIDATE_RESULT": "success", "PLUGIN_RESULT": "success", "ACS_RESULT": "success"}), false},
		{"docs_only_validate_failed", docsOnlyResult(map[string]string{"VALIDATE_RESULT": "failure"}), false},
		{"docs_only_without_the_docs_route", docsOnlyResult(map[string]string{"DOCS_REQUIRED": "false", "STE_LINT_RESULT": "skipped"}), false},
		{"docs_only_unknown_flag", docsOnlyResult(map[string]string{"DOCS_ONLY": "TRUE"}), false},
		{"mixed_docs_and_code_green", map[string]string{"DOCS_REQUIRED": "true", "STE_LINT_RESULT": "success"}, true},
		{"mixed_docs_and_code_lint_failed", map[string]string{"DOCS_REQUIRED": "true", "STE_LINT_RESULT": "failure"}, false},
		{"mixed_docs_and_code_lint_missing", map[string]string{"DOCS_REQUIRED": "true", "STE_LINT_RESULT": ""}, false},
		{"mixed_docs_and_code_validate_skipped", map[string]string{"DOCS_REQUIRED": "true", "STE_LINT_RESULT": "success", "VALIDATE_RESULT": "skipped", "PLUGIN_RESULT": "", "ACS_RESULT": ""}, false},
		{"lint_ran_without_the_docs_route", map[string]string{"STE_LINT_RESULT": "success"}, false},
		{"go_only", map[string]string{"LANDING_REQUIRED": "false", "LANDING_RESULT": "skipped", "LANDING_SUITE_RESULT": ""}, true},
		{"landing_only", map[string]string{"GO_REQUIRED": "false", "GO_RESULT": "skipped", "GO_SUITE_RESULT": ""}, true},
		{"inbox_only_green", inboxOnlyResult(nil), true},
		{"inbox_beside_go_green", map[string]string{"INBOX_REQUIRED": "true", "INBOX_RESULT": "success"}, true},
		{"inbox_only_lint_failed", inboxOnlyResult(map[string]string{"INBOX_RESULT": "failure"}), false},
		{"inbox_only_lint_cancelled", inboxOnlyResult(map[string]string{"INBOX_RESULT": "cancelled"}), false},
		{"inbox_only_lint_skipped", inboxOnlyResult(map[string]string{"INBOX_RESULT": "skipped"}), false},
		{"inbox_only_lint_missing", inboxOnlyResult(map[string]string{"INBOX_RESULT": ""}), false},
		{"inbox_unselected_but_failed", map[string]string{"INBOX_RESULT": "failure"}, false},
		{"inbox_unselected_but_cancelled", map[string]string{"INBOX_RESULT": "cancelled"}, false},
		{"inbox_route_unknown", map[string]string{"INBOX_REQUIRED": "TRUE", "INBOX_RESULT": "success"}, false},
		{"route_failed_docs_outputs", map[string]string{"ROUTE_RESULT": "failure", "GO_REQUIRED": "false", "GO_RESULT": "skipped", "GO_SUITE_RESULT": "", "LANDING_REQUIRED": "false", "LANDING_RESULT": "skipped", "LANDING_SUITE_RESULT": ""}, false},
	}
	for _, dependency := range []string{"ROUTE_RESULT", "VALIDATE_RESULT", "GO_RESULT", "LANDING_RESULT", "PLUGIN_RESULT", "ACS_RESULT", "GO_SUITE_RESULT", "LANDING_SUITE_RESULT", "STE_LINT_RESULT", "DOCS_ONLY", "DOCS_REQUIRED"} {
		for _, result := range []string{"failure", "cancelled", "skipped", "", "neutral", "pending"} {
			if dependency == "STE_LINT_RESULT" && result == "skipped" {
				continue
			}
			cases = append(cases, resultCase{dependency + "_" + result, map[string]string{dependency: result}, false})
		}
	}
	for _, suite := range []string{"GO", "LANDING"} {
		for _, route := range []string{"", "TRUE", "false", "garbage"} {
			cases = append(cases, resultCase{suite + "_route_" + route, map[string]string{suite + "_REQUIRED": route}, false})
		}
		for _, result := range []string{"failure", "cancelled", ""} {
			cases = append(cases, resultCase{suite + "_unselected_" + result, map[string]string{suite + "_REQUIRED": "false", suite + "_RESULT": result}, false})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"INBOX_REQUIRED": "false", "INBOX_RESULT": "skipped", "DOCS_ONLY": "false", "DOCS_REQUIRED": "false", "STE_LINT_RESULT": "skipped", "ROUTE_RESULT": "success", "VALIDATE_RESULT": "success", "GO_REQUIRED": "true", "GO_RESULT": "success", "LANDING_REQUIRED": "true", "LANDING_RESULT": "success", "PLUGIN_RESULT": "success", "ACS_RESULT": "success", "GO_SUITE_RESULT": "success", "LANDING_SUITE_RESULT": "success"}
			for key, value := range tc.set {
				env[key] = value
			}
			out, err := runWorkflowShell(t, t.TempDir(), step.Run, env)
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%v want %v; %v\n%s", err == nil, tc.pass, err, out)
			}
		})
	}
}

func docsOnlyResult(set map[string]string) map[string]string {
	env := map[string]string{
		"DOCS_ONLY": "true", "DOCS_REQUIRED": "true", "STE_LINT_RESULT": "success",
		"VALIDATE_RESULT": "skipped", "PLUGIN_RESULT": "", "ACS_RESULT": "",
		"GO_REQUIRED": "false", "GO_RESULT": "skipped", "GO_SUITE_RESULT": "",
		"LANDING_REQUIRED": "false", "LANDING_RESULT": "skipped", "LANDING_SUITE_RESULT": "",
	}
	for key, value := range set {
		env[key] = value
	}
	return env
}

func inboxOnlyResult(set map[string]string) map[string]string {
	env := map[string]string{
		"INBOX_REQUIRED": "true", "INBOX_RESULT": "success",
		"GO_REQUIRED": "false", "GO_RESULT": "skipped", "GO_SUITE_RESULT": "",
		"LANDING_REQUIRED": "false", "LANDING_RESULT": "skipped", "LANDING_SUITE_RESULT": "",
	}
	for key, value := range set {
		env[key] = value
	}
	return env
}

func runWorkflowShell(t *testing.T, dir, script string, env map[string]string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script)
	cmd.Dir = dir
	cmd.Env = workflowTestEnv()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("workflow harness deadline: %v\n%s", ctx.Err(), out)
	}
	return string(out), err
}

func workflowTestEnv() []string {
	var env []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GIT_") {
			env = append(env, value)
		}
	}
	return env
}
