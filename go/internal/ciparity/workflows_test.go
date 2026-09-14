package ciparity

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type workflowContract struct {
	On   map[string]struct{ Paths []string }
	Jobs map[string]struct {
		Uses  string
		If    string
		Needs yaml.Node
		Steps []struct {
			Run              string
			Uses             string
			If               string
			With             map[string]string
			WorkingDirectory string `yaml:"working-directory"`
		}
	}
}

func TestRelease_RequiresSharedValidationBeforePublishing(t *testing.T) {
	release := readWorkflowContract(t, "release.yml")
	for job, source := range map[string]string{"test": "go.yml", "validate": "ci.yml"} {
		if got := release.Jobs[job].Uses; got != "./.github/workflows/"+source {
			t.Errorf("release %s uses %q; want shared %s validation", job, got, source)
		}
		if _, ok := readWorkflowContract(t, source).On["workflow_call"]; !ok {
			t.Errorf("%s cannot be called on the tagged release revision", source)
		}
	}
	var needs []string
	if n := release.Jobs["goreleaser"].Needs; n.Kind == yaml.ScalarNode {
		needs = []string{n.Value}
	} else if err := n.Decode(&needs); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"test", "validate"} {
		if !slices.Contains(needs, want) {
			t.Errorf("publication does not require %s: %v", want, needs)
		}
	}
	if release.Jobs["goreleaser"].If != "" {
		t.Fatal("publication must retain the default successful-dependencies condition")
	}
}

func TestGoWorkflow_TriggersForConsumedConfiguration(t *testing.T) {
	w := readWorkflowContract(t, "go.yml")
	for _, event := range []string{"push", "pull_request"} {
		if _, ok := w.On[event]; !ok {
			t.Errorf("workflow has no %s trigger", event)
			continue
		}
		for _, input := range []string{".goreleaser.yml", ".github/workflows/release.yml", ".github/workflows/ci.yml", ".github/workflows/landing-pages.yml"} {
			selected := len(w.On[event].Paths) == 0
			for _, pattern := range w.On[event].Paths {
				if matched, _ := filepath.Match(pattern, input); matched || (strings.HasSuffix(pattern, "/**") && strings.HasPrefix(input, strings.TrimSuffix(pattern, "**"))) {
					selected = true
				}
			}
			if !selected {
				t.Errorf("%s does not select consumed input %s", event, input)
			}
		}
	}
}

func TestGoWorkflow_UsesSharedIntegrationAndAPIGates(t *testing.T) {
	w := readWorkflowContract(t, "go.yml")
	for _, required := range []string{"make test-integration", "make apicover-check", "make cover-strict", "make test-e2e"} {
		found := false
		for _, step := range w.Jobs["build-test"].Steps {
			for _, line := range strings.Split(step.Run, "\n") {
				if strings.TrimSpace(line) == required {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("workflow does not execute shared gate %q", required)
		}
	}
}

func TestReusableGoValidation_PreservesReleaseHistory(t *testing.T) {
	for _, step := range readWorkflowContract(t, "go.yml").Jobs["build-test"].Steps {
		if strings.HasPrefix(step.Uses, "actions/checkout@") {
			if step.With["fetch-depth"] != "0" {
				t.Fatal("shared validation must preserve release's full-history checkout")
			}
			return
		}
	}
	t.Fatal("validation has no checkout")
}

func TestLandingWorkflow_TestsPullRequestsBeforeBuild(t *testing.T) {
	w := readWorkflowContract(t, "landing-pages.yml")
	if _, ok := w.On["pull_request"]; !ok {
		t.Error("landing tests are not selected for pull requests")
	}
	var needs []string
	if n := w.Jobs["build"].Needs; n.Kind == yaml.ScalarNode {
		needs = []string{n.Value}
	} else if err := n.Decode(&needs); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(needs, "test") {
		t.Error("site build does not depend on successful module tests")
	}
	if _, ok := w.Jobs["test"]; !ok {
		t.Error("landing has no module test job")
	}
	selected := false
	for _, step := range w.Jobs["test"].Steps {
		if step.WorkingDirectory == "landing" && strings.Contains(step.Run, "go test -race -count=1 ./...") {
			selected = true
		}
	}
	if !selected {
		t.Error("landing test job does not select the complete module with race detection")
	}
	const guard = "github.event_name != 'pull_request'"
	if w.Jobs["deploy"].If != guard {
		t.Errorf("deploy guard = %q; want %q", w.Jobs["deploy"].If, guard)
	}
	for _, step := range w.Jobs["build"].Steps {
		if strings.HasPrefix(step.Uses, "actions/configure-pages@") || strings.HasPrefix(step.Uses, "actions/upload-pages-artifact@") {
			if step.If != guard {
				t.Errorf("%s guard = %q; want %q", step.Uses, step.If, guard)
			}
		}
	}
}

func readWorkflowContract(t *testing.T, name string) workflowContract {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(workflowRepoRoot(t), ".github/workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	var w workflowContract
	if err := yaml.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	return w
}

func workflowRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate workflow test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
}
