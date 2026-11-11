//go:build acs

package cycle2324

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const lintRunMarker = "golangci-lint run"

const (
	requiredWorkflow = ".github/workflows/required.yml"
	pinFile          = "go/.golangci-version"
	configFile       = "go/.golangci.yml"
	lintBudget       = 10 * time.Minute
	fixtureBudget    = 3 * time.Minute
)

var (
	semverPin     = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	semverLiteral = regexp.MustCompile(`\bv?\d+\.\d+\.\d+\b`)
	needsExpr     = regexp.MustCompile(`\$\{\{\s*needs\.([A-Za-z0-9_-]+)\.(result|outputs\.([A-Za-z0-9_]+))\s*\}\}`)
	anyExpr       = regexp.MustCompile(`\$\{\{[^}]*\}\}`)
	hostVersion   = regexp.MustCompile(`has version (\S+)`)
	configFileRE  = regexp.MustCompile(`(^|/)\.golangci\.(ya?ml|toml|json)$`)
)

var forbiddenLintFlags = []string{
	"--config", "-c", "--no-config", "--disable", "-D", "--enable", "-E",
	"--exclude", "--new", "--issues-exit-code", "--default", "--fix",
}

type step struct {
	Run              string `yaml:"run"`
	Uses             string `yaml:"uses"`
	WorkingDirectory string `yaml:"working-directory"`
	ContinueOnError  any    `yaml:"continue-on-error"`
	Env              map[string]string
}

type job struct {
	If              string    `yaml:"if"`
	Needs           yaml.Node `yaml:"needs"`
	ContinueOnError any       `yaml:"continue-on-error"`
	Env             map[string]string
	Steps           []step
	Defaults        struct {
		Run struct {
			WorkingDirectory string `yaml:"working-directory"`
		} `yaml:"run"`
	} `yaml:"defaults"`
}

type workflow struct {
	jobs  map[string]job
	nodes map[string]yaml.Node
}

func loadWorkflow(t *testing.T) workflow {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(requiredWorkflow)))
	if err != nil {
		t.Fatalf("read %s: %v", requiredWorkflow, err)
	}
	var doc struct {
		Jobs map[string]yaml.Node `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", requiredWorkflow, err)
	}
	wf := workflow{jobs: map[string]job{}, nodes: doc.Jobs}
	for id, node := range doc.Jobs {
		var j job
		if err := node.Decode(&j); err != nil {
			t.Fatalf("decode job %s: %v", id, err)
		}
		wf.jobs[id] = j
	}
	return wf
}

func needsOf(t *testing.T, j job) []string {
	t.Helper()
	switch j.Needs.Kind {
	case 0:
		return nil
	case yaml.ScalarNode:
		return []string{j.Needs.Value}
	}
	var ids []string
	if err := j.Needs.Decode(&ids); err != nil {
		t.Fatalf("decode needs: %v", err)
	}
	return ids
}

func (wf workflow) lintJob(t *testing.T) (string, job) {
	t.Helper()
	var ids []string
	for id, j := range wf.jobs {
		for _, s := range j.Steps {
			if strings.Contains(s.Run, lintRunMarker) {
				ids = append(ids, id)
				break
			}
		}
	}
	sort.Strings(ids)
	if len(ids) != 1 {
		t.Fatalf("%s must have exactly one job whose run step calls %q, found %v", requiredWorkflow, lintRunMarker, ids)
	}
	return ids[0], wf.jobs[ids[0]]
}

func lintStep(t *testing.T, j job) (step, string) {
	t.Helper()
	for _, s := range j.Steps {
		for _, line := range strings.Split(s.Run, "\n") {
			if strings.Contains(line, lintRunMarker) {
				return s, strings.TrimSpace(line)
			}
		}
	}
	t.Fatalf("no run step calls %q", lintRunMarker)
	return step{}, ""
}

func lintWorkingDir(j job, s step) string {
	if s.WorkingDirectory != "" {
		return s.WorkingDirectory
	}
	return j.Defaults.Run.WorkingDirectory
}

func runBash(t *testing.T, dir string, script string, env []string, budget time.Duration) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-e", "-o", "pipefail", "-c", script)
	cmd.Dir = dir
	cmd.Env = env
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode()
	}
	t.Fatalf("run bash: %v", err)
	return "", -1
}

type scenario map[string]map[string]string

func (s scenario) resolve(value string, lintID string) string {
	value = needsExpr.ReplaceAllStringFunc(value, func(m string) string {
		parts := needsExpr.FindStringSubmatch(m)
		id, kind, name := parts[1], parts[2], parts[3]
		fields := s[id]
		if kind == "result" {
			return fields["result"]
		}
		if id == lintID {
			if r := fields["result"]; r == "success" || r == "failure" {
				return r
			}
			return ""
		}
		return fields[name]
	})
	return anyExpr.ReplaceAllString(value, "")
}

func requiredGateOutcome(t *testing.T, wf workflow, lintID string, sc scenario) int {
	t.Helper()
	gate, ok := wf.jobs["required"]
	if !ok {
		t.Fatalf("%s has no required job", requiredWorkflow)
	}
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	for k, v := range gate.Env {
		env = append(env, k+"="+sc.resolve(v, lintID))
	}
	var script strings.Builder
	for _, s := range gate.Steps {
		for k, v := range s.Env {
			env = append(env, k+"="+sc.resolve(v, lintID))
		}
		script.WriteString(s.Run)
		script.WriteString("\n")
	}
	_, code := runBash(t, t.TempDir(), script.String(), env, fixtureBudget)
	return code
}

func goRouteScenario(lintID, lintResult string) scenario {
	return scenario{
		"changes":  {"result": "success", "go": "true", "landing": "false", "docs_only": "false", "docs": "false", "inbox": "false"},
		"validate": {"result": "success", "plugin_result": "success", "acs_result": "success"},
		"go":       {"result": "success", "suite_result": "success"},
		"landing":  {"result": "skipped"},
		"inbox":    {"result": "skipped"},
		"ste-lint": {"result": "skipped"},
		lintID:     {"result": lintResult},
	}
}

func TestC2324_001_RequiredGateFailsWhenLintFailsAndPassesWhenLintPasses(t *testing.T) {
	wf := loadWorkflow(t)
	lintID, _ := wf.lintJob(t)
	if !slices.Contains(needsOf(t, wf.jobs["required"]), lintID) {
		t.Fatalf("required job needs %v, missing lint job %q — a red lint would not stop the merge", needsOf(t, wf.jobs["required"]), lintID)
	}
	cases := []struct {
		name       string
		sc         scenario
		wantFailed bool
	}{
		{"lint passes on a go route", goRouteScenario(lintID, "success"), false},
		{"lint fails on a go route", goRouteScenario(lintID, "failure"), true},
		{"lint cancelled on a go route", goRouteScenario(lintID, "cancelled"), true},
		{"lint skipped on a go route", goRouteScenario(lintID, "skipped"), true},
	}
	noGoRoute := goRouteScenario(lintID, "skipped")
	noGoRoute["changes"]["go"] = "false"
	noGoRoute["changes"]["landing"] = "true"
	noGoRoute["go"] = map[string]string{"result": "skipped"}
	noGoRoute["landing"] = map[string]string{"result": "success", "suite_result": "success"}
	cases = append(cases, struct {
		name       string
		sc         scenario
		wantFailed bool
	}{"lint skipped on a route with no go change", noGoRoute, false})
	for _, c := range cases {
		code := requiredGateOutcome(t, wf, lintID, c.sc)
		if failed := code != 0; failed != c.wantFailed {
			t.Errorf("%s: required gate exit=%d, wantFailed=%v", c.name, code, c.wantFailed)
		}
	}
}

func fixtureModule(t *testing.T, source string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "go")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(configFile)))
	if err != nil {
		t.Fatalf("read %s: %v", configFile, err)
	}
	files := map[string]string{
		"go.mod":        "module fixture\n\ngo 1.23\n",
		"fixture.go":    source,
		".golangci.yml": string(config),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func lintEnv(t *testing.T) []string {
	t.Helper()
	return []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "GOLANGCI_LINT_CACHE=" + t.TempDir()}
}

const uncheckedErrorSource = `package fixture

import "os"

func Remove() {
	os.Remove("leftover")
}
`

const checkedErrorSource = `package fixture

import "os"

func Remove() error {
	return os.Remove("leftover")
}
`

func TestC2324_002_WorkflowLintCommandRejectsAnUncheckedError(t *testing.T) {
	wf := loadWorkflow(t)
	_, j := wf.lintJob(t)
	s, command := lintStep(t, j)
	if wd := lintWorkingDir(j, s); wd != "go" {
		t.Fatalf("lint step working directory = %q, want %q (the module that holds %s)", wd, "go", configFile)
	}
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Fatalf("golangci-lint is not on PATH: %v", err)
	}
	badOut, badCode := runBash(t, fixtureModule(t, uncheckedErrorSource), command, lintEnv(t), fixtureBudget)
	if badCode == 0 {
		t.Errorf("workflow lint command %q exited 0 on a fixture with an unchecked error — CI would let it merge", command)
	}
	if !strings.Contains(badOut, "errcheck") {
		t.Errorf("workflow lint command did not report errcheck on the unchecked error:\n%s", acsassert.OutputTail(badOut, 15))
	}
	cleanOut, cleanCode := runBash(t, fixtureModule(t, checkedErrorSource), command, lintEnv(t), fixtureBudget)
	if cleanCode != 0 {
		t.Errorf("workflow lint command %q exited %d on a clean fixture:\n%s", command, cleanCode, acsassert.OutputTail(cleanOut, 15))
	}
}

func TestC2324_003_LintJobFollowsTheGoRouteAndNeverSoftFails(t *testing.T) {
	wf := loadWorkflow(t)
	_, j := wf.lintJob(t)
	if got := strings.Join(strings.Fields(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(j.If), "${{"), "}}")), " "); got != "needs.changes.outputs.go == 'true'" {
		t.Errorf("lint job if = %q, want it to follow the go route: needs.changes.outputs.go == 'true'", j.If)
	}
	if !slices.Contains(needsOf(t, j), "changes") {
		t.Errorf("lint job needs %v, missing %q", needsOf(t, j), "changes")
	}
	if softFail(j.ContinueOnError) {
		t.Errorf("lint job sets continue-on-error: %v", j.ContinueOnError)
	}
	for _, s := range j.Steps {
		if softFail(s.ContinueOnError) {
			t.Errorf("a lint job step sets continue-on-error: %v", s.ContinueOnError)
		}
	}
	_, command := lintStep(t, j)
	for _, masking := range []string{"||", "|", ";"} {
		if strings.Contains(command, masking) {
			t.Errorf("lint command %q contains %q, which can hide a non-zero exit", command, masking)
		}
	}
}

func softFail(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return strings.TrimSpace(x) != "false"
	}
	return true
}

func TestC2324_004_LintVersionHasOneHomeThatCIReads(t *testing.T) {
	pin := readPin(t)
	wf := loadWorkflow(t)
	lintID, _ := wf.lintJob(t)
	text, err := yaml.Marshal(wf.nodes[lintID])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), ".golangci-version") {
		t.Errorf("lint job never reads %s, so CI would not use the pinned version", pinFile)
	}
	if lit := semverLiteral.FindString(string(text)); lit != "" {
		t.Errorf("lint job hard-codes the version %q; it must come from %s (%s)", lit, pinFile, pin)
	}
	workflowsDir := filepath.Join(acsassert.RepoRoot(t), ".github", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		body, err := os.ReadFile(filepath.Join(workflowsDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, "golangci") && semverLiteral.MatchString(line) {
				t.Errorf("%s:%d pins a golangci-lint version in a second place: %s", e.Name(), i+1, strings.TrimSpace(line))
			}
		}
	}
}

func readPin(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pinFile)))
	if err != nil {
		t.Fatalf("version pin %s missing: %v", pinFile, err)
	}
	pin := strings.TrimSpace(string(raw))
	if !semverPin.MatchString(pin) {
		t.Fatalf("%s holds %q, want an exact version such as 2.14.0", pinFile, pin)
	}
	if !strings.HasPrefix(pin, "2.") {
		t.Fatalf("%s holds %q, but %s is a golangci-lint v2 config", pinFile, pin, configFile)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", pinFile); code != 0 {
		t.Fatalf("%s is untracked — it may be gitignored and dropped at ship", pinFile)
	}
	return pin
}

func TestC2324_005_GateAndCIShareOneConfigAndOneVersion(t *testing.T) {
	pin := readPin(t)
	root := acsassert.RepoRoot(t)
	tracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files")
	if err != nil || code != 0 {
		t.Fatalf("git ls-files: code=%d err=%v", code, err)
	}
	var configs []string
	for _, f := range strings.Split(tracked, "\n") {
		if configFileRE.MatchString(f) {
			configs = append(configs, f)
		}
	}
	if len(configs) != 1 || configs[0] != configFile {
		t.Errorf("tracked golangci configs = %v, want exactly [%s]", configs, configFile)
	}
	wf := loadWorkflow(t)
	_, j := wf.lintJob(t)
	_, command := lintStep(t, j)
	for _, tok := range strings.Fields(command) {
		for _, flag := range forbiddenLintFlags {
			if tok == flag || strings.HasPrefix(tok, flag+"=") {
				t.Errorf("lint command %q uses %q, which makes CI differ from the gate's plain `golangci-lint run`", command, tok)
			}
		}
	}
	acsassert.FileNotContains(t, filepath.Join(root, filepath.FromSlash(configFile)), "CI does not run golangci")
	out, _, vcode, verr := acsassert.SubprocessOutput("golangci-lint", "--version")
	if verr != nil || vcode != 0 {
		t.Fatalf("golangci-lint --version: code=%d err=%v", vcode, verr)
	}
	m := hostVersion.FindStringSubmatch(out)
	if m == nil || m[1] != pin {
		t.Errorf("the gate's golangci-lint reports %q, but %s pins %s", strings.TrimSpace(out), pinFile, pin)
	}
}

func TestC2324_006_MainPassesTheWorkflowLintCommand(t *testing.T) {
	wf := loadWorkflow(t)
	_, j := wf.lintJob(t)
	s, command := lintStep(t, j)
	dir := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(lintWorkingDir(j, s)))
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Fatalf("golangci-lint is not on PATH: %v", err)
	}
	out, code := runBash(t, dir, command, os.Environ(), lintBudget)
	if code != 0 {
		t.Errorf("%q exited %d in %s — main holds lint debt, so the new CI job would be red on merge:\n%s", command, code, dir, acsassert.OutputTail(out, 30))
	}
}
