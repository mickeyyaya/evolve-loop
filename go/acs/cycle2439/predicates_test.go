//go:build acs

package cycle2439

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mickeyyaya/evolve-loop/go/internal/mergewindow"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	redTest       = "TestPRMerge_RefusesAPRWithoutATrailerInEnforce"
	trailerVerb   = "inbox trailer-check"
	requiredYML   = ".github/workflows/required.yml"
	prFile        = "cmd_pr.go"
	trailerFile   = "cmd_inbox_trailercheck.go"
	coverRunNames = "^(TestPR|TestNewPRMerger|TestRunPR|TestParsePRMergeArgs|TestPlaneLiveRuns|TestCmd_InboxTrailerCheck|TestFloorRun_TierCommandsMatchRequiredCI$)"
	commandPkg    = "./cmd/evolve"
)

var baselineShortFunctions = []string{"parse", "parsePRMergeArgs", "ghIn", "planeLiveRuns", "failingWhat"}

func goCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

type cmdEvolveRun struct {
	out, coverFunc string
	err            error
}

var (
	cmdEvolveOnce   sync.Once
	cmdEvolveResult cmdEvolveRun
	scratchDir      string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs2439")
	if err != nil {
		os.Exit(2)
	}
	scratchDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func commandPackageRun(t *testing.T) cmdEvolveRun {
	t.Helper()
	cmdEvolveOnce.Do(func() {
		profile := filepath.Join(scratchDir, "cover.out")
		out, err := goCmd(t, "test", "-count=1", "-v", "-run", coverRunNames, "-coverprofile="+profile, commandPkg)
		cmdEvolveResult = cmdEvolveRun{out: out, err: err}
		if err == nil {
			cmdEvolveResult.coverFunc, cmdEvolveResult.err = goCmd(t, "tool", "cover", "-func="+profile)
		}
	})
	if cmdEvolveResult.err != nil {
		t.Fatalf("go test %s: %v\n%s", commandPkg, cmdEvolveResult.err, acsassert.OutputTail(cmdEvolveResult.out, 40))
	}
	return cmdEvolveResult
}

func requirePassed(t *testing.T, names ...string) {
	t.Helper()
	run := commandPackageRun(t)
	for _, name := range names {
		if !strings.Contains(run.out, "--- PASS: "+name) {
			t.Errorf("%s did not run and pass:\n%s", name, acsassert.OutputTail(run.out, 40))
		}
	}
}

func TestC2439_001_AnEnforceStageRefusesAPRWithoutATrailerAndMergesNothing(t *testing.T) {
	requirePassed(t, redTest)
}

func TestC2439_002_TheOtherStagesAndTrailerShapesBehave(t *testing.T) {
	requirePassed(t,
		"TestPRMerge_RefusesAnIdThatIsNotOpenInEnforce",
		"TestPRMerge_AcceptsADeclaredTrailerInEnforce",
		"TestPRMerge_ShadowStageWarnsAndStillMerges",
		"TestPRMerge_ReadsTheBodyAndCommitsOfThePR",
	)
}

func TestC2439_003_NewPRMergerCarriesThePolicyStageIntoTheMerger(t *testing.T) {
	requirePassed(t, "TestNewPRMerger_CarriesTheTrailerStageOfPolicy")
}

func TestC2439_004_TheVerbAndTheMergerStillBehaveAsBefore(t *testing.T) {
	requirePassed(t,
		"TestPRMerger_WaitsForGreenThenMergesAtTheVerifiedHead",
		"TestPRMerger_RefusesAndStops",
		"TestCmd_InboxTrailerCheck_AcceptsNone",
		"TestCmd_InboxTrailerCheck_RefusesARangeWithNoTrailer",
	)
}

func TestC2439_005_EveryFunctionOfTheTwoChangedFilesIsFullyCovered(t *testing.T) {
	seen := map[string]bool{}
	for _, line := range strings.Split(commandPackageRun(t).coverFunc, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		file := filepath.Base(strings.SplitN(fields[0], ":", 2)[0])
		if file != prFile && file != trailerFile {
			continue
		}
		seen[file] = true
		if slices.Contains(baselineShortFunctions, fields[1]) || fields[len(fields)-1] == "100.0%" {
			continue
		}
		t.Errorf("%s: %s is at %s, want 100.0%%", file, fields[1], fields[len(fields)-1])
	}
	if !seen[prFile] || !seen[trailerFile] {
		t.Fatalf("the cover output lists no function of %s and %s: %v", prFile, trailerFile, seen)
	}
}

type workflow struct {
	Jobs map[string]job `yaml:"jobs"`
}

type job struct {
	If    string   `yaml:"if"`
	Needs yamlList `yaml:"needs"`
	Steps []step   `yaml:"steps"`
}

type step struct {
	Run string            `yaml:"run"`
	Env map[string]string `yaml:"env"`
}

type yamlList []string

func (l *yamlList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*l = yamlList{n.Value}
		return nil
	}
	var many []string
	if err := n.Decode(&many); err != nil {
		return err
	}
	*l = many
	return nil
}

func loadWorkflow(t *testing.T) (workflow, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), requiredYML))
	if err != nil {
		t.Fatal(err)
	}
	var wf workflow
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("%s does not parse: %v", requiredYML, err)
	}
	return wf, raw
}

func trailerJobID(wf workflow) string {
	for id, j := range wf.Jobs {
		for _, s := range j.Steps {
			if strings.Contains(s.Run, trailerVerb) && strings.Contains(s.Run, "--range") {
				return id
			}
		}
	}
	return ""
}

func TestC2439_006_RequiredYMLRunsTheTrailerCheckOnEveryPullRequest(t *testing.T) {
	wf, _ := loadWorkflow(t)
	id := trailerJobID(wf)
	if id == "" {
		t.Fatalf("no job of %s runs `evolve %s --range`", requiredYML, trailerVerb)
	}
	j := wf.Jobs[id]
	if !strings.Contains(j.If, "github.event_name == 'pull_request'") {
		t.Errorf("job %s if = %q, want it limited to pull_request events", id, j.If)
	}
	for _, route := range []string{"needs.changes", "docs_only", "outputs.go", "outputs.docs"} {
		if strings.Contains(j.If, route) {
			t.Errorf("job %s if = %q reads the path route %q: a docs-only PR would skip the check", id, j.If, route)
		}
	}
	if slices.Contains(j.Needs, "changes") {
		t.Errorf("job %s needs the path router: it must run for every PR", id)
	}
}

func TestC2439_007_TheRequiredAggregatorWaitsOnTheTrailerCheck(t *testing.T) {
	wf, _ := loadWorkflow(t)
	id := trailerJobID(wf)
	if id == "" {
		t.Fatalf("no job of %s runs `evolve %s --range`", requiredYML, trailerVerb)
	}
	required, ok := wf.Jobs["required"]
	if !ok {
		t.Fatal("required.yml has no required job")
	}
	if !slices.Contains(required.Needs, id) {
		t.Errorf("required needs %v, want %s in it", required.Needs, id)
	}
	reads := false
	for _, s := range required.Steps {
		for name, v := range s.Env {
			if strings.Contains(v, "needs."+id+".result") && strings.Contains(s.Run, "$"+name) {
				reads = true
			}
		}
	}
	if !reads {
		t.Errorf("no required script tests the needs.%s.result variable: a red trailer check would not block the aggregate", id)
	}
}

func TestC2439_008_TheMergeWindowParserStillReadsRequiredYML(t *testing.T) {
	_, raw := loadWorkflow(t)
	if _, err := mergewindow.Parse(raw); err != nil {
		t.Fatalf("mergewindow.Parse(%s): %v", requiredYML, err)
	}
	requirePassed(t, "TestFloorRun_TierCommandsMatchRequiredCI")
}

func TestC2439_009_TheCIParityPackageStillPasses(t *testing.T) {
	if out, err := goCmd(t, "test", "-count=1", "./internal/ciparity"); err != nil {
		t.Fatalf("internal/ciparity: %v\n%s", err, acsassert.OutputTail(out, 30))
	}
}

var checkedFiles = regexp.MustCompile(`(\d+) file\(s\) checked`)

func TestC2439_010_TheSpecAndPackageNoteStateTheChangeInSTE(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, "docs", "architecture", "packages", "cmd-evolve.md")
	spec := filepath.Join(root, "docs", "architecture", "inbox-curator.md")
	for _, c := range []struct {
		path    string
		needles []string
	}{
		{note, []string{"evolve pr merge", "Closes-Inbox", "mergeOne"}},
		{spec, []string{"evolve pr merge", "required.yml", "Closes-Inbox"}},
	} {
		if !acsassert.LineContainsAll(c.path, c.needles[:2]...) {
			t.Errorf("%s: no line names %q together", c.path, c.needles[:2])
		}
		for _, n := range c.needles[2:] {
			if !acsassert.FileContains(t, c.path, n) {
				t.Errorf("%s: missing %q", c.path, n)
			}
		}
	}
	binary := filepath.Join(t.TempDir(), "evolve")
	if out, err := goCmd(t, "build", "-o", binary, commandPkg); err != nil {
		t.Fatalf("go build: %v\n%s", err, acsassert.OutputTail(out, 30))
	}
	base, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "refs/remotes/origin/main")
	if err != nil || code != 0 {
		t.Fatalf("git merge-base: code %d err %v", code, err)
	}
	out, errOut, code, err := acsassert.SubprocessOutput(binary, "docs", "ste-lint", "--strict", "--changed", strings.TrimSpace(base), "--added", "--project-root", root)
	if err != nil || code != 0 {
		t.Fatalf("ste-lint exit %d: %v\n%s%s", code, err, out, errOut)
	}
	m := checkedFiles.FindStringSubmatch(out + errOut)
	if m == nil {
		t.Fatalf("ste-lint printed no checked-file count: %q", out+errOut)
	}
	if n, _ := strconv.Atoi(m[1]); n < 2 {
		t.Errorf("ste-lint checked %d file(s), want the 2 changed docs checked", n)
	}
}
