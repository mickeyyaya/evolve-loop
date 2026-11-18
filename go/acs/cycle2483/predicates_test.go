//go:build acs

package cycle2483

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const evolveImportPath = "./cmd/evolve"

var (
	toolDir     string
	resultLine  = regexp.MustCompile(`(?m)^--- (PASS|FAIL): (\S+)`)
	latencyTest = []string{
		"TestBoundarySteps_RunTheT5AndT6LatencyChecksBetweenTheQualityReportAndGC",
		"TestBoundaryDispatch_RegistersTheLatencyCheckVerbs",
		"TestBoundaryRun_ABreachThatFilesAnItemDoesNotBlockTheBoundary",
		"TestBoundaryRun_AFailedLatencyCheckWarnsAndTheBoundaryContinuesToTheLaunch",
		"TestBoundaryRun_PassingLatencyChecksPrintNoWarning",
		"TestWaveNext_AFailedLatencyCheckWarnsAndTheWaveLaunches",
		"TestBoundaryRun_StopsAtFirstFailedStepWithItsExitCode",
	}
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "boundary-latency-c2483-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	toolDir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

type builtEvolve struct {
	path string
	err  error
}

var buildEvolve = sync.OnceValue(func() builtEvolve {
	modDir, err := acsassert.ModuleDir(".")
	if err != nil {
		return builtEvolve{err: err}
	}
	out := filepath.Join(toolDir, "evolve")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", modDir, "build", "-o", out, evolveImportPath)
	if err != nil || code != 0 {
		return builtEvolve{err: fmt.Errorf("go build exited %d: %v\n%s", code, err, acsassert.OutputTail(stdout+stderr, 30))}
	}
	return builtEvolve{path: out}
})

type packageRun struct {
	verdicts map[string]string
	tail     string
	err      error
}

var runLatencyTests = sync.OnceValue(func() packageRun {
	modDir, err := acsassert.ModuleDir(".")
	if err != nil {
		return packageRun{err: err}
	}
	pattern := "^(" + strings.Join(latencyTest, "|") + ")$"
	stdout, stderr, _, _ := acsassert.SubprocessOutput("go", "-C", modDir, "test", "-count=1", "-v", "./cmd/evolve", "-run", pattern)
	verdicts := map[string]string{}
	for _, m := range resultLine.FindAllStringSubmatch(stdout, -1) {
		verdicts[m[2]] = m[1]
	}
	return packageRun{verdicts: verdicts, tail: acsassert.OutputTail(stdout+stderr, 40)}
})

func requirePassing(t *testing.T, names ...string) {
	t.Helper()
	run := runLatencyTests()
	if run.err != nil {
		t.Fatalf("RED: %v", run.err)
	}
	for _, name := range names {
		if got := run.verdicts[name]; got != "PASS" {
			t.Errorf("RED: %s verdict = %q, want PASS\n%s", name, got, run.tail)
		}
	}
}

func boundaryPlan(t *testing.T) string {
	t.Helper()
	built := buildEvolve()
	if built.err != nil {
		t.Fatalf("RED: %v", built.err)
	}
	root := acsassert.SetupTempProject(t)
	goal := filepath.Join(t.TempDir(), "goal.txt")
	if err := os.WriteFile(goal, []byte("drain the console queue\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(built.path, "boundary", "run", "--dry-run", "--goal-text-file", goal, "--project-root", root)
	if err != nil || code != 0 {
		t.Fatalf("evolve boundary run --dry-run exited %d: %v\n%s", code, err, stdout+stderr)
	}
	return stdout
}

func TestC2483_001_BoundaryPlanRunsBothLatencyChecksBetweenQualityAndGC(t *testing.T) {
	plan := boundaryPlan(t)
	quality := strings.Index(plan, "evolve quality report")
	ci := strings.Index(plan, "evolve ci latency check")
	wave := strings.Index(plan, "evolve wave latency --check")
	gc := strings.Index(plan, "evolve gc ")
	if quality < 0 || ci < 0 || wave < 0 || gc < 0 {
		t.Fatalf("RED: plan lacks a step (quality %d, ci latency check %d, wave latency --check %d, gc %d)\n%s", quality, ci, wave, gc, plan)
	}
	if !(quality < ci && ci < wave && wave < gc) {
		t.Errorf("RED: order quality %d, ci %d, wave %d, gc %d; want the checks between the quality report and gc\n%s", quality, ci, wave, gc, plan)
	}
	requirePassing(t,
		"TestBoundarySteps_RunTheT5AndT6LatencyChecksBetweenTheQualityReportAndGC",
		"TestBoundaryDispatch_RegistersTheLatencyCheckVerbs")
}

func TestC2483_002_ABreachFilesAnItemAndDoesNotBlockTheBoundary(t *testing.T) {
	requirePassing(t,
		"TestBoundaryRun_ABreachThatFilesAnItemDoesNotBlockTheBoundary",
		"TestBoundaryRun_PassingLatencyChecksPrintNoWarning")
}

func TestC2483_003_AFailedCheckWarnsAndTheBoundaryContinues(t *testing.T) {
	requirePassing(t,
		"TestBoundaryRun_AFailedLatencyCheckWarnsAndTheBoundaryContinuesToTheLaunch",
		"TestWaveNext_AFailedLatencyCheckWarnsAndTheWaveLaunches")
}

func TestC2483_004_AFailedHardStepStillStopsTheBoundary(t *testing.T) {
	requirePassing(t, "TestBoundaryRun_StopsAtFirstFailedStepWithItsExitCode")
}
