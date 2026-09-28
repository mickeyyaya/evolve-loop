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
	On map[string]struct {
		Paths          []string
		PathsIgnore    []string `yaml:"paths-ignore"`
		Branches       []string
		BranchesIgnore []string `yaml:"branches-ignore"`
		Outputs        map[string]struct{ Value string }
	}
	Permissions map[string]string
	Jobs        map[string]workflowJobContract
}

type workflowJobContract struct {
	Name            string
	Uses            string
	If              string
	Needs           yaml.Node
	Outputs         map[string]string
	ContinueOnError string `yaml:"continue-on-error"`
	Steps           []workflowStepContract
}

type workflowStepContract struct {
	ContinueOnError  string `yaml:"continue-on-error"`
	ID               string
	Run              string
	Uses             string
	If               string
	Env              map[string]string
	With             map[string]string
	WorkingDirectory string `yaml:"working-directory"`
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
	var needs []string
	if n := w.Jobs["build"].Needs; n.Kind == yaml.ScalarNode {
		needs = []string{n.Value}
	} else if err := n.Decode(&needs); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(needs, "test") {
		t.Error("site build does not depend on successful module tests")
	}
	const shared = "./.github/workflows/landing-validation.yml"
	if w.Jobs["test"].Uses != shared || readWorkflowContract(t, "required.yml").Jobs["landing"].Uses != shared {
		t.Error("Pages and required PR checks must share landing validation")
	}
	validation := readWorkflowContract(t, "landing-validation.yml")
	if _, ok := validation.On["workflow_call"]; !ok {
		t.Error("landing validation is not reusable")
	}
	var commands []string
	for _, step := range validation.Jobs["test"].Steps {
		if step.WorkingDirectory == "landing" {
			commands = append(commands, strings.Fields(step.Run)...)
		}
	}
	for _, want := range []string{"go test -race -count=1 ./...", "go vet ./...", "go run ./cmd/build"} {
		if !strings.Contains(strings.Join(commands, " "), want) {
			t.Errorf("shared landing validation does not run %q", want)
		}
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
