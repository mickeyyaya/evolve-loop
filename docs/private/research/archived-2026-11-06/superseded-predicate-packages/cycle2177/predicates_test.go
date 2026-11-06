//go:build acs

package cycle2177

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const simulationFile = "go/internal/fleet/pool_simulation_test.go"

const poolFile = "go/internal/fleet/pool.go"

const admissionFile = "go/internal/admission/admission.go"

const admissionPackage = "github.com/mickeyyaya/evolve-loop/go/internal/admission"

const seedCount = 200

const runCeiling = 5 * time.Minute

const durationCeilingSeconds = 2.0

var passLine = regexp.MustCompile(`--- PASS: TestWaveSimulation \((\d+\.\d+)s\)`)

var summaryLine = regexp.MustCompile(`wave-simulation seeds=(\d+) graham=(\d+) serial=(\d+) arrivals=(\d+) checks=(\d+)`)

var seedInFailure = regexp.MustCompile(`(?i)seed[=: ]+\d+`)

type simRun struct {
	output   string
	exitCode int
}

type mutant struct {
	name string
	file string
	old  string
	new  string
}

var baselineOnce sync.Once

var baselineRun simRun

func moduleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runSimulation(t *testing.T, overlayPath string) simRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runCeiling)
	defer cancel()
	args := []string{"test", "-count=1", "-v", "-timeout", "120s", "-run", "^TestWaveSimulation$"}
	if overlayPath != "" {
		args = append(args, "-overlay", overlayPath)
	}
	args = append(args, "./internal/fleet")
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		}
	}
	return simRun{output: string(out), exitCode: code}
}

func baseline(t *testing.T) simRun {
	t.Helper()
	baselineOnce.Do(func() { baselineRun = runSimulation(t, "") })
	return baselineRun
}

func requireGreenBaseline(t *testing.T) {
	t.Helper()
	run := baseline(t)
	if run.exitCode != 0 || !passLine.MatchString(run.output) {
		t.Fatalf("RED: TestWaveSimulation does not pass on the unmutated pool (exit %d), so no mutant can be judged\n%s", run.exitCode, acsassert.OutputTail(run.output, 40))
	}
}

func mutatedRun(t *testing.T, m mutant) simRun {
	t.Helper()
	rel := m.file
	if rel == "" {
		rel = poolFile
	}
	pool := filepath.Join(acsassert.RepoRoot(t), rel)
	src, err := os.ReadFile(pool)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if got := strings.Count(string(src), m.old); got != 1 {
		t.Fatalf("mutant %q needs its anchor to occur exactly once in %s, found %d", m.name, rel, got)
	}
	dir := t.TempDir()
	mutated := filepath.Join(dir, "source_mutant.go")
	if err := os.WriteFile(mutated, []byte(strings.Replace(string(src), m.old, m.new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	body, err := json.Marshal(map[string]map[string]string{"Replace": {pool: mutated}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return runSimulation(t, overlay)
}

func requireKilled(t *testing.T, m mutant) {
	t.Helper()
	requireGreenBaseline(t)
	run := mutatedRun(t, m)
	if run.exitCode == 0 {
		t.Errorf("RED: the simulation still passes when the pool is broken (%s)", m.name)
		return
	}
	if strings.Contains(run.output, "panic: test timed out") {
		t.Errorf("RED: the simulation hangs instead of failing when the pool is broken (%s)\n%s", m.name, acsassert.OutputTail(run.output, 20))
		return
	}
	if !strings.Contains(run.output, "--- FAIL: TestWaveSimulation") {
		t.Errorf("RED: the broken pool (%s) did not fail TestWaveSimulation itself\n%s", m.name, acsassert.OutputTail(run.output, 30))
		return
	}
	if !seedInFailure.MatchString(run.output) {
		t.Errorf("RED: the failure for the broken pool (%s) does not print the seed\n%s", m.name, acsassert.OutputTail(run.output, 30))
	}
}

func summaryCounts(t *testing.T, output string) map[string]int {
	t.Helper()
	m := summaryLine.FindStringSubmatch(output)
	if m == nil {
		t.Fatalf("RED: no line shaped like %q in the -v output\n%s", "wave-simulation seeds=200 graham=<n> serial=<n> arrivals=<n> checks=<n>", acsassert.OutputTail(output, 40))
	}
	counts := map[string]int{}
	for i, key := range []string{"seeds", "graham", "serial", "arrivals", "checks"} {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			t.Fatalf("summary field %s: %v", key, err)
		}
		counts[key] = n
	}
	return counts
}

func summaryCountsWith(t *testing.T, output, extra string) map[string]int {
	t.Helper()
	counts := summaryCounts(t, output)
	m := regexp.MustCompile(`wave-simulation [^\n]*?` + extra).FindStringSubmatch(output)
	if m == nil {
		t.Fatalf("RED: the wave-simulation line has no field matching %q\n%s", extra, acsassert.OutputTail(output, 20))
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	counts["admission"] = n
	return counts
}

func TestC2177_001_SimulationPassesAndRunsBothBoundPopulations(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, simulationFile)) {
		t.Fatalf("RED: %s is missing", simulationFile)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", simulationFile); code == 0 {
		t.Errorf("RED: %s is gitignored and would be dropped at ship", simulationFile)
	}
	run := baseline(t)
	if run.exitCode != 0 {
		t.Fatalf("RED: go test -run ^TestWaveSimulation$ ./internal/fleet exited %d\n%s", run.exitCode, acsassert.OutputTail(run.output, 40))
	}
	if !passLine.MatchString(run.output) {
		t.Fatalf("RED: no \"--- PASS: TestWaveSimulation\" line: the test did not run or was skipped\n%s", acsassert.OutputTail(run.output, 40))
	}
	counts := summaryCounts(t, run.output)
	if counts["seeds"] != seedCount {
		t.Errorf("RED: the simulation ran %d seeds, want %d", counts["seeds"], seedCount)
	}
	if counts["graham"] == 0 {
		t.Errorf("RED: no seed was checked against the Graham bound (no conflicts, Window nil)")
	}
	if counts["serial"] == 0 {
		t.Errorf("RED: no seed was checked against the serial bound (conflicts)")
	}
	if counts["graham"]+counts["serial"] != counts["seeds"] {
		t.Errorf("RED: graham %d + serial %d must account for all %d seeds", counts["graham"], counts["serial"], counts["seeds"])
	}
	if counts["arrivals"] == 0 {
		t.Errorf("RED: no seed delivered a todo through Resupply after the start")
	}
	if counts["checks"] < counts["seeds"] {
		t.Errorf("RED: %d invariant checks over %d seeds: each seed needs at least one check at a settle terminal event", counts["checks"], counts["seeds"])
	}
}

func TestC2177_002_TwoHundredSeedsRunUnderTwoSeconds(t *testing.T) {
	requireGreenBaseline(t)
	best := 1e9
	for attempt := 0; attempt < 3; attempt++ {
		run := baseline(t)
		if attempt > 0 {
			run = runSimulation(t, "")
		}
		m := passLine.FindStringSubmatch(run.output)
		if m == nil {
			t.Fatalf("RED: no timed pass line in attempt %d\n%s", attempt, acsassert.OutputTail(run.output, 20))
		}
		secs, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		best = min(best, secs)
		if best < durationCeilingSeconds {
			return
		}
	}
	t.Errorf("RED: the fastest of 3 runs took %.2fs, want under %.1fs", best, durationCeilingSeconds)
}

func TestC2177_003_FailureCatchesAnIdleSlotAndPrintsTheSeed(t *testing.T) {
	requireKilled(t, mutant{
		name: "settle never backfills a freed slot",
		old:  "if full, _ := p.fill(PoolBackfilled, false); full {\n\t\treturn\n\t}\n\tif !p.resupply() {",
		new:  "if !p.resupply() {",
	})
}

func TestC2177_004_FailureCatchesLiveCountPastTheTarget(t *testing.T) {
	requireKilled(t, mutant{
		name: "fill admits up to width plus target lanes",
		old:  "for len(p.running) < width && len(p.pending) > 0 && p.open() {",
		new:  "for len(p.running) < width+p.cfg.Target && len(p.pending) > 0 && p.open() {",
	})
}

func TestC2177_005_FailureCatchesAPoolThatSerialisesTheWave(t *testing.T) {
	requireKilled(t, mutant{
		name: "width is one lane when Window is nil",
		old:  "if p.cfg.Window == nil {\n\t\treturn p.limit\n\t}",
		new:  "if p.cfg.Window == nil {\n\t\treturn 1\n\t}",
	})
}

func TestC2177_006_FailureCatchesATodoThatArrivesAfterTheStartAndNeverRuns(t *testing.T) {
	requireKilled(t, mutant{
		name: "settle never resupplies",
		old:  "if p.cfg.Resupply == nil || !p.open() {",
		new:  "if p.cfg.Resupply == nil || !p.open() || true {",
	})
}

func TestC2177_007_SimulationTestBinaryContainsTheRealAdmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), runCeiling)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-test", "./internal/fleet")
	cmd.Dir = moduleDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps -test ./internal/fleet: %v\n%s", err, acsassert.OutputTail(string(out), 20))
	}
	for _, line := range strings.Split(string(out), "\n") {
		importPath, _, _ := strings.Cut(strings.TrimSpace(line), " [")
		if importPath == admissionPackage {
			return
		}
	}
	t.Errorf("RED: the test binary of ./internal/fleet does not contain %s, so the simulation cannot run the real admission (an in-package test cannot import it: admission imports fleet, so the simulation must be package fleet_test)", admissionPackage)
}

func TestC2177_008_SimulationReportsBothSelectorPaths(t *testing.T) {
	requireGreenBaseline(t)
	counts := summaryCountsWith(t, baseline(t).output, `admission=(\d+)`)
	admitted := counts["admission"]
	if admitted <= 0 {
		t.Errorf("RED: no seed ran with PoolConfig.Next set from the real admission (admission=%d)", admitted)
	}
	if admitted >= seedCount {
		t.Errorf("RED: admission=%d of %d seeds: keep seeds with Next nil, so the pool's own selector stays covered", admitted, seedCount)
	}
}

func TestC2177_009_FailureCatchesAPoolThatIgnoresTheAdmission(t *testing.T) {
	requireKilled(t, mutant{
		name: "next ignores cfg.Next and returns no lane",
		old:  "return p.cfg.Next(p.todos(p.inFlight()), p.todos(p.pending))",
		new:  "return -1",
	})
}

func TestC2177_010_FailureCatchesAnAdmissionThatNeverReportsAConflict(t *testing.T) {
	requireKilled(t, mutant{
		name: "admission firstConflict never matches, so two lanes share a file",
		file: admissionFile,
		old:  "if rule(h.fp, fp) {",
		new:  "if false && rule(h.fp, fp) {",
	})
}

func TestC2177_011_FailureCatchesTheFileRuleOfThePoolSelector(t *testing.T) {
	requireKilled(t, mutant{
		name: "selectDisjoint skips the disjoint test",
		old:  "if !p.disjoint(idx) {\n\t\t\tcontinue\n\t\t}",
		new:  "if false {\n\t\t\tcontinue\n\t\t}",
	})
}
