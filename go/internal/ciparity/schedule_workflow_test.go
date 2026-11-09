package ciparity

import (
	"strings"
	"testing"
)

func TestRequiredWorkflow_RunsTheFullGraphOnceADayOnTheDefaultBranch(t *testing.T) {
	t.Parallel()
	on, ok := readWorkflowContract(t, RequiredWorkflow).On["schedule"]
	if !ok {
		t.Fatal("required.yml has no schedule trigger, so main gets a finished full run only when a push is not superseded")
	}
	if len(on.Crons) != 1 {
		t.Fatalf("want exactly one daily cron, got %v", on.Crons)
	}
	fields := strings.Fields(on.Crons[0])
	if len(fields) != 5 {
		t.Fatalf("cron %q must have five fields", on.Crons[0])
	}
	for i, name := range []string{"minute", "hour"} {
		if fields[i] == "*" || strings.ContainsAny(fields[i], "*,-/") {
			t.Errorf("cron %q: %s must be one fixed value, not %q", on.Crons[0], name, fields[i])
		}
	}
	for i, name := range []string{"day of month", "month", "day of week"} {
		if fields[i+2] != "*" {
			t.Errorf("cron %q: %s must be * for a daily run, got %q", on.Crons[0], name, fields[i+2])
		}
	}
	if len(on.Paths) != 0 || len(on.Branches) != 0 {
		t.Errorf("a schedule trigger takes no path or branch filter: %+v", on)
	}
}

func TestRequiredWorkflow_ScheduleLeavesTheResultAndRoutingGraphUnchanged(t *testing.T) {
	t.Parallel()
	w := readWorkflowContract(t, RequiredWorkflow)
	if got := w.Jobs["required"].If; got != "${{ always() }}" {
		t.Errorf("CI required must still always report on a scheduled run, got if %q", got)
	}
	for suite, output := range w.Jobs["changes"].Outputs {
		if want := "${{ steps.paths.outputs." + suite + " }}"; output != want {
			t.Errorf("%s output = %q, want %q", suite, output, want)
		}
	}
	step := workflowStep(t, RequiredWorkflow, "changes", "paths")
	if step.Env["EVENT_NAME"] != "${{ github.event_name }}" {
		t.Errorf("routing must read the host event, got %q", step.Env["EVENT_NAME"])
	}
}
