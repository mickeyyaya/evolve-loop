package ciparity

import (
	"os"
	"path/filepath"
	"regexp"
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
	Concurrency struct {
		Group            string
		CancelInProgress string `yaml:"cancel-in-progress"`
	}
	Jobs map[string]workflowJobContract
}

type workflowJobContract struct {
	Name            string
	Uses            string
	If              string
	RunsOn          string `yaml:"runs-on"`
	Needs           yaml.Node
	Permissions     map[string]string
	Env             map[string]string
	Outputs         map[string]string
	ContinueOnError string `yaml:"continue-on-error"`
	TimeoutMinutes  int    `yaml:"timeout-minutes"`
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	if w.Jobs["test"].Uses != shared || readWorkflowContract(t, RequiredWorkflow).Jobs["landing"].Uses != shared {
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

func TestGoWorkflow_RunsCoverChangedRightAfterTheIntegrationProfile(t *testing.T) {
	t.Parallel()
	steps := readWorkflowContract(t, "go.yml").Jobs["build-test"].Steps
	integration, cover := -1, -1
	for i, step := range steps {
		if strings.Contains(step.Run, "make test-integration") {
			integration = i
		}
		if strings.Contains(step.Run, "make cover-changed") {
			cover = i
		}
	}
	if integration < 0 || cover != integration+1 {
		t.Fatalf("cover-changed step at %d, integration step at %d; want cover-changed right after the integration profile is written", cover, integration)
	}
	step := steps[cover]
	if step.If != "" || step.ContinueOnError != "" {
		t.Errorf("the cover-changed step must always run and fail the job: if=%q continue-on-error=%q", step.If, step.ContinueOnError)
	}
	if got := step.Env["COVER_BASE"]; got != "${{ github.event.pull_request.base.sha || github.event.before }}" {
		t.Errorf("COVER_BASE = %q, want the PR base or the pushed-over commit", got)
	}
	if !strings.Contains(step.Run, `COVER_CHANGED_BASE="$base"`) {
		t.Errorf("the step must pass its base to make:\n%s", step.Run)
	}
}

func TestGoWorkflow_ReportsPackageWallBudgetWithoutGatingTheJob(t *testing.T) {
	t.Parallel()
	steps := readWorkflowContract(t, "go.yml").Jobs["build-test"].Steps
	integration, budget := -1, -1
	for i, step := range steps {
		if strings.Contains(step.Run, "make test-integration") {
			integration = i
		}
		if strings.Contains(step.Run, "cmd/testlatency") {
			budget = i
		}
	}
	if integration < 0 || budget <= integration {
		t.Fatalf("budget step at %d, integration step at %d; want the report after the run it reads", budget, integration)
	}
	stream := steps[integration].Env["TEST_JSON"]
	if stream == "" {
		t.Fatal("the integration step must set TEST_JSON so the recipe writes the stream the report reads")
	}
	step := steps[budget]
	if step.ContinueOnError != "" || step.If != "" {
		t.Errorf("a validation job step may not carry a condition or continue-on-error: continue-on-error=%q if=%q", step.ContinueOnError, step.If)
	}
	for _, want := range []string{"-budget 180s", stream, `"$GITHUB_STEP_SUMMARY"`, `|| echo "::warning::`} {
		if !strings.Contains(step.Run, want) {
			t.Errorf("budget step lacks %q:\n%s", want, step.Run)
		}
	}
	if strings.Contains(step.Run, "fail-over-budget") {
		t.Error("the budget step is advisory; --fail-over-budget belongs to a later enforce step")
	}
}

func TestGoWorkflow_GatesTheSuiteOnAPinnedGovulncheckFromTheMakeTarget(t *testing.T) {
	t.Parallel()
	job, ok := readWorkflowContract(t, "go.yml").Jobs["vulncheck"]
	if !ok {
		t.Fatal("go.yml has no vulncheck job; a failed job fails the reusable workflow, so it gates the required result")
	}
	if job.If != "" || job.ContinueOnError != "" {
		t.Errorf("the vulncheck job must always run and fail the suite: if=%q continue-on-error=%q", job.If, job.ContinueOnError)
	}
	var runs, goVersion string
	for _, step := range job.Steps {
		if step.ContinueOnError != "" || step.If != "" {
			t.Errorf("vulncheck step %q may not skip or ignore its failure", step.Run+step.Uses)
		}
		runs += strings.TrimSpace(step.Run) + "\n"
		if strings.HasPrefix(step.Uses, "actions/setup-go@") {
			goVersion = step.With["go-version"]
		}
	}
	if !strings.Contains(runs, "make vulncheck\n") || goVersion != "1.27" {
		t.Errorf("vulncheck job runs %q on Go %q; want `make vulncheck` on the current 1.27 toolchain", runs, goVersion)
	}
	makefile, err := os.ReadFile(filepath.Join(workflowRepoRoot(t), "go", "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(makefile)
	if !regexp.MustCompile(`(?m)^GOVULNCHECK_VERSION := v\d+\.\d+\.\d+$`).MatchString(text) {
		t.Error("Makefile must pin GOVULNCHECK_VERSION to an exact release")
	}
	if !strings.Contains(text, "govulncheck@$(GOVULNCHECK_VERSION) -json $(PKG)") {
		t.Error("the vulncheck recipe must run the pinned govulncheck over every package")
	}
}

func TestGoWorkflow_VetsAndFormatsOncePerPlatformNotOncePerMatrixLeg(t *testing.T) {
	t.Parallel()
	jobs := readWorkflowContract(t, "go.yml").Jobs
	runsOf := func(name string) string {
		var runs string
		for _, step := range jobs[name].Steps {
			if name != "build-test" && (step.If != "" || step.ContinueOnError != "") {
				t.Errorf("%s step %q may not skip or ignore its failure", name, step.Run+step.Uses)
			}
			runs += step.Run + "\n"
		}
		return runs
	}
	goVersionOf := func(name string) string {
		for _, step := range jobs[name].Steps {
			if strings.HasPrefix(step.Uses, "actions/setup-go@") {
				return step.With["go-version"]
			}
		}
		return ""
	}
	if matrixRuns := runsOf("build-test"); strings.Contains(matrixRuns, "go vet") || strings.Contains(matrixRuns, "gofmt") {
		t.Errorf("the OS and Go matrix runs vet or gofmt once per leg:\n%s", matrixRuns)
	}
	lint := runsOf("lint")
	for _, want := range []string{"\ngo vet ./...\n", "GOOS=darwin go vet ./...", "gofmt -d -s ."} {
		if !strings.Contains("\n"+lint, want) {
			t.Errorf("lint job lacks %q:\n%s", want, lint)
		}
	}
	if goVersionOf("lint") != "1.23" || goVersionOf("vet-current") != "1.27" {
		t.Errorf("lint runs Go %q and vet-current runs Go %q; want 1.23 and 1.27", goVersionOf("lint"), goVersionOf("vet-current"))
	}
	if current := runsOf("vet-current"); !strings.Contains(current, "go vet ./...") || strings.Contains(current, "gofmt") {
		t.Errorf("vet-current must vet the current toolchain and leave gofmt to the 1.23 job:\n%s", current)
	}
	for _, name := range []string{"lint", "vet-current"} {
		if jobs[name].If != "" || jobs[name].ContinueOnError != "" {
			t.Errorf("%s must always run and fail the suite: if=%q continue-on-error=%q", name, jobs[name].If, jobs[name].ContinueOnError)
		}
	}
}

func runLines(job workflowJobContract) []string {
	var lines []string
	for _, step := range job.Steps {
		for _, line := range strings.Split(step.Run, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

func setupGoVersion(job workflowJobContract) string {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "actions/setup-go@") {
			return step.With["go-version"]
		}
	}
	return ""
}

func TestGoWorkflow_MacOSRunsOnlyTheDarwinSetWithItsCoverageGatesAndTheE2ETier(t *testing.T) {
	t.Parallel()
	jobs := readWorkflowContract(t, "go.yml").Jobs
	var macJobs []string
	for name, job := range jobs {
		if strings.HasPrefix(job.RunsOn, "macos") {
			macJobs = append(macJobs, name)
		}
	}
	if !slices.Equal(macJobs, []string{"build-test-darwin"}) {
		t.Fatalf("macOS jobs = %v; want only build-test-darwin", macJobs)
	}
	mac := jobs["build-test-darwin"]
	if mac.If != "" || mac.ContinueOnError != "" || setupGoVersion(mac) != "1.23" {
		t.Errorf("the macOS job must always run on Go 1.23 and fail the suite: if=%q continue-on-error=%q go=%q", mac.If, mac.ContinueOnError, setupGoVersion(mac))
	}
	lines := runLines(mac)
	for _, want := range []string{"make test-integration-darwin", `make cover-changed-darwin COVER_CHANGED_BASE="$base"`, "make test-e2e", "make cover-strict-darwin"} {
		if !slices.Contains(lines, want) {
			t.Errorf("the macOS job does not run %q", want)
		}
	}
	for _, line := range lines {
		if slices.Contains([]string{"make test-integration", "make cover-strict", "make apicover-check"}, line) || strings.HasPrefix(line, "make cover-changed ") {
			t.Errorf("the macOS job runs the whole-module %q; ubuntu Go 1.23 owns it", line)
		}
	}
	integration, cover := -1, -1
	for i, step := range mac.Steps {
		if step.Run != "" && (step.If != "" || step.ContinueOnError != "") {
			t.Errorf("macOS step %q may not skip or ignore its failure", step.Run)
		}
		if strings.Contains(step.Run, "make test-integration-darwin") {
			integration = i
		}
		if strings.Contains(step.Run, "make cover-changed-darwin") {
			cover = i
			if step.Env["COVER_BASE"] != "${{ github.event.pull_request.base.sha || github.event.before }}" {
				t.Errorf("the macOS COVER_BASE = %q, want the PR base or the pushed-over commit", step.Env["COVER_BASE"])
			}
		}
	}
	if integration < 0 || cover != integration+1 {
		t.Errorf("macOS cover-changed step at %d, darwin-set test step at %d; want it right after the profile is written", cover, integration)
	}
}

func TestGoWorkflow_Ubuntu123OwnsEveryWholeModuleGateAndGo127RunsTheSuiteOnly(t *testing.T) {
	t.Parallel()
	jobs := readWorkflowContract(t, "go.yml").Jobs
	full, current := jobs["build-test"], jobs["build-test-current"]
	if full.RunsOn != "ubuntu-latest" || setupGoVersion(full) != "1.23" {
		t.Errorf("build-test runs on %q with Go %q; want ubuntu-latest and 1.23", full.RunsOn, setupGoVersion(full))
	}
	for _, want := range []string{"make test-integration", `make cover-changed COVER_CHANGED_BASE="$base"`, "make apicover-check", "make cover-strict", "make test-e2e"} {
		if !slices.Contains(runLines(full), want) {
			t.Errorf("ubuntu Go 1.23 does not run %q", want)
		}
	}
	if current.RunsOn != "ubuntu-latest" || setupGoVersion(current) != "1.27" || current.If != "" || current.ContinueOnError != "" {
		t.Errorf("build-test-current runs on %q with Go %q (if=%q); want ubuntu-latest, 1.27, always", current.RunsOn, setupGoVersion(current), current.If)
	}
	lines := runLines(current)
	if !slices.Contains(lines, "make test-integration") {
		t.Error("Go 1.27 does not run the whole suite")
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "make cover") || strings.HasPrefix(line, "make apicover") || strings.HasPrefix(line, "make test-e2e") {
			t.Errorf("Go 1.27 runs %q; its gates and the e2e tier belong to the Go 1.23 jobs", line)
		}
	}
}

func TestMakefile_DarwinTargetsReadTheOneDarwinSetFile(t *testing.T) {
	t.Parallel()
	makefile, err := os.ReadFile(filepath.Join(workflowRepoRoot(t), "go", "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(makefile)
	if !regexp.MustCompile(`(?m)^DARWIN_SET := ` + regexp.QuoteMeta(DarwinSetFile) + `$`).MatchString(text) {
		t.Errorf("the Makefile must name %s as DARWIN_SET", DarwinSetFile)
	}
	for _, want := range []string{"test-integration-darwin: INTEGRATION_SET = $(DARWIN_SET)", "test-integration-darwin: test-integration", "cover changed --base $(COVER_CHANGED_BASE) --root . --packages $(DARWIN_SET)", "cover strict --root . --packages $(DARWIN_SET)", "./cmd/darwinset > $(DARWIN_SET).tmp"} {
		if !strings.Contains(text, want) {
			t.Errorf("the Makefile lacks %q", want)
		}
	}
}
