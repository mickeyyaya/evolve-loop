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
	step := workflowStep(t, RequiredWorkflow, "required", "result")
	bindings := map[string]string{
		"ROUTE_RESULT": "${{ needs.changes.result }}", "VALIDATE_RESULT": "${{ needs.validate.result }}",
		"GO_REQUIRED": "${{ needs.changes.outputs.go }}", "GO_RESULT": "${{ needs.go.result }}",
		"LANDING_REQUIRED": "${{ needs.changes.outputs.landing }}", "LANDING_RESULT": "${{ needs.landing.result }}",
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
		{"docs_only", map[string]string{"GO_REQUIRED": "false", "GO_RESULT": "skipped", "GO_SUITE_RESULT": "", "LANDING_REQUIRED": "false", "LANDING_RESULT": "skipped", "LANDING_SUITE_RESULT": ""}, true},
		{"go_only", map[string]string{"LANDING_REQUIRED": "false", "LANDING_RESULT": "skipped", "LANDING_SUITE_RESULT": ""}, true},
		{"landing_only", map[string]string{"GO_REQUIRED": "false", "GO_RESULT": "skipped", "GO_SUITE_RESULT": ""}, true},
		{"route_failed_docs_outputs", map[string]string{"ROUTE_RESULT": "failure", "GO_REQUIRED": "false", "GO_RESULT": "skipped", "GO_SUITE_RESULT": "", "LANDING_REQUIRED": "false", "LANDING_RESULT": "skipped", "LANDING_SUITE_RESULT": ""}, false},
	}
	for _, dependency := range []string{"ROUTE_RESULT", "VALIDATE_RESULT", "GO_RESULT", "LANDING_RESULT", "PLUGIN_RESULT", "ACS_RESULT", "GO_SUITE_RESULT", "LANDING_SUITE_RESULT"} {
		for _, result := range []string{"failure", "cancelled", "skipped", "", "neutral", "pending"} {
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
			env := map[string]string{"ROUTE_RESULT": "success", "VALIDATE_RESULT": "success", "GO_REQUIRED": "true", "GO_RESULT": "success", "LANDING_REQUIRED": "true", "LANDING_RESULT": "success", "PLUGIN_RESULT": "success", "ACS_RESULT": "success", "GO_SUITE_RESULT": "success", "LANDING_SUITE_RESULT": "success"}
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
