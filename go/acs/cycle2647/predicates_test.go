//go:build acs

package cycle2647

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	runBound          = 8 * time.Minute
	parityTest        = "TestFloorRun_TierCommandsMatchRequiredCI"
	samplerSentinel   = "hostsampler-stderr-sentinel-2647"
	steLintJob        = "ste-lint"
	requiredWorkflow  = "required.yml"
	goWorkflow        = "go.yml"
	floorTestFile     = "go/cmd/evolve/cmd_floor_test.go"
	evolveBuildScript = `go build -o "$RUNNER_TEMP/evolve" ./cmd/evolve`
)

var (
	evolveBinary    string
	evolveBuild     = regexp.MustCompile(`go build .*cmd/evolve`)
	yamlCheckInvoke = regexp.MustCompile(`"?\$\{?RUNNER_TEMP\}?/evolve"?\s+yaml\s+check\b`)
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "evolve-acs-2647-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	evolveBinary = filepath.Join(dir, "evolve")
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if stdout, stderr, _, err := acsassert.SubprocessOutputIn(moduleDir, "go", "build", "-o", evolveBinary, "./cmd/evolve"); err != nil {
		fmt.Fprintf(os.Stderr, "go build ./cmd/evolve: %v\n%s%s", err, stdout, stderr)
		os.RemoveAll(dir)
		os.Exit(2)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type workflowStep struct {
	Name             string         `yaml:"name"`
	If               string         `yaml:"if"`
	Uses             string         `yaml:"uses"`
	Run              string         `yaml:"run"`
	WorkingDirectory string         `yaml:"working-directory"`
	With             map[string]any `yaml:"with"`
}

type workflowJob struct {
	Defaults struct {
		Run struct {
			WorkingDirectory string `yaml:"working-directory"`
		} `yaml:"run"`
	} `yaml:"defaults"`
	Steps []workflowStep `yaml:"steps"`
}

type workflowFile struct {
	Jobs map[string]workflowJob `yaml:"jobs"`
}

func loadWorkflow(t *testing.T, root, name string) workflowFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	var wf workflowFile
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return wf
}

func evolveKeys() []string {
	var args []string
	for _, kv := range os.Environ() {
		if key, _, _ := strings.Cut(kv, "="); strings.HasPrefix(key, "EVOLVE_") {
			args = append(args, "-u", key)
		}
	}
	return args
}

func runBash(t *testing.T, dir string, assignments []string, script string) (string, int) {
	t.Helper()
	args := append(evolveKeys(), assignments...)
	args = append(args, "bash", "-e", "-c", script)
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(dir, "env", args...)
	if err != nil && code < 0 {
		t.Fatalf("bash: %v\n%s%s", err, stdout, stderr)
	}
	return stdout + stderr, code
}

func stepWorkingDir(job workflowJob, step workflowStep) string {
	if step.WorkingDirectory != "" {
		return step.WorkingDirectory
	}
	return job.Defaults.Run.WorkingDirectory
}

type yamlCheckStep struct {
	job   workflowJob
	step  workflowStep
	index int
}

func findYamlCheckStep(t *testing.T, root string) yamlCheckStep {
	t.Helper()
	job, ok := loadWorkflow(t, root, requiredWorkflow).Jobs[steLintJob]
	if !ok {
		t.Fatalf("%s has no %s job", requiredWorkflow, steLintJob)
	}
	var found []yamlCheckStep
	for i, step := range job.Steps {
		if strings.Contains(step.Run, "yaml check") {
			found = append(found, yamlCheckStep{job: job, step: step, index: i})
		}
	}
	if len(found) != 1 {
		t.Fatalf("RED: %s job %q has %d step(s) running `yaml check`, want exactly 1", requiredWorkflow, steLintJob, len(found))
	}
	return found[0]
}

func runYamlCheckStep(t *testing.T, found yamlCheckStep, workspace string) (string, int) {
	t.Helper()
	runnerTemp := t.TempDir()
	if err := os.Symlink(evolveBinary, filepath.Join(runnerTemp, "evolve")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(workspace, stepWorkingDir(found.job, found.step))
	return runBash(t, dir, []string{"RUNNER_TEMP=" + runnerTemp, "GITHUB_WORKSPACE=" + workspace}, found.step.Run)
}

func TestC2647_001_SteLintJobRunsYamlCheckOnTheBinaryItAlreadyBuilds(t *testing.T) {
	root := acsassert.RepoRoot(t)
	found := findYamlCheckStep(t, root)
	builds := 0
	buildAt := -1
	for i, step := range found.job.Steps {
		if evolveBuild.MatchString(step.Run) {
			builds++
			buildAt = i
		}
	}
	if builds != 1 {
		t.Fatalf("RED: %s job %q builds evolve %d times, want the one existing build reused", requiredWorkflow, steLintJob, builds)
	}
	if !strings.Contains(found.job.Steps[buildAt].Run, evolveBuildScript) {
		t.Errorf("the evolve build step no longer writes $RUNNER_TEMP/evolve: %q", found.job.Steps[buildAt].Run)
	}
	if found.index <= buildAt {
		t.Errorf("yaml check step (index %d) runs before the evolve build step (index %d)", found.index, buildAt)
	}
	if !yamlCheckInvoke.MatchString(found.step.Run) {
		t.Errorf("yaml check step does not run the already-built binary \"$RUNNER_TEMP/evolve\": %q", found.step.Run)
	}
	if strings.Contains(found.step.Run, "go run") {
		t.Errorf("yaml check step rebuilds through `go run`: %q", found.step.Run)
	}
}

func TestC2647_002_YamlCheckStepPassesOnTheRealTree(t *testing.T) {
	root := acsassert.RepoRoot(t)
	found := findYamlCheckStep(t, root)
	out, code := runYamlCheckStep(t, found, root)
	if code != 0 {
		t.Errorf("RED: the ste-lint yaml check step exited %d on this tree:\n%s", code, out)
	}
	if !strings.Contains(out, "yaml check:") {
		t.Errorf("the step never reached `evolve yaml check` (no verdict line):\n%s", out)
	}
}

func TestC2647_003_YamlCheckStepFailsOnABrokenWorkflow(t *testing.T) {
	root := acsassert.RepoRoot(t)
	found := findYamlCheckStep(t, root)

	valid := gittest.Fixture(t)
	writeWorkflow(t, valid.Dir, "ok.yml", "name: ok\non: push\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n")
	if out, code := runYamlCheckStep(t, found, valid.Dir); code != 0 {
		t.Errorf("RED: a valid workflow tree made the step exit %d:\n%s", code, out)
	}

	broken := gittest.Fixture(t)
	writeWorkflow(t, broken.Dir, "broken.yml", "name: broken\n")
	out, code := runYamlCheckStep(t, found, broken.Dir)
	if code == 0 {
		t.Errorf("RED: a workflow without on/jobs did not fail the step:\n%s", out)
	}
	if !strings.Contains(out, "broken.yml") {
		t.Errorf("the failing step does not name broken.yml:\n%s", out)
	}
}

func writeWorkflow(t *testing.T, dir, name, body string) {
	t.Helper()
	wfDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type goTestEvent struct {
	Action string
	Test   string
}

func TestC2647_004_ParityTestCoversTheYamlTier(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, _, runErr := acsassert.SubprocessOutputIn(filepath.Join(root, "go"), "go", "test", "-json", "-count=1", "-run", "^"+parityTest+"$", "./cmd/evolve")

	passed := map[string]bool{}
	failed := []string{}
	for _, line := range strings.Split(stdout, "\n") {
		var ev goTestEvent
		if json.Unmarshal([]byte(line), &ev) != nil || ev.Test == "" {
			continue
		}
		switch ev.Action {
		case "pass":
			passed[ev.Test] = true
		case "fail":
			failed = append(failed, ev.Test)
		}
	}
	if runErr != nil || len(failed) > 0 {
		t.Fatalf("go test %s: err=%v failed=%v\nstdout:\n%s\nstderr:\n%s", parityTest, runErr, failed, acsassert.OutputTail(stdout, 40), stderr)
	}
	if !passed[parityTest] {
		t.Errorf("%s did not run to a pass", parityTest)
	}
	yamlRows := 0
	for name := range passed {
		if strings.HasPrefix(name, parityTest+"/yaml/") {
			yamlRows++
		}
	}
	if yamlRows == 0 {
		t.Errorf("RED: %s has no passing yaml/<command> subtest, so floorCIParity has no yaml row", parityTest)
	}
}

type parityRow map[string]string

func parityRows(t *testing.T, root string) []parityRow {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, floorTestFile), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var rows []parityRow
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || spec.Names[0].Name != "floorCIParity" || len(spec.Values) != 1 {
			return true
		}
		table, ok := spec.Values[0].(*ast.CompositeLit)
		if !ok {
			return false
		}
		for _, elt := range table.Elts {
			lit, ok := elt.(*ast.CompositeLit)
			if !ok {
				continue
			}
			row := parityRow{}
			for _, field := range lit.Elts {
				kv, ok := field.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, _ := kv.Key.(*ast.Ident)
				val, _ := kv.Value.(*ast.BasicLit)
				if key == nil || val == nil || val.Kind != token.STRING {
					continue
				}
				if s, err := strconv.Unquote(val.Value); err == nil {
					row[key.Name] = s
				}
			}
			rows = append(rows, row)
		}
		return false
	})
	return rows
}

func TestC2647_005_YamlParityRowPointsAtTheSteLintJob(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rows := parityRows(t, root)
	if len(rows) == 0 {
		t.Fatal("floorCIParity yielded no rows")
	}
	i := slices.IndexFunc(rows, func(r parityRow) bool { return r["tier"] == "yaml" })
	if i < 0 {
		t.Fatalf("RED: floorCIParity has no yaml row among %d rows", len(rows))
	}
	row := rows[i]
	if row["workflow"] != requiredWorkflow || row["job"] != steLintJob {
		t.Errorf("yaml row points at %s/%s, want %s/%s", row["workflow"], row["job"], requiredWorkflow, steLintJob)
	}
	if !strings.Contains(row["want"], "yaml check") {
		t.Errorf("yaml row wants %q, which is not the yaml check command", row["want"])
	}
}

func TestC2647_006_HostSamplerStderrIsKeptAndUploaded(t *testing.T) {
	root := acsassert.RepoRoot(t)
	wf := loadWorkflow(t, root, goWorkflow)
	var samplerJobs []string
	for name, job := range wf.Jobs {
		if samplerStepIndex(job) >= 0 {
			samplerJobs = append(samplerJobs, name)
		}
	}
	slices.Sort(samplerJobs)
	for _, want := range []string{"build-test", "build-test-darwin"} {
		if !slices.Contains(samplerJobs, want) {
			t.Fatalf("job %q does not start the host sampler (sampler jobs: %v)", want, samplerJobs)
		}
	}
	for _, name := range samplerJobs {
		t.Run(name, func(t *testing.T) { assertSamplerStderrKept(t, wf.Jobs[name]) })
	}
}

func samplerStepIndex(job workflowJob) int {
	return slices.IndexFunc(job.Steps, func(step workflowStep) bool {
		return strings.Contains(step.Run, "hostsampler -out")
	})
}

func assertSamplerStderrKept(t *testing.T, job workflowJob) {
	t.Helper()
	step := job.Steps[samplerStepIndex(job)]
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, stepWorkingDir(job, step)), 0o755); err != nil {
		t.Fatal(err)
	}
	shims := t.TempDir()
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(shims, "hostsampler-stub")
	writeExecutable(t, stub, "#!/bin/sh\necho \"$SAMPLER_SENTINEL\" >&2\necho sampler-stdout-noise\n")
	writeExecutable(t, filepath.Join(shims, "go"), "#!/bin/sh\nif [ \"$1\" = build ] && [ \"$2\" = -o ]; then\n  mkdir -p \"$(dirname \"$3\")\"\n  cp \"$SAMPLER_STUB\" \"$3\"\n  chmod +x \"$3\"\n  exit 0\nfi\nexec \"$REAL_GO\" \"$@\"\n")

	env := []string{
		"GITHUB_WORKSPACE=" + workspace,
		"PATH=" + shims + string(os.PathListSeparator) + os.Getenv("PATH"),
		"SAMPLER_STUB=" + stub,
		"SAMPLER_SENTINEL=" + samplerSentinel,
		"REAL_GO=" + realGo,
	}
	out, code := runBash(t, filepath.Join(workspace, stepWorkingDir(job, step)), env, step.Run+"\nwait\n")
	if code != 0 {
		t.Fatalf("the sampler step exited %d:\n%s", code, out)
	}

	var kept []string
	walkErr := filepath.WalkDir(workspace, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
			return err
		}
		raw, readErr := os.ReadFile(p)
		if readErr == nil && bytes.Contains(raw, []byte(samplerSentinel)) {
			rel, _ := filepath.Rel(workspace, p)
			kept = append(kept, filepath.ToSlash(rel))
		}
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	if len(kept) == 0 {
		t.Fatalf("RED: the sampler's stderr reached no file under the workspace (step output: %q)", out)
	}
	uploaded := alwaysUploadedPaths(job)
	for _, rel := range kept {
		if !slices.ContainsFunc(uploaded, func(pattern string) bool {
			ok, _ := path.Match(pattern, rel)
			return ok
		}) {
			t.Errorf("RED: sampler stderr file %s is not in any `if: always()` upload-artifact path list %v", rel, uploaded)
		}
	}
}

func writeExecutable(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func alwaysUploadedPaths(job workflowJob) []string {
	var paths []string
	for _, step := range job.Steps {
		if !strings.HasPrefix(step.Uses, "actions/upload-artifact") || !strings.Contains(step.If, "always()") {
			continue
		}
		raw, _ := step.With["path"].(string)
		for _, line := range strings.Split(raw, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				paths = append(paths, line)
			}
		}
	}
	return paths
}

func gitLines(t *testing.T, root string, args ...string) []string {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root}, args...)...)
	if err != nil || code != 0 {
		t.Fatalf("git %v: exit=%d err=%v\n%s", args, code, err, stderr)
	}
	return strings.Fields(out)
}

func TestC2647_007_NoMacOSSpawnShimInThisChange(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base := "HEAD"
	if refs := gitLines(t, root, "branch", "-a", "--list", "origin/main", "main"); len(refs) > 0 {
		ref := strings.TrimPrefix(refs[len(refs)-1], "*")
		if mb := gitLines(t, root, "merge-base", "HEAD", ref); len(mb) == 1 {
			base = mb[0]
		}
	}
	changed := append(gitLines(t, root, "diff", "--name-only", base), gitLines(t, root, "ls-files", "--others", "--exclude-standard")...)
	for _, file := range changed {
		lower := strings.ToLower(file)
		switch {
		case strings.Contains(path.Base(lower), "shim"):
			t.Errorf("changed file %s is a shim, which the operator did not approve", file)
		case strings.HasPrefix(file, "go/internal/sysexec/"):
			t.Errorf("changed file %s is in the spawn layer, which this change must not touch", file)
		case regexp.MustCompile(`_darwin(_test)?\.go$`).MatchString(file):
			t.Errorf("changed file %s is darwin-only Go code, which this change must not add or edit", file)
		}
	}
	for _, name := range []string{goWorkflow, requiredWorkflow} {
		for jobName, job := range loadWorkflow(t, root, name).Jobs {
			for _, step := range job.Steps {
				if strings.Contains(strings.ToLower(step.Name+" "+step.Run), "shim") {
					t.Errorf("%s job %q step %q installs a shim", name, jobName, step.Name)
				}
			}
		}
	}
}
