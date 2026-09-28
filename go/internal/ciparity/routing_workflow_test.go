package ciparity

import (
	"strings"
	"testing"
)

func TestRequiredRouting_UsesHostEventAndPublishesBothDecisions(t *testing.T) {
	w := readWorkflowContract(t, RequiredWorkflow)
	for _, suite := range []string{"go", "landing"} {
		if w.Jobs["changes"].Outputs[suite] != "${{ steps.paths.outputs."+suite+" }}" {
			t.Errorf("%s routing is not bound to computed paths", suite)
		}
	}
	step := workflowStep(t, RequiredWorkflow, "changes", "paths")
	for name, want := range map[string]string{
		"EVENT_NAME": "${{ github.event_name }}",
		"BASE_SHA":   "${{ github.event.pull_request.base.sha || github.event.before }}",
		"HEAD_SHA":   "${{ github.event.pull_request.head.sha || github.sha }}",
	} {
		if step.Env[name] != want {
			t.Errorf("%s = %q, want host event %q", name, step.Env[name], want)
		}
	}
	for _, s := range w.Jobs["changes"].Steps {
		if strings.HasPrefix(s.Uses, "actions/checkout@") && s.With["fetch-depth"] == "0" {
			return
		}
	}
	t.Fatal("routing requires full history for PR merge-base and push-before objects")
}
