//go:build acs

package cycle2610

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/landcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	docReadByARegistryRow = "docs/operations/runtime-reference.md"
	docOfTheReplay        = "docs/plans/harness-practices-2026-11.md"
	docReadThroughConst   = "docs/plans/harness-removal-test-2026-11.md"
	cmdEvolve             = "./cmd/evolve"
	usageExit             = 10
)

var (
	binaryOnce sync.Once
	binaryPath string
	binaryErr  error
	binaryDir  string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binaryDir != "" {
		_ = os.RemoveAll(binaryDir)
	}
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	binaryOnce.Do(func() {
		binaryDir, binaryErr = os.MkdirTemp("", "evolve-c2610-")
		if binaryErr != nil {
			return
		}
		binaryPath = filepath.Join(binaryDir, "evolve")
		cmd := sysexec.Command(context.Background(), "go", "build", "-o", binaryPath, "./cmd/evolve")
		cmd.Dir = acsassert.GoModuleDir(t)
		if out, err := cmd.CombinedOutput(); err != nil {
			binaryErr = errors.New("go build ./cmd/evolve: " + err.Error() + "\n" + string(out))
		}
	})
	if binaryErr != nil {
		t.Fatalf("the evolve binary must build: %v", binaryErr)
	}
	return binaryPath
}

func nulPaths(paths ...string) string {
	var b strings.Builder
	for _, p := range paths {
		b.WriteString(p)
		b.WriteByte(0)
	}
	return b.String()
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func docsTestsViaBinary(t *testing.T, root, stdin string, flags ...string) (string, string, int) {
	t.Helper()
	cmd := sysexec.Command(context.Background(), evolveBinary(t), append([]string{"ci", "docs-tests"}, flags...)...)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return out.String(), errb.String(), exitCodeOf(err)
}

func docsTestsViaRunCI(root, stdin string, flags ...string) (string, string, int) {
	deps := landcmd.Deps{ResolveRoot: func(projectRoot string, _ io.Writer) (string, error) { return projectRoot, nil }}
	var out, errb bytes.Buffer
	args := append([]string{"docs-tests", "--project-root", root}, flags...)
	rc := landcmd.RunCI(args, strings.NewReader(stdin), &out, &errb, deps)
	return out.String(), errb.String(), rc
}

func parseSelection(t *testing.T, out string) map[string]string {
	t.Helper()
	sel := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		pkg, re, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("selection line %q is not <package><TAB><regexp>", line)
		}
		sel[pkg] = re
	}
	return sel
}

func selects(t *testing.T, sel map[string]string, pkg, test string) bool {
	t.Helper()
	re, ok := sel[pkg]
	if !ok {
		return false
	}
	hit, err := regexp.MatchString(re, test)
	if err != nil {
		t.Fatalf("selection regexp %q for %s does not compile: %v", re, pkg, err)
	}
	return hit
}

func selectReal(t *testing.T, changed ...string) map[string]string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, errOut, rc := docsTestsViaBinary(t, root, nulPaths(changed...), "--project-root", root)
	if rc != 0 {
		t.Fatalf("evolve ci docs-tests %v exit %d\nstdout: %s\nstderr: %s", changed, rc, out, errOut)
	}
	return parseSelection(t, out)
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func docReaderFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go/go.mod": "module example.test/fx\n\ngo 1.22\n",
		"go/internal/x/x_test.go": `package x

import "testing"

const spacedDoc = "docs/with space.md"

func TestReadsIt(t *testing.T) { _ = "docs/read.md" }

func TestReadsNeighbour(t *testing.T) { _ = "docs/read.md.bak" }

func TestReadsSpaced(t *testing.T) { _ = spacedDoc }
`,
		"go/internal/x/prod.go": "package x\n\nconst ProdDoc = \"docs/prod-only.md\"\n",
		"go/internal/y/y_test.go": `//go:build acs

package y

import "testing"

func TestTagged(t *testing.T) { _ = "docs/tagged.md" }
`,
	})
	return root
}

func TestC2610_001_ADocsOnlyChangeToADocAGoTestReadsSelectsThatTest(t *testing.T) {
	sel := selectReal(t, docReadByARegistryRow)
	if !selects(t, sel, cmdEvolve, "TestHandoffVerbs_AreInTheRegistryTheUsageTextAndTheRuntimeReference") {
		t.Errorf("RED: changing %s must select the test that reads it; selection = %v", docReadByARegistryRow, sel)
	}
}

func TestC2610_002_TheReplayOf1671SelectsTheTestThatWentRedOnMain(t *testing.T) {
	sel := selectReal(t, docOfTheReplay)
	for _, name := range []string{
		"TestRemovalDesignAmp_TheR9RowLinksOnlyToAnExistingDesign",
		"TestRemovalDesignAmp_EveryRelativeLinkResolves",
	} {
		if !selects(t, sel, cmdEvolve, name) {
			t.Errorf("RED: the replay of #1671 (change to %s) must select %s; selection = %v", docOfTheReplay, name, sel)
		}
	}
	if selects(t, sel, cmdEvolve, "TestLoopSkill_NoBudgetReferences") {
		t.Errorf("RED: a doc change must select the tests that read that doc, not the whole data-file pack: %v", sel)
	}
	for pkg := range sel {
		if strings.HasPrefix(pkg, "./acs/") {
			t.Errorf("RED: package %s is acs-tagged and has no test to run without its tag", pkg)
		}
	}
}

func TestC2610_003_ADocNamedByAConstantInAnotherTestFileSelectsItsReaders(t *testing.T) {
	sel := selectReal(t, docReadThroughConst)
	if !selects(t, sel, cmdEvolve, "TestRemovalDesignAmp_TheDesignHasOneSectionPerRequiredHeading") {
		t.Errorf("RED: %s is named by a const of another test file of the package; its reader must be selected: %v", docReadThroughConst, sel)
	}
}

func TestC2610_004_ADocNoGoTestReadsSelectsNothing(t *testing.T) {
	root := docReaderFixture(t)
	for _, doc := range []string{"docs/unread.md", "docs/prod-only.md"} {
		out, errOut, rc := docsTestsViaRunCI(root, nulPaths(doc))
		if rc != 0 || strings.TrimSpace(out) != "" {
			t.Errorf("RED: %s is read by no Go test; want exit 0 and no selection, got exit %d stdout %q stderr %q", doc, rc, out, errOut)
		}
	}
}

func TestC2610_005_ASelectionIsExactToThePathAndSkipsAcsTaggedFiles(t *testing.T) {
	root := docReaderFixture(t)
	out, errOut, rc := docsTestsViaRunCI(root, nulPaths("docs/read.md"))
	if rc != 0 {
		t.Fatalf("RED: exit %d stderr %q", rc, errOut)
	}
	sel := parseSelection(t, out)
	if len(sel) != 1 || !selects(t, sel, "./internal/x", "TestReadsIt") {
		t.Errorf("RED: docs/read.md must select only TestReadsIt of ./internal/x, got %v", sel)
	}
	for _, other := range []string{"TestReadsNeighbour", "TestReadsSpaced"} {
		if selects(t, sel, "./internal/x", other) {
			t.Errorf("RED: %s reads another path and must not be selected: %v", other, sel)
		}
	}
	tagged, _, rc := docsTestsViaRunCI(root, nulPaths("docs/tagged.md"))
	if rc != 0 || strings.TrimSpace(tagged) != "" {
		t.Errorf("RED: an acs-tagged test file is not part of go test; want no selection, got exit %d stdout %q", rc, tagged)
	}
}

func TestC2610_006_APathWithASpaceAndAConstReaderIsSelected(t *testing.T) {
	root := docReaderFixture(t)
	out, errOut, rc := docsTestsViaRunCI(root, nulPaths("docs/with space.md"))
	if rc != 0 {
		t.Fatalf("RED: exit %d stderr %q", rc, errOut)
	}
	if sel := parseSelection(t, out); !selects(t, sel, "./internal/x", "TestReadsSpaced") {
		t.Errorf("RED: NUL-separated path %q read through a const must select TestReadsSpaced: %v", "docs/with space.md", sel)
	}
}

func TestC2610_007_AnUnknownFlagIsAUsageErrorWithNoSelection(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if _, errOut, rc := docsTestsViaBinary(t, root, nulPaths(docOfTheReplay), "--project-root", root); rc != 0 {
		t.Fatalf("RED: the verb must accept --project-root, exit %d stderr %q", rc, errOut)
	}
	out, _, rc := docsTestsViaBinary(t, root, nulPaths(docOfTheReplay), "--no-such-flag")
	if rc != usageExit || strings.TrimSpace(out) != "" {
		t.Errorf("RED: an unknown flag must exit %d with nothing on stdout, got exit %d stdout %q", usageExit, rc, out)
	}
}

type workflowDoc struct {
	Jobs map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	Needs           yaml.Node      `yaml:"needs"`
	If              string         `yaml:"if"`
	ContinueOnError any            `yaml:"continue-on-error"`
	Steps           []workflowStep `yaml:"steps"`
}

type workflowStep struct {
	ID               string            `yaml:"id"`
	If               string            `yaml:"if"`
	Run              string            `yaml:"run"`
	Env              map[string]string `yaml:"env"`
	WorkingDirectory string            `yaml:"working-directory"`
	ContinueOnError  any               `yaml:"continue-on-error"`
}

func loadRequiredWorkflow(t *testing.T) workflowDoc {
	t.Helper()
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "required.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc workflowDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("required.yml does not parse: %v", err)
	}
	return doc
}

func (j workflowJob) needs(t *testing.T) []string {
	t.Helper()
	switch j.Needs.Kind {
	case yaml.ScalarNode:
		return []string{j.Needs.Value}
	case yaml.SequenceNode:
		var list []string
		if err := j.Needs.Decode(&list); err != nil {
			t.Fatal(err)
		}
		return list
	}
	return nil
}

func normalizedCondition(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(strings.TrimPrefix(s, "${{"), "}}")
	return strings.Join(strings.Fields(s), "")
}

func docsTestsStep(t *testing.T, wf workflowDoc) workflowStep {
	t.Helper()
	job, ok := wf.Jobs["docs-tests"]
	if !ok {
		t.Fatalf("RED: required.yml has no docs-tests job")
	}
	var hits []workflowStep
	for _, s := range job.Steps {
		if strings.Contains(s.Run, "ci docs-tests") {
			hits = append(hits, s)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("RED: the docs-tests job needs exactly one step that runs `ci docs-tests` and the selected go test; found %d", len(hits))
	}
	return hits[0]
}

func TestC2610_008_TheDocsTestsJobRunsOnTheDocsOnlyRouteAndCannotBeSkippedOrIgnored(t *testing.T) {
	wf := loadRequiredWorkflow(t)
	job, ok := wf.Jobs["docs-tests"]
	if !ok {
		t.Fatalf("RED: required.yml has no docs-tests job")
	}
	if !slices.Contains(job.needs(t), "changes") {
		t.Errorf("RED: docs-tests must need the changes job: %v", job.needs(t))
	}
	if got := normalizedCondition(job.If); got != "needs.changes.outputs.docs_only=='true'" {
		t.Errorf("RED: docs-tests must run exactly on the docs-only route, if = %q", job.If)
	}
	if job.ContinueOnError != nil {
		t.Errorf("RED: docs-tests may not continue after an error")
	}
	step := docsTestsStep(t, wf)
	if step.If != "" || step.ContinueOnError != nil {
		t.Errorf("RED: the docs-tests step may not be skipped or ignore its failure: if %q continue-on-error %v", step.If, step.ContinueOnError)
	}
	if !strings.Contains(step.Run, "go test") {
		t.Errorf("RED: the step that selects the tests must also run them with go test")
	}
}

type fakeBin struct {
	evolveOut  string
	goTestExit string
}

type stepOutcome struct {
	rc          int
	output      string
	evolveArgs  string
	evolveStdin []string
	goTests     []goCall
	repoGo      string
}

type goCall struct {
	pwd  string
	args []string
}

func docPR(t *testing.T, changed ...string) (dir, base, head string) {
	t.Helper()
	r := gittest.Fixture(t)
	changedTree := map[string]string{"go/.keep": "keep\n"}
	for _, rel := range changed {
		changedTree[rel] = "base\n"
	}
	writeTree(t, r.Dir, changedTree)
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "base")
	base = r.Git("rev-parse", "HEAD")
	for _, rel := range changed {
		changedTree[rel] = "changed\n"
	}
	writeTree(t, r.Dir, changedTree)
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "change")
	head = r.Git("rev-parse", "HEAD")
	return r.Dir, base, head
}

func shellEnv(extra map[string]string) []string {
	env := map[string]string{"PATH": os.Getenv("PATH"), "HOME": os.Getenv("HOME"), "LC_ALL": "C"}
	for k, v := range extra {
		env[k] = v
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func runScript(t *testing.T, dir, script string, env map[string]string) (string, int) {
	t.Helper()
	cmd := sysexec.Command(context.Background(), "bash", "-eo", "pipefail", "-c", script)
	cmd.Dir = dir
	cmd.Env = shellEnv(env)
	out, err := cmd.CombinedOutput()
	return string(out), exitCodeOf(err)
}

func readOrEmpty(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}

type eventEnv func(step workflowStep, base, head string) map[string]string

func literalPullRequestEnv(_ workflowStep, base, head string) map[string]string {
	return map[string]string{"EVENT_NAME": "pull_request", "BASE_SHA": base, "HEAD_SHA": head}
}

func workflowResolvedEnv(github map[string]string) eventEnv {
	return func(step workflowStep, base, head string) map[string]string {
		resolved := map[string]string{}
		for name, expr := range step.Env {
			resolved[name] = resolveExpression(expr, github, base, head)
		}
		return resolved
	}
}

func resolveExpression(expr string, github map[string]string, base, head string) string {
	trimmed := strings.TrimSpace(expr)
	if !strings.HasPrefix(trimmed, "${{") || !strings.HasSuffix(trimmed, "}}") {
		return expr
	}
	for _, operand := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(trimmed, "${{"), "}}"), "||") {
		operand = strings.TrimSpace(operand)
		value := github[operand]
		if strings.HasPrefix(operand, "'") {
			value = strings.Trim(operand, "'")
		}
		value = strings.NewReplacer("<base>", base, "<head>", head).Replace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

var (
	pullRequestContext = map[string]string{
		"github.event_name":                  "pull_request",
		"github.event.pull_request.base.sha": "<base>",
		"github.event.pull_request.head.sha": "<head>",
		"github.sha":                         "0123456789012345678901234567890123456789",
	}
	pushContext = map[string]string{
		"github.event_name":   "push",
		"github.event.before": "<base>",
		"github.sha":          "<head>",
	}
)

func runDocsTestsStep(t *testing.T, fake fakeBin, changed ...string) stepOutcome {
	t.Helper()
	return runDocsTestsStepOn(t, fake, literalPullRequestEnv, changed...)
}

func runDocsTestsStepOn(t *testing.T, fake fakeBin, eventEnv eventEnv, changed ...string) stepOutcome {
	t.Helper()
	step := docsTestsStep(t, loadRequiredWorkflow(t))
	repo, base, head := docPR(t, changed...)
	bin, runnerTemp, logs := t.TempDir(), t.TempDir(), t.TempDir()
	writeTree(t, bin, map[string]string{
		"go": "#!/bin/sh\nprintf '%s\\t%s\\n' \"$(pwd -P)\" \"$*\" >> \"$FAKE_GO_LOG\"\nif [ \"$1\" = test ]; then exit \"${FAKE_GO_TEST_EXIT:-0}\"; fi\nexit 0\n",
	})
	writeTree(t, runnerTemp, map[string]string{
		"evolve": "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$FAKE_EVOLVE_ARGS\"\ncat > \"$FAKE_EVOLVE_STDIN\"\nif [ -s \"$FAKE_EVOLVE_OUT\" ]; then cat \"$FAKE_EVOLVE_OUT\"; fi\n",
	})
	for _, exe := range []string{filepath.Join(bin, "go"), filepath.Join(runnerTemp, "evolve")} {
		if err := os.Chmod(exe, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTree(t, logs, map[string]string{"evolve-out": fake.evolveOut})
	goExit := fake.goTestExit
	if goExit == "" {
		goExit = "0"
	}
	env := map[string]string{
		"PATH":                bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"RUNNER_TEMP":         runnerTemp,
		"GITHUB_WORKSPACE":    repo,
		"GITHUB_OUTPUT":       filepath.Join(logs, "github-output"),
		"FAKE_GO_LOG":         filepath.Join(logs, "go-log"),
		"FAKE_GO_TEST_EXIT":   goExit,
		"FAKE_EVOLVE_ARGS":    filepath.Join(logs, "evolve-args"),
		"FAKE_EVOLVE_STDIN":   filepath.Join(logs, "evolve-stdin"),
		"FAKE_EVOLVE_OUT":     filepath.Join(logs, "evolve-out"),
		"GIT_TERMINAL_PROMPT": "0",
	}
	for k, v := range eventEnv(step, base, head) {
		env[k] = v
	}
	wd := filepath.Join(repo, filepath.FromSlash(step.WorkingDirectory))
	out, rc := runScript(t, wd, step.Run, env)
	repoReal, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	return readStepLogs(logs, stepOutcome{rc: rc, output: out, repoGo: filepath.Join(repoReal, "go")})
}

func readStepLogs(logs string, res stepOutcome) stepOutcome {
	res.evolveArgs = strings.TrimSpace(readOrEmpty(filepath.Join(logs, "evolve-args")))
	for _, field := range strings.Split(readOrEmpty(filepath.Join(logs, "evolve-stdin")), "\x00") {
		if field != "" {
			res.evolveStdin = append(res.evolveStdin, field)
		}
	}
	for _, line := range strings.Split(readOrEmpty(filepath.Join(logs, "go-log")), "\n") {
		pwd, args, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		call := goCall{pwd: pwd, args: strings.Fields(args)}
		if len(call.args) > 0 && call.args[0] == "test" {
			res.goTests = append(res.goTests, call)
		}
	}
	return res
}

func (c goCall) runs(pkg, re string) bool {
	if !slices.Contains(c.args, pkg) {
		return false
	}
	for i, a := range c.args {
		if a == "-run="+re || (a == "-run" && i+1 < len(c.args) && c.args[i+1] == re) {
			return true
		}
	}
	return false
}

const (
	replayRegexp = "^TestRemovalDesignAmp_TheR9RowLinksOnlyToAnExistingDesign$"
	packRegexp   = "^(TestHandoffVerbs_AreInTheRegistryTheUsageTextAndTheRuntimeReference|TestSignalsCodes_RepoDocIsInSyncWithTheLinkedRegistry)$"
	twoSelected  = "./cmd/evolve\t" + replayRegexp + "\n./internal/ciparity\t" + packRegexp + "\n"
)

func TestC2610_009_ABatchOfDocsPipesEveryChangedPathAndRunsOnlyTheSelectedTests(t *testing.T) {
	changed := []string{docReadByARegistryRow, docOfTheReplay}
	got := runDocsTestsStep(t, fakeBin{evolveOut: twoSelected}, changed...)
	if got.rc != 0 {
		t.Fatalf("RED: the step must pass when the selected tests pass, exit %d\n%s", got.rc, got.output)
	}
	if !strings.HasPrefix(got.evolveArgs, "ci docs-tests") {
		t.Errorf("RED: the step must call `evolve ci docs-tests`, called with %q", got.evolveArgs)
	}
	want := slices.Clone(changed)
	slices.Sort(want)
	gotPaths := slices.Clone(got.evolveStdin)
	slices.Sort(gotPaths)
	if !slices.Equal(gotPaths, want) {
		t.Errorf("RED: the step must pipe every changed path of the diff, NUL-separated, to the verb: got %q want %q", gotPaths, want)
	}
	if len(got.goTests) != 2 {
		t.Fatalf("RED: want one go test per selected package, got %d: %+v", len(got.goTests), got.goTests)
	}
	for _, call := range got.goTests {
		if call.pwd != got.repoGo {
			t.Errorf("RED: go test runs in %s, packages are relative to %s", call.pwd, got.repoGo)
		}
		if !slices.Contains(call.args, "-count=1") {
			t.Errorf("RED: go test must run with -count=1: %v", call.args)
		}
		if slices.Contains(call.args, "./...") {
			t.Errorf("RED: the docs-only job must not run the full Go suite: %v", call.args)
		}
	}
	if !slices.ContainsFunc(got.goTests, func(c goCall) bool { return c.runs("./cmd/evolve", replayRegexp) }) ||
		!slices.ContainsFunc(got.goTests, func(c goCall) bool { return c.runs("./internal/ciparity", packRegexp) }) {
		t.Errorf("RED: each selected package must run with its -run regexp: %+v", got.goTests)
	}
}

func TestC2610_010_AFailingSelectedTestFailsTheJobBeforeTheMerge(t *testing.T) {
	got := runDocsTestsStep(t, fakeBin{evolveOut: "./cmd/evolve\t" + replayRegexp + "\n", goTestExit: "1"}, docOfTheReplay)
	if got.rc == 0 {
		t.Errorf("RED: the replay of #1671 makes the selected test fail; the PR job must fail too\n%s", got.output)
	}
	if len(got.goTests) == 0 {
		t.Errorf("RED: the step never ran the selected test")
	}
}

func TestC2610_011_ADocNoGoTestReadsRunsNoGoTestAndPasses(t *testing.T) {
	got := runDocsTestsStep(t, fakeBin{evolveOut: ""}, "docs/guides/unread-doc.md")
	if got.rc != 0 {
		t.Errorf("RED: an empty selection is a pass, exit %d\n%s", got.rc, got.output)
	}
	if !strings.HasPrefix(got.evolveArgs, "ci docs-tests") {
		t.Errorf("RED: the step must ask the verb before it decides, called with %q", got.evolveArgs)
	}
	if len(got.goTests) != 0 {
		t.Errorf("RED: an unread doc selects no test, the job must run none: %+v", got.goTests)
	}
}

func requiredResultStep(t *testing.T) workflowStep {
	t.Helper()
	for _, s := range loadRequiredWorkflow(t).Jobs["required"].Steps {
		if s.ID == "result" {
			return s
		}
	}
	t.Fatal("the required job has no result step")
	return workflowStep{}
}

func docsOnlyResultEnv(docsTests string) map[string]string {
	return map[string]string{
		"EVENT_NAME": "pull_request", "ROUTE_RESULT": "success", "DOCS_ONLY": "true", "DOCS_REQUIRED": "true",
		"VALIDATE_RESULT": "skipped", "PLUGIN_RESULT": "", "ACS_RESULT": "", "STE_LINT_RESULT": "success",
		"TRAILER_CHECK_RESULT": "success", "GO_REQUIRED": "false", "GO_RESULT": "skipped", "GO_SUITE_RESULT": "",
		"LINT_RESULT": "skipped", "LANDING_REQUIRED": "false", "LANDING_RESULT": "skipped", "LANDING_SUITE_RESULT": "",
		"INBOX_REQUIRED": "false", "INBOX_RESULT": "skipped", "DOCS_TESTS_RESULT": docsTests,
	}
}

func codeResultEnv(docsTests string) map[string]string {
	env := docsOnlyResultEnv(docsTests)
	for k, v := range map[string]string{
		"DOCS_ONLY": "false", "DOCS_REQUIRED": "false", "VALIDATE_RESULT": "success", "PLUGIN_RESULT": "success",
		"ACS_RESULT": "success", "STE_LINT_RESULT": "skipped", "GO_REQUIRED": "true", "GO_RESULT": "success",
		"GO_SUITE_RESULT": "success", "LINT_RESULT": "success", "LANDING_REQUIRED": "true", "LANDING_RESULT": "success",
		"LANDING_SUITE_RESULT": "success",
	} {
		env[k] = v
	}
	return env
}

func TestC2610_012_TheRequiredResultTakesTheDocsTestsResultOnTheDocsOnlyRoute(t *testing.T) {
	script := requiredResultStep(t).Run
	type verdict struct {
		name string
		env  map[string]string
		pass bool
	}
	cases := []verdict{
		{"docs_only_docs_tests_success", docsOnlyResultEnv("success"), true},
		{"code_pr_docs_tests_skipped", codeResultEnv("skipped"), true},
	}
	for _, result := range []string{"failure", "cancelled", "skipped", "", "neutral"} {
		cases = append(cases, verdict{"docs_only_docs_tests_" + result, docsOnlyResultEnv(result), false})
	}
	for _, result := range []string{"failure", "cancelled", "success", "", "neutral"} {
		cases = append(cases, verdict{"code_pr_docs_tests_" + result, codeResultEnv(result), false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, rc := runScript(t, t.TempDir(), script, tc.env)
			if (rc == 0) != tc.pass {
				t.Errorf("RED: pass=%v want %v (exit %d)\n%s", rc == 0, tc.pass, rc, out)
			}
		})
	}
}

func TestC2610_013_TheRequiredJobWaitsForDocsTestsAndBindsItsResult(t *testing.T) {
	wf := loadRequiredWorkflow(t)
	required := wf.Jobs["required"]
	if !slices.Contains(required.needs(t), "docs-tests") {
		t.Errorf("RED: the CI required job must need docs-tests: %v", required.needs(t))
	}
	if got := requiredResultStep(t).Env["DOCS_TESTS_RESULT"]; got != "${{ needs.docs-tests.result }}" {
		t.Errorf("RED: DOCS_TESTS_RESULT must bind the real dependency result, got %q", got)
	}
}

func runPathsStep(t *testing.T, changed ...string) map[string]string {
	t.Helper()
	var paths workflowStep
	for _, s := range loadRequiredWorkflow(t).Jobs["changes"].Steps {
		if s.ID == "paths" {
			paths = s
		}
	}
	if paths.Run == "" {
		t.Fatal("the changes job has no paths step")
	}
	repo, base, head := docPR(t, changed...)
	out := filepath.Join(t.TempDir(), "github-output")
	if err := os.WriteFile(out, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	log, rc := runScript(t, repo, paths.Run, map[string]string{
		"EVENT_NAME": "pull_request", "BASE_SHA": base, "HEAD_SHA": head, "GITHUB_OUTPUT": out, "RUNNER_TEMP": t.TempDir(),
	})
	if rc != 0 {
		t.Fatalf("the paths step exit %d\n%s", rc, log)
	}
	got := map[string]string{}
	for _, line := range strings.Split(readOrEmpty(out), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			got[k] = v
		}
	}
	return got
}

func TestC2610_014_ADocsOnlyPullRequestStillSkipsTheFullGoSuiteAndStaysDocsOnly(t *testing.T) {
	cases := []struct {
		name    string
		changed []string
	}{
		{"a doc a go test reads", []string{docReadByARegistryRow}},
		{"the doc of the replay", []string{docOfTheReplay}},
		{"a doc no go test reads", []string{"docs/guides/unread-doc.md"}},
		{"a batch of docs", []string{docReadByARegistryRow, docOfTheReplay, "docs/guides/unread-doc.md"}},
	}
	for _, tc := range cases {
		got := runPathsStep(t, tc.changed...)
		if got["go"] != "false" || got["docs_only"] != "true" || got["landing"] != "false" {
			t.Errorf("%s: want go=false landing=false docs_only=true, got %v", tc.name, got)
		}
	}
	code := runPathsStep(t, "go/internal/x/x.go", docOfTheReplay)
	if code["go"] != "true" || code["docs_only"] != "false" {
		t.Errorf("a PR that also changes Go code must run the full Go suite, got %v", code)
	}
}

func TestC2610_015_TheNewPackageIsGraduatedAndDocumented(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforce := filepath.Join(root, "go", ".apicover-enforce")
	if !acsassert.FileMatchesRegex(t, enforce, `(?m)^\./internal/docstests$`) {
		t.Errorf("RED: go/.apicover-enforce must list ./internal/docstests")
	}
	for _, rel := range []string{
		"go/internal/docstests/apicover_named_test.go",
		"docs/architecture/packages/internal-docstests.md",
	} {
		if !acsassert.FileExists(t, filepath.Join(root, rel)) {
			t.Errorf("RED: %s missing", rel)
			continue
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
			t.Errorf("RED: %s is gitignored and would be dropped at ship (check-ignore exit %d)", rel, code)
		}
	}
}

// acs-predicate: config-check
func TestC2610_016_TheRuntimeReferenceDocumentsTheVerb(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileContains(t, filepath.Join(root, "docs", "operations", "runtime-reference.md"), "evolve ci docs-tests") {
		t.Errorf("RED: docs/operations/runtime-reference.md must document `evolve ci docs-tests`")
	}
}

func TestC2610_017_TheDocsTestsStepRunsOnAPushEventWithTheEnvTheWorkflowGivesIt(t *testing.T) {
	got := runDocsTestsStepOn(t, fakeBin{evolveOut: twoSelected}, workflowResolvedEnv(pushContext), docReadByARegistryRow, docOfTheReplay)
	if got.rc != 0 {
		t.Fatalf("RED: on a push event the docs-tests step must resolve its range from the event and exit 0, exit %d\n%s", got.rc, got.output)
	}
	if !slices.Contains(got.evolveStdin, docReadByARegistryRow) || !slices.Contains(got.evolveStdin, docOfTheReplay) {
		t.Errorf("RED: the push range must reach evolve ci docs-tests, got stdin %q", got.evolveStdin)
	}
	if len(got.goTests) != 2 {
		t.Errorf("RED: want 2 selected go test runs on a push, got %d: %v", len(got.goTests), got.goTests)
	}
}

func TestC2610_018_TheDocsTestsStepStillRunsOnAPullRequestWithTheEnvTheWorkflowGivesIt(t *testing.T) {
	got := runDocsTestsStepOn(t, fakeBin{evolveOut: twoSelected}, workflowResolvedEnv(pullRequestContext), docReadByARegistryRow, docOfTheReplay)
	if got.rc != 0 {
		t.Fatalf("RED: on a pull request the docs-tests step must exit 0 with the env the workflow gives, exit %d\n%s", got.rc, got.output)
	}
	if !slices.Contains(got.evolveStdin, docReadByARegistryRow) || !slices.Contains(got.evolveStdin, docOfTheReplay) {
		t.Errorf("RED: the PR range must reach evolve ci docs-tests, got stdin %q", got.evolveStdin)
	}
	if len(got.goTests) != 2 {
		t.Errorf("RED: want 2 selected go test runs on a pull request, got %d: %v", len(got.goTests), got.goTests)
	}
}

func TestC2610_019_AFailingSelectedTestFailsTheDocsTestsStepOnAPushEvent(t *testing.T) {
	got := runDocsTestsStepOn(t, fakeBin{evolveOut: twoSelected, goTestExit: "1"}, workflowResolvedEnv(pushContext), docOfTheReplay)
	if len(got.goTests) == 0 {
		t.Fatalf("RED: on a push the step must reach the selected go test before it can judge it, exit %d\n%s", got.rc, got.output)
	}
	if got.rc == 0 {
		t.Errorf("RED: a failing selected test must fail the step on a push, got exit 0")
	}
}

func TestC2610_020_AnEmptySelectionOnAPushEventRunsNoGoTestAndPasses(t *testing.T) {
	got := runDocsTestsStepOn(t, fakeBin{}, workflowResolvedEnv(pushContext), "docs/guides/unread-doc.md")
	if got.rc != 0 {
		t.Fatalf("RED: an empty selection on a push must exit 0, exit %d\n%s", got.rc, got.output)
	}
	if !slices.Contains(got.evolveStdin, "docs/guides/unread-doc.md") {
		t.Errorf("RED: the push range must still reach the selector, got stdin %q", got.evolveStdin)
	}
	if len(got.goTests) != 0 {
		t.Errorf("RED: an empty selection must run no go test, got %v", got.goTests)
	}
}
