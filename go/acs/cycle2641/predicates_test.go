//go:build acs

package cycle2641

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	missingCIStep   = "evolve ci latency"
	waveLatencyStep = "evolve wave latency --check"
)

var (
	evolveBin string
	buildErr  string
)

func buildEvolve(dir string) (bin, failure string) {
	mod, err := acsassert.ModuleDir(".")
	if err != nil {
		return "", err.Error()
	}
	bin = filepath.Join(dir, "evolve-under-test")
	cmd := sysexec.Command(context.Background(), "go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(mod, "cmd", "evolve")
	if out, berr := cmd.CombinedOutput(); berr != nil {
		return "", "go build: " + berr.Error() + "\n" + string(out)
	}
	return bin, ""
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs2641-")
	if err != nil {
		buildErr = err.Error()
	} else {
		evolveBin, buildErr = buildEvolve(dir)
	}
	code := m.Run()
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
	os.Exit(code)
}

type boundaryPlan struct {
	Data struct {
		Plan []string `json:"plan"`
	} `json:"data"`
}

func dryRunPlan(t *testing.T) []string {
	t.Helper()
	if evolveBin == "" {
		t.Fatalf("evolve binary did not build: %s", buildErr)
	}
	root := t.TempDir()
	goal := filepath.Join(root, "goal.txt")
	if err := os.WriteFile(goal, []byte("goal"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, stderr, code, err := acsassert.SubprocessOutput(evolveBin, "boundary", "run", "--dry-run", "--json", "--goal-text-file", goal, "--project-root", root)
	if err != nil || code != 0 {
		t.Fatalf("boundary run --dry-run exit=%d: %v\n%s", code, err, stderr)
	}
	var plan boundaryPlan
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("boundary run --dry-run --json is not JSON: %v\n%s", err, out)
	}
	if len(plan.Data.Plan) == 0 {
		t.Fatalf("boundary plan is empty\n%s", out)
	}
	return plan.Data.Plan
}

func stepsStartingWith(plan []string, prefix string) []string {
	var hits []string
	for _, step := range plan {
		if strings.HasPrefix(step, prefix) {
			hits = append(hits, step)
		}
	}
	return hits
}

func TestC2641_001_BoundaryPlanRunsNoMissingCILatencyVerb(t *testing.T) {
	plan := dryRunPlan(t)
	if hits := stepsStartingWith(plan, missingCIStep); len(hits) > 0 {
		t.Errorf("boundary plan still runs a verb that does not exist: %v", hits)
	}
}

func TestC2641_002_BoundaryPlanKeepsTheWaveLatencyCheckAdvisoryBeforeGC(t *testing.T) {
	plan := dryRunPlan(t)
	waveIdx, gcIdx, qualityIdx := -1, -1, -1
	for i, step := range plan {
		switch {
		case strings.HasPrefix(step, waveLatencyStep):
			waveIdx = i
		case strings.HasPrefix(step, "evolve gc "):
			gcIdx = i
		case strings.HasPrefix(step, "evolve quality report"):
			qualityIdx = i
		}
	}
	if waveIdx < 0 {
		t.Fatalf("boundary plan lacks %q: %v", waveLatencyStep, plan)
	}
	if !(qualityIdx < waveIdx && waveIdx < gcIdx) {
		t.Errorf("order: quality %d, wave latency %d, gc %d; want the check between the quality report and gc", qualityIdx, waveIdx, gcIdx)
	}
}

func TestC2641_003_EveryBoundaryStepVerbAndSubverbIsRegisteredAndPinnedByATest(t *testing.T) {
	mod := acsassert.GoModuleDir(t)
	out, stderr, code, err := acsassert.SubprocessOutputIn(mod, "go", "test", "-count=1", "-v", "-run", "^TestBoundarySteps_EachStepNamesARegisteredVerbAndSubverb$", "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go test exit=%d: %v\n%s%s", code, err, out, stderr)
	}
	if !strings.Contains(out, "--- PASS: TestBoundarySteps_EachStepNamesARegisteredVerbAndSubverb") {
		t.Errorf("the registered-verb pin did not run and pass\n%s", acsassert.OutputTail(out, 20))
	}
}
