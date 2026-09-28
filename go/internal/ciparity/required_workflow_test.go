package ciparity

import (
	"slices"
	"strings"
	"testing"
)

func TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies(t *testing.T) {
	w := readWorkflowContract(t, "required.yml")
	for _, event := range []string{"push", "pull_request", "workflow_dispatch"} {
		on, ok := w.On[event]
		if !ok || len(on.Paths) != 0 || len(on.PathsIgnore) != 0 {
			t.Errorf("%s must exist without path filters", event)
		}
		if event == "pull_request" && (len(on.Branches) != 0 || len(on.BranchesIgnore) != 0) {
			t.Error("all pull request target branches require a result")
		}
	}
	if !slices.Contains(w.On["push"].Branches, "main") || !slices.Contains(w.On["push"].Branches, "go-rewrite-phase-1") {
		t.Error("push routing dropped an existing validation branch")
	}
	if len(w.Permissions) != 1 || w.Permissions["contents"] != "read" {
		t.Fatal("required validation must have only read-only contents permission")
	}
	for id, job := range w.Jobs {
		if job.ContinueOnError != "" {
			t.Errorf("%s may not continue after error", id)
		}
	}
	required := w.Jobs["required"]
	if required.Name != "CI required" || required.If != "${{ always() }}" {
		t.Error("the stable CI required job must always report")
	}
	var needs []string
	if err := required.Needs.Decode(&needs); err != nil {
		t.Fatal(err)
	}
	slices.Sort(needs)
	if !slices.Equal(needs, []string{"changes", "go", "landing", "validate"}) {
		t.Fatalf("required result dependencies = %v", needs)
	}
	for id, source := range map[string]string{"go": "go.yml", "validate": "ci.yml", "landing": "landing-validation.yml"} {
		job := w.Jobs[id]
		if job.Uses != "./.github/workflows/"+source || job.ContinueOnError != "" {
			t.Errorf("%s must require the real reusable %s without allowing failures", id, source)
		}
		if id != "validate" && (job.If != "needs.changes.outputs."+id+" == 'true'" || job.Needs.Value != "changes") {
			t.Errorf("%s must use the successful changes job's explicit route", id)
		}
		if id == "validate" && job.If != "" {
			t.Error("plugin/ACS validation cannot be skipped for documentation changes")
		}
	}
}

func TestRequiredWorkflow_ReusableSuitesAreNotDuplicated(t *testing.T) {
	for _, name := range []string{"go.yml", "ci.yml"} {
		w := readWorkflowContract(t, name)
		if _, ok := w.On["workflow_call"]; !ok {
			t.Errorf("%s cannot be reused", name)
		}
		for _, event := range []string{"push", "pull_request"} {
			if _, ok := w.On[event]; ok {
				t.Errorf("%s duplicates the orchestrator's %s suite", name, event)
			}
		}
	}
}

func workflowStep(t *testing.T, workflow, job, id string) workflowStepContract {
	t.Helper()
	for _, step := range readWorkflowContract(t, workflow).Jobs[job].Steps {
		if step.ID == id && strings.TrimSpace(step.Run) != "" {
			return step
		}
	}
	t.Fatalf("%s has no executable %s/%s step", workflow, job, id)
	return workflowStepContract{}
}

func TestRequiredResult_ConsumesEachReusableJobResult(t *testing.T) {
	step := workflowStep(t, "required.yml", "required", "result")
	if step.If != "" || step.ContinueOnError != "" {
		t.Fatal("the result step may not be skipped or continue after failure")
	}
	for _, binding := range []struct{ workflow, output, child, env, caller string }{
		{"ci.yml", "plugin_result", "validate", "PLUGIN_RESULT", "validate"},
		{"ci.yml", "acs_result", "acs-durable", "ACS_RESULT", "validate"},
		{"go.yml", "suite_result", "build-test", "GO_SUITE_RESULT", "go"},
		{"landing-validation.yml", "suite_result", "test", "LANDING_SUITE_RESULT", "landing"},
	} {
		w := readWorkflowContract(t, binding.workflow)
		if w.On["workflow_call"].Outputs[binding.output].Value != "${{ jobs."+binding.child+".outputs.result }}" {
			t.Errorf("%s output %s is not bound to the real %s result", binding.workflow, binding.output, binding.child)
		}
		if w.Jobs[binding.child].Outputs["result"] != "${{ job.status }}" {
			t.Errorf("%s/%s must expose its actual job status", binding.workflow, binding.child)
		}
		if step.Env[binding.env] != "${{ needs."+binding.caller+".outputs."+binding.output+" }}" {
			t.Errorf("%s does not consume the reusable job result", binding.env)
		}
		for _, childStep := range w.Jobs[binding.child].Steps {
			if childStep.ContinueOnError != "" || (childStep.Run != "" && childStep.If != "") {
				t.Errorf("%s/%s may not conditionally skip validation or ignore its failure", binding.workflow, binding.child)
			}
		}
		if job, ok := w.Jobs[binding.child]; !ok || job.If != "" || job.ContinueOnError != "" {
			t.Errorf("%s must execute %s without skipping or ignoring errors", binding.workflow, binding.child)
		}
	}
}

func TestLandingValidation_PagesUsesCompleteValidatedArtifact(t *testing.T) {
	validation := readWorkflowContract(t, "landing-validation.yml")
	if len(validation.Permissions) != 1 || validation.Permissions["contents"] != "read" {
		t.Error("shared landing validation requires only read-only contents")
	}
	var upload, download workflowStepContract
	for _, step := range validation.Jobs["test"].Steps {
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			upload = step
		}
	}
	pages := readWorkflowContract(t, "landing-pages.yml")
	for _, step := range pages.Jobs["build"].Steps {
		if strings.HasPrefix(step.Uses, "actions/download-artifact@") {
			download = step
		}
	}
	if upload.With["name"] != "landing-dist" || upload.With["path"] != "landing/dist" || upload.With["if-no-files-found"] != "error" || upload.With["include-hidden-files"] != "true" {
		t.Error("shared landing validation must publish the complete rendered tree and fail if it is absent")
	}
	if download.With["name"] != upload.With["name"] || download.With["path"] != "landing/dist" {
		t.Error("Pages must consume this run's validated landing tree")
	}
	if !slices.Contains(pages.On["push"].Paths, ".github/workflows/landing-validation.yml") {
		t.Error("changes to the shared site builder must trigger Pages")
	}
}
