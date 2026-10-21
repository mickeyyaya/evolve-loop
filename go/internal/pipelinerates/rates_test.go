package pipelinerates

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func loaded(t *testing.T, root string) []Cycle {
	t.Helper()
	cycles, err := LoadCycles(root)
	if err != nil {
		t.Fatalf("LoadCycles: %v", err)
	}
	return cycles
}

func TestShipRate_WavesOneThirtySevenToOneFortyMatchTheResearch(t *testing.T) {
	t.Parallel()
	cycles := loaded(t, wavesFixture(t))
	want := map[int][2]int{137: {2, 5}, 138: {1, 6}, 139: {2, 5}, 140: {3, 5}}
	for wave, kn := range want {
		k, n := RateIn(cycles, Window{From: wave, To: wave}, Shipped)
		if k != kn[0] || n != kn[1] {
			t.Errorf("wave %d: %d/%d, want %d/%d", wave, k, n, kn[0], kn[1])
		}
	}
	bounds := map[int][2]float64{137: {0.1176, 0.7693}, 138: {0.0301, 0.5635}, 139: {0.1176, 0.7693}, 140: {0.2307, 0.8824}}
	for wave, b := range bounds {
		k, n := RateIn(cycles, Window{From: wave, To: wave}, Shipped)
		if iv := qualitypanel.Wilson(k, n); math.Abs(iv.Lo-b[0]) > 0.0005 || math.Abs(iv.Hi-b[1]) > 0.0005 {
			t.Errorf("wave %d Wilson = %+v, want [%.4f, %.4f]", wave, iv, b[0], b[1])
		}
	}
	readings := ShipRate(cycles)
	if len(readings) != 2 || readings[0].Window != "wave:140" || readings[1].Window != "waves:137-140" {
		t.Fatalf("readings = %+v, want wave:140 and waves:137-140", readings)
	}
	if got := readings[0].Interval; readings[0].N != 5 || math.Abs(got.Lo-0.2307) > 0.0005 || math.Abs(got.Hi-0.8824) > 0.0005 {
		t.Errorf("wave 140 = %+v n %d, want 3/5 [0.231, 0.882]", got, readings[0].N)
	}
	if r := readings[1]; r.N != 21 || math.Abs(r.Interval.Value-8.0/21) > 1e-9 || r.Index != "PQ1" || r.Source != ".evolve/runs" {
		t.Errorf("rolling = %+v, want PQ1 8/21 from .evolve/runs", r)
	}
}

func TestShipRate_RunJSONShippedCountsWithoutTheSignal(t *testing.T) {
	t.Parallel()
	root := wavesFixture(t)
	writeCycle(t, root, fixtureCycle{id: 2013, runShipped: true, seal: "final verdict PASS"})

	if k, n := RateIn(loaded(t, root), Window{From: 138, To: 138}, Shipped); k != 2 || n != 6 {
		t.Errorf("wave 138 = %d/%d, want 2/6: run.json shipped is a ship from before ship.landed existed", k, n)
	}
}

func TestDeadlineRate_AnUnsealedRunAndALaneDeadlineSealAreDeadlineEnds(t *testing.T) {
	t.Parallel()
	cycles := loaded(t, wavesFixture(t))

	readings := DeadlineRate(cycles)

	if c := cycleByID(cycles, 2010); c.Sealed || !c.DeadlineEnd() {
		t.Errorf("cycle 2010 = %+v, want unsealed and a deadline end", c)
	}
	if c := cycleByID(cycles, 2012); !c.LaneDeadline || !c.DeadlineEnd() {
		t.Errorf("cycle 2012 = %+v, want a lane-deadline seal", c)
	}
	if c := cycleByID(cycles, 2015); !c.SystemFail || c.DeadlineEnd() {
		t.Errorf("cycle 2015 = %+v, want a system fail that is not a deadline end", c)
	}
	if len(readings) != 4 || readings[1].N != 21 || math.Abs(readings[1].Interval.Value-2.0/21) > 1e-9 || readings[1].Threshold != nil {
		t.Errorf("deadline rolling = %+v, want 2/21 with the registry threshold", readings[1])
	}
	sf := readings[3]
	if sf.Window != "waves:137-140/system-fail" || math.Abs(sf.Interval.Value-1.0/21) > 1e-9 || sf.Threshold == nil || *sf.Threshold != 0.05 {
		t.Errorf("system-fail = %+v, want 1/21 against its own 0.05", sf)
	}
}

func TestPackagesPerShip_MedianOfGoPackagesByWave(t *testing.T) {
	t.Parallel()
	cycles := loaded(t, wavesFixture(t))
	show := func(commit string) ([]string, error) {
		if commit == "sha2028" {
			return []string{"go/a/x.go", "go/b/y.go", "go/c/z_test.go", "docs/n.md", ".evolve/inbox/i.go"}, nil
		}
		return []string{"go/a/x.go", "go/a/y.go"}, nil
	}

	readings, err := PackagesPerShip(cycles, show)

	if err != nil || len(readings) != 2 {
		t.Fatalf("readings %+v, %v", readings, err)
	}
	if r := readings[1]; r.Index != "CQ2" || r.N != 8 || r.Interval.Value != 1 || r.Method != qualitypanel.MethodBootstrap95 {
		t.Errorf("rolling = %+v, want CQ2 median 1 over 8 ships", r)
	}
	if r := readings[0]; r.N != 3 || r.Interval.Value != 1 {
		t.Errorf("wave 140 = %+v, want median of {1,1,3} = 1 over 3 ships", r)
	}
	if _, err := PackagesPerShip(cycles, func(string) ([]string, error) { return nil, errors.New("bad object") }); err == nil {
		t.Error("a failed git show must fail the reader")
	}
}

func TestGitShipPaths_ListsThePathsOfACommit(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("go/internal/a/a.go", "package a\n")
	repo.Write("docs/b.md", "b\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "c")
	sha := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))

	paths, err := GitShipPaths(repo.Dir)(sha)

	if err != nil || strings.Join(paths, ",") != "docs/b.md,go/internal/a/a.go" || PackageCount(paths) != 1 {
		t.Errorf("paths %v, %v; want the two files and one Go package", paths, err)
	}
	if _, err := GitShipPaths(repo.Dir)("0000000000000000000000000000000000000000"); err == nil {
		t.Error("an unknown commit must be an error")
	}
}

func TestLoadCycles_SourceErrors(t *testing.T) {
	t.Parallel()
	if _, err := LoadCycles(t.TempDir()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("no waves: %v, want a not-exist error", err)
	}
	noRuns := t.TempDir()
	writeWave(t, noRuns, 1, 1)
	if _, err := LoadCycles(noRuns); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("no runs: %v, want a not-exist error", err)
	}
	badWave := wavesFixture(t)
	if err := os.WriteFile(filepath.Join(badWave, ".evolve", "waves", "wave-141.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCycles(badWave); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Errorf("a bad wave record: %v, want a parse error", err)
	}
	badRun := wavesFixture(t)
	if err := os.WriteFile(filepath.Join(badRun, ".evolve", "runs", "cycle-2009", "run.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCycles(badRun); err == nil {
		t.Error("a bad run.json must fail the load")
	}
	badStream := wavesFixture(t)
	stream := filepath.Join(badStream, ".evolve", "runs", "cycle-2010", "signals.ndjson")
	if err := os.Remove(stream); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stream, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCycles(badStream); err == nil {
		t.Error("an unreadable signal stream must fail the load")
	}
}

func TestLoadCycles_SkipsCyclesBeforeTheFirstWaveAndOtherEntries(t *testing.T) {
	t.Parallel()
	root := wavesFixture(t)
	writeCycle(t, root, fixtureCycle{id: 1999, item: "early"})
	writeJSON(t, filepath.Join(root, ".evolve", "waves", "wave-0.json"), map[string]int{"number": 0, "cycle_floor": 1})
	writeJSON(t, filepath.Join(root, ".evolve", "runs", "cycle-x", "run.json"), map[string]bool{"shipped": true})
	writeJSON(t, filepath.Join(root, ".evolve", "runs", "notes.json"), map[string]bool{})

	cycles := loaded(t, root)

	if len(cycles) != 21 || cycles[0].ID != 2008 {
		t.Errorf("cycles = %d starting at %d, want the 21 cycles of waves 137 to 140", len(cycles), cycles[0].ID)
	}
	if Windows(nil) != nil {
		t.Error("no cycles must give no window")
	}
}

func TestWindows_AddTheAllWavesWindowOnlyBeyondTheRollingTen(t *testing.T) {
	t.Parallel()
	cycles := []Cycle{{Wave: 100}, {Wave: 140}}
	got := Windows(cycles)
	if len(got) != 3 || got[1].Name != "waves:131-140" || got[2].Name != "waves:100-140" {
		t.Errorf("windows = %+v", got)
	}
	if one := Windows([]Cycle{{Wave: 7}}); len(one) != 1 || one[0].Name != "wave:7" {
		t.Errorf("one wave = %+v", one)
	}
}

func TestGitShipPaths_AMergeShowsItsFirstParentDiffAndARenameItsNewPath(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("go/old/a.go", "package old\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	repo.Git("checkout", "-q", "-b", "side")
	repo.Write("go/new dir/a.go", "package old\n")
	repo.Git("rm", "-q", "go/old/a.go")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "rename")
	repo.Git("checkout", "-q", "-")
	repo.Git("merge", "-q", "--no-ff", "-m", "merge", "side")
	sha := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))

	paths, err := GitShipPaths(repo.Dir)(sha)

	if err != nil || strings.Join(paths, ",") != "go/new dir/a.go,go/old/a.go" || PackageCount(paths) != 2 {
		t.Errorf("paths %q, %v; want both sides of the rename against the first parent", paths, err)
	}
}

func TestPackagesPerShip_AnUnresolvableCommitIsReportedAndTheRestStillRead(t *testing.T) {
	t.Parallel()
	cycles := []Cycle{{ID: 1, Wave: 1, Shipped: true, Commit: "good"}, {ID: 2, Wave: 1, Shipped: true, Commit: "gone"}}
	show := func(c string) ([]string, error) {
		if c == "gone" {
			return nil, errors.New("bad object gone")
		}
		return []string{"go/a/x.go"}, nil
	}

	readings, err := PackagesPerShip(cycles, show)

	if err == nil || !strings.Contains(err.Error(), "gone") || len(readings) != 1 || readings[0].N != 1 {
		t.Errorf("readings %+v err %v, want one reading over the good ship and an error naming the bad one", readings, err)
	}
}
