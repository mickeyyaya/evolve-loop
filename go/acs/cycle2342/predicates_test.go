//go:build acs

package cycle2342

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	waveNumber       = 166
	waveFloor        = 2318
	firstCycle       = 2319
	lastCycle        = 2334
	journaledShips   = 11
	lateShippedCycle = 2333
	forgedCycle      = 2334
	startedAt        = "2026-11-12T00:32:46Z"
	endedAt          = "2026-11-12T02:54:17Z"
	lateCompletedAt  = "2026-11-12T03:10:00Z"
)

var (
	headlinePattern = regexp.MustCompile(`outcome: (\d+) of (\d+) cycles shipped`)
	costPattern     = regexp.MustCompile(`\((\d+) of (\d+) shipped\)`)
	goalPattern     = regexp.MustCompile(`Wave 166 facts \(run [^)]*\): (\d+) of (\d+) cycles shipped`)
)

var evolveBin string

var buildErr string

func TestMain(m *testing.M) {
	os.Exit(runWithBinary(m))
}

func runWithBinary(m *testing.M) int {
	dir, err := os.MkdirTemp("", "acs-cycle2342-bin-")
	if err != nil {
		buildErr = fmt.Sprintf("mktemp for evolve build: %v", err)
		return m.Run()
	}
	defer os.RemoveAll(dir)
	wd, err := os.Getwd()
	if err != nil {
		buildErr = fmt.Sprintf("getwd: %v", err)
		return m.Run()
	}
	goMod, err := acsassert.ModuleDir(wd)
	if err != nil {
		buildErr = err.Error()
		return m.Run()
	}
	bin := filepath.Join(dir, "evolve-under-test")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = goMod
	if out, berr := cmd.CombinedOutput(); berr != nil {
		buildErr = fmt.Sprintf("go build ./cmd/evolve (dir=%s): %v\n%s", goMod, berr, out)
	} else {
		evolveBin = bin
	}
	return m.Run()
}

type planeFiles map[string]string

func (files planeFiles) materialize(t *testing.T, root string) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func landingIntent(cycle int, sha, completedAt string) string {
	return fmt.Sprintf(`{"cycle":%d,"commit_sha":"%s","status":"complete","completed_at":"%s"}`, cycle, sha, completedAt)
}

func journalLine(cycle int, sha, ts string) string {
	return fmt.Sprintf(`{"sha":"%s","class":"cycle","ts":"%s","cycle":%d}`, sha, ts, cycle) + "\n"
}

func wave166Plane(t *testing.T, withForgedFlag bool) string {
	t.Helper()
	root := t.TempDir()
	files := planeFiles{
		".evolve/wave-goal.md": "Standing goal of the fixture.\n",
		".evolve/state.json":   fmt.Sprintf(`{"lastCycleNumber":%d}`, lastCycle),
		fmt.Sprintf(".evolve/waves/wave-%d.json", waveNumber): fmt.Sprintf(
			`{"number":%d,"run_id":"20261111T162732Z","cycle_floor":%d,"started_at":"%s","ended_at":"%s"}`,
			waveNumber, waveFloor, startedAt, endedAt),
	}
	journal := ""
	for c := firstCycle; c <= lastCycle; c++ {
		files[fmt.Sprintf(".evolve/runs/cycle-%d/run.json", c)] = fmt.Sprintf(`{"cycle_id":%d,"phase":"ship"}`, c)
		landing := fmt.Sprintf(".evolve/landing/cycle-%d.json", c)
		switch {
		case c < firstCycle+journaledShips:
			sha := fmt.Sprintf("sha%d", c)
			ts := fmt.Sprintf("2026-11-12T01:%02d:00Z", c-firstCycle+10)
			journal += journalLine(c, sha, ts)
			files[landing] = landingIntent(c, sha, ts)
		case c == lateShippedCycle:
			files[landing] = landingIntent(c, fmt.Sprintf("sha%d", c), lateCompletedAt)
		case c == forgedCycle && withForgedFlag:
			files[fmt.Sprintf(".evolve/runs/cycle-%d/run.json", c)] = fmt.Sprintf(`{"cycle_id":%d,"phase":"build","shipped":true}`, c)
		}
	}
	files[".evolve/ship-journal.jsonl"] = journal
	files.materialize(t, root)
	return root
}

func runEvolve(t *testing.T, args ...string) string {
	t.Helper()
	if evolveBin == "" {
		t.Fatalf("evolve binary was not built: %s", buildErr)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBin, args...)
	if err != nil || code != 0 {
		t.Fatalf("evolve %v exit=%d err=%v\nstdout:\n%s\nstderr:\n%s", args, code, err, stdout, stderr)
	}
	return stdout
}

func shippedCount(t *testing.T, label string, pattern *regexp.Regexp, text string) (shipped, started int) {
	t.Helper()
	m := pattern.FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("%s: no match for %q in:\n%s", label, pattern, text)
	}
	shipped, _ = strconv.Atoi(m[1])
	started, _ = strconv.Atoi(m[2])
	return shipped, started
}

type threeCounts struct {
	headline, cost, goal int
	cycles               [3]int
}

func countsOf(t *testing.T, root string) threeCounts {
	t.Helper()
	status := runEvolve(t, "wave", "status", "--project-root", root)
	goal := runEvolve(t, "wave", "next", "--dry-run", "--project-root", root)
	var c threeCounts
	c.headline, c.cycles[0] = shippedCount(t, "status headline", headlinePattern, status)
	c.cost, c.cycles[1] = shippedCount(t, "status cost line", costPattern, status)
	c.goal, c.cycles[2] = shippedCount(t, "next wave goal", goalPattern, goal)
	return c
}

func assertAllCounts(t *testing.T, got threeCounts, want int) {
	t.Helper()
	if got.headline != want || got.cost != want || got.goal != want {
		t.Errorf("shipped counts: headline %d, cost line %d, next wave goal %d; want %d in all three", got.headline, got.cost, got.goal, want)
	}
	total := lastCycle - firstCycle + 1
	for i, n := range got.cycles {
		if n != total {
			t.Errorf("source %d counts %d cycles, want %d", i, n, total)
		}
	}
}

func TestC2342_001_ALateShippedCycleGivesTheSameCountInHeadlineCostLineAndNextWaveGoal(t *testing.T) {
	root := wave166Plane(t, false)

	assertAllCounts(t, countsOf(t, root), journaledShips+1)
}

func TestC2342_002_AForgedShippedFlagWithNoLandingCountsInNoneOfTheThreeCounts(t *testing.T) {
	root := wave166Plane(t, true)

	assertAllCounts(t, countsOf(t, root), journaledShips+1)
}
