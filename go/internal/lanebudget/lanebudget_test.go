package lanebudget

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func inLane(t *testing.T, cpus int, width string) {
	t.Helper()
	prev := hostCPUs
	hostCPUs = func() int { return cpus }
	t.Cleanup(func() { hostCPUs = prev })
	t.Setenv(ipcenv.FleetWidthKey, width)
	t.Setenv(ipcenv.FleetLiveWidthKey, "")
}

func TestProcs_DividesTheHostAmongTheLanes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cpus, width, want int
	}{
		{14, 4, 3},
		{14, 1, 14},
		{2, 8, 1},
		{14, 0, 0},
		{14, -2, 0},
	}
	for _, c := range cases {
		if got := Procs(c.cpus, c.width); got != c.want {
			t.Errorf("Procs(%d, %d) = %d, want %d", c.cpus, c.width, got, c.want)
		}
	}
}

func TestCurrent_ReadsTheFleetWidthOfThisProcess(t *testing.T) {
	cases := []struct {
		width string
		want  int
	}{
		{"4", 3},
		{" 2 ", 7},
		{"", 0},
		{"wide", 0},
		{"0", 0},
	}
	for _, c := range cases {
		inLane(t, 14, c.width)
		if got := Current(); got != c.want {
			t.Errorf("Current() with width %q = %d, want %d", c.width, got, c.want)
		}
	}
}

func TestCurrent_TheRealHostCountIsTheDefault(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, "1")
	if got := Current(); got < 1 {
		t.Errorf("Current() at width 1 = %d, want the host CPU count", got)
	}
}

func TestArgs_ALaneGoTestGetsTheBudgetAfterTest(t *testing.T) {
	inLane(t, 14, "4")
	got := Args([]string{"test", "-json", "-count=1", "./internal/core"})
	want := []string{"test", "-p", "3", "-parallel", "3", "-json", "-count=1", "./internal/core"}
	if !slices.Equal(got, want) {
		t.Errorf("Args = %q, want %q", got, want)
	}
}

func TestArgs_AConsoleRunIsUnchanged(t *testing.T) {
	inLane(t, 14, "")
	in := []string{"test", "-count=1", "./..."}
	if got := Args(in); !slices.Equal(got, in) {
		t.Errorf("Args = %q, want %q unchanged", got, in)
	}
}

func TestArgs_OnlyAGoTestArgvChanges(t *testing.T) {
	inLane(t, 14, "4")
	for _, in := range [][]string{{"vet", "./..."}, {"tool", "cover", "-func=x"}, {}, nil} {
		if got := Args(in); !slices.Equal(got, in) {
			t.Errorf("Args(%q) = %q, want it unchanged", in, got)
		}
	}
}

func TestArgs_AFlagTheCallerSetIsKept(t *testing.T) {
	inLane(t, 14, "4")
	got := Args([]string{"test", "-p", "2", "-parallel=5", "./x"})
	want := []string{"test", "-p", "2", "-parallel=5", "./x"}
	if !slices.Equal(got, want) {
		t.Errorf("Args = %q, want %q", got, want)
	}
}

func TestArgs_NeverWritesIntoTheCallersArray(t *testing.T) {
	inLane(t, 14, "4")
	in := make([]string, 2, 8)
	copy(in, []string{"test", "./x"})
	_ = Args(in)
	if got := in[:cap(in)][2]; got != "" {
		t.Errorf("Args wrote %q past the caller's slice", got)
	}
}

func TestEnv_ALaneChildGetsGOMAXPROCS(t *testing.T) {
	inLane(t, 14, "4")
	in := []string{"PATH=/bin"}
	got := Env(in)
	if !slices.Equal(got, []string{"PATH=/bin", "GOMAXPROCS=3"}) {
		t.Errorf("Env = %q, want PATH and GOMAXPROCS=3", got)
	}
	if len(in) != 1 {
		t.Errorf("Env changed the caller's env to %q", in)
	}
}

func TestEnv_ANilEnvInheritsTheProcessEnvironment(t *testing.T) {
	inLane(t, 14, "4")
	got := Env(nil)
	if !slices.Contains(got, ipcenv.FleetWidthKey+"=4") || got[len(got)-1] != "GOMAXPROCS=3" {
		t.Errorf("Env(nil) = %q, want the process environment and then GOMAXPROCS=3", got)
	}
}

func TestEnv_AConsoleRunIsUnchanged(t *testing.T) {
	inLane(t, 14, "")
	if got := Env(nil); got != nil {
		t.Errorf("Env(nil) = %q, want nil so the child inherits", got)
	}
	in := []string{"PATH=/bin"}
	if got := Env(in); !slices.Equal(got, in) {
		t.Errorf("Env = %q, want %q unchanged", got, in)
	}
}

type call struct {
	name       string
	args, env  []string
	stdoutSeen bool
}

func recorder(calls *[]call) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(_ context.Context, name, _ string, args, env []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		*calls = append(*calls, call{name: name, args: args, env: env, stdoutSeen: stdout != nil})
		return 7, nil
	}
}

func TestRunner_BudgetsEachGoTestAndPassesTheRestThrough(t *testing.T) {
	inLane(t, 14, "4")
	var calls []call
	run := Runner(recorder(&calls))

	code, err := run(context.Background(), "go", "/m", []string{"test", "./x"}, []string{"PATH=/bin"}, nil, io.Discard, nil)
	_, _ = run(context.Background(), "go", "/m", []string{"list", "./x"}, []string{"PATH=/bin"}, nil, nil, nil)
	_, _ = run(context.Background(), "git", "/m", []string{"test"}, nil, nil, nil, nil)

	if code != 7 || err != nil || len(calls) != 3 {
		t.Fatalf("Runner = %d, %v with %d calls, want the inner result and 3 calls", code, err, len(calls))
	}
	if want := []string{"test", "-p", "3", "-parallel", "3", "./x"}; !slices.Equal(calls[0].args, want) || !slices.Equal(calls[0].env, []string{"PATH=/bin", "GOMAXPROCS=3"}) || !calls[0].stdoutSeen {
		t.Errorf("go test call = %+v, want args %q and GOMAXPROCS=3", calls[0], want)
	}
	if !slices.Equal(calls[1].args, []string{"list", "./x"}) || !slices.Equal(calls[1].env, []string{"PATH=/bin"}) {
		t.Errorf("go list call = %+v, want it unchanged", calls[1])
	}
	if calls[2].env != nil || !slices.Equal(calls[2].args, []string{"test"}) {
		t.Errorf("git call = %+v, want it unchanged", calls[2])
	}
}

func liveFile(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), LiveWidthFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ipcenv.FleetLiveWidthKey, path)
}

func aliveAs(t *testing.T, alive bool, pid int) {
	t.Helper()
	prev := processAlive
	processAlive = func(p int) bool { return alive && p == pid }
	t.Cleanup(func() { processAlive = prev })
}

func TestCurrent_ALoneLaneGetsTheHostCPUs(t *testing.T) {
	inLane(t, 14, "8")
	aliveAs(t, true, 4242)
	liveFile(t, `{"pid":4242,"width":1}`)
	if got := Current(); got != 14 {
		t.Errorf("Current() with 1 live lane under a pool limit of 8 = %d, want all 14 CPUs", got)
	}
}

func TestCurrent_ReadsTheLiveWidthNotThePoolLimit(t *testing.T) {
	inLane(t, 14, "8")
	aliveAs(t, true, 4242)
	liveFile(t, `{"pid":4242,"width":3}`)
	if got := Current(); got != 4 {
		t.Errorf("Current() with 3 live lanes under a pool limit of 8 = %d, want 4", got)
	}
}

func TestCurrent_ADeadPoolFileFallsBackToTheSpawnWidth(t *testing.T) {
	inLane(t, 14, "8")
	aliveAs(t, false, 4242)
	liveFile(t, `{"pid":4242,"width":1}`)
	if got := Current(); got != 1 {
		t.Errorf("Current() with a dead pool pid = %d, want the spawn width share 1", got)
	}
}

func TestCurrent_AnUnusableLiveFileFallsBackToTheSpawnWidth(t *testing.T) {
	for name, body := range map[string]string{
		"garbage":    `not json`,
		"zero width": `{"pid":4242,"width":0}`,
	} {
		inLane(t, 14, "8")
		aliveAs(t, true, 4242)
		liveFile(t, body)
		if got := Current(); got != 1 {
			t.Errorf("%s: Current() = %d, want the spawn width share 1", name, got)
		}
	}
	inLane(t, 14, "8")
	aliveAs(t, true, 4242)
	t.Setenv(ipcenv.FleetLiveWidthKey, filepath.Join(t.TempDir(), "absent.json"))
	if got := Current(); got != 1 {
		t.Errorf("a missing file: Current() = %d, want 1", got)
	}
}

func TestCurrent_ALiveFileWithNoSpawnWidthStaysAConsoleRun(t *testing.T) {
	inLane(t, 14, "")
	aliveAs(t, true, 4242)
	liveFile(t, `{"pid":4242,"width":2}`)
	if got := Current(); got != 0 {
		t.Errorf("Current() = %d, want 0: a run with no spawn width stays a console run", got)
	}
}

func TestWriteLiveWidth_RoundTripsThroughCurrent(t *testing.T) {
	inLane(t, 14, "8")
	aliveAs(t, true, os.Getpid())
	path := filepath.Join(t.TempDir(), LiveWidthFile)
	t.Setenv(ipcenv.FleetLiveWidthKey, path)
	if err := WriteLiveWidth(path, os.Getpid(), 2); err != nil {
		t.Fatal(err)
	}
	if got := Current(); got != 7 {
		t.Errorf("Current() after WriteLiveWidth(2) = %d, want 7", got)
	}
	if err := WriteLiveWidth(path, os.Getpid(), 0); err != nil {
		t.Fatal(err)
	}
	if got := Current(); got != 14 {
		t.Errorf("a written width of 0 is stored as 1: Current() = %d, want 14", got)
	}
}

func TestWriteLiveWidth_AnUnwritablePathIsAnError(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteLiveWidth(filepath.Join(blocker, LiveWidthFile), 1, 1); err == nil {
		t.Error("WriteLiveWidth below a regular file returned nil")
	}
}

func TestProcessAlive_IsTheRunLeaseCheck(t *testing.T) {
	if !processAlive(os.Getpid()) || processAlive(0) {
		t.Error("the default liveness check must see this process and reject pid 0")
	}
}

func TestArgsFor_AGoTestGetsTheGivenShareNotTheLaneShare(t *testing.T) {
	inLane(t, 14, "1")
	got := ArgsFor(2, []string{"test", "./x"})
	if !slices.Equal(got, []string{"test", "-p", "2", "-parallel", "2", "./x"}) {
		t.Errorf("ArgsFor(2) = %q, want -p 2 -parallel 2 and not the lane share of 14", got)
	}
}

func TestArgsFor_AZeroShareLeavesTheArgvAlone(t *testing.T) {
	inLane(t, 14, "1")
	in := []string{"test", "./x"}
	if got := ArgsFor(0, in); !slices.Equal(got, in) {
		t.Errorf("ArgsFor(0) = %q, want %q unchanged", got, in)
	}
}

func TestArgsFor_OnlyAGoTestArgvChanges(t *testing.T) {
	if got := ArgsFor(3, []string{"list", "./x"}); !slices.Equal(got, []string{"list", "./x"}) {
		t.Errorf("ArgsFor on go list = %q, want it unchanged", got)
	}
}

func TestEnvFor_AChildGetsGOMAXPROCSOfTheGivenShare(t *testing.T) {
	inLane(t, 14, "1")
	in := []string{"PATH=/bin"}
	got := EnvFor(2, in)
	if !slices.Equal(got, []string{"PATH=/bin", "GOMAXPROCS=2"}) {
		t.Errorf("EnvFor(2) = %q, want GOMAXPROCS=2 and not the lane share of 14", got)
	}
	if len(in) != 1 {
		t.Errorf("EnvFor changed the caller's env to %q", in)
	}
}

func TestEnvFor_AZeroShareLeavesTheEnvAlone(t *testing.T) {
	if got := EnvFor(0, nil); got != nil {
		t.Errorf("EnvFor(0, nil) = %q, want nil so the child inherits", got)
	}
}

func TestEnvFor_ANilEnvInheritsTheProcessEnvironment(t *testing.T) {
	t.Setenv("LANEBUDGET_PROBE", "1")
	got := EnvFor(2, nil)
	if !slices.Contains(got, "LANEBUDGET_PROBE=1") || got[len(got)-1] != "GOMAXPROCS=2" {
		t.Errorf("EnvFor(2, nil) = %q, want the process environment and then GOMAXPROCS=2", got)
	}
}

func TestArgsFor_AOneCPUShareStillBoundsTheChild(t *testing.T) {
	got := ArgsFor(1, []string{"test", "./x"})
	if want := []string{"test", "-p", "1", "-parallel", "1", "./x"}; !slices.Equal(got, want) {
		t.Errorf("ArgsFor(1) = %q, want %q", got, want)
	}
}

func TestEnvFor_ASharedEnvWithSpareCapacityIsNeverWrittenThrough(t *testing.T) {
	in := make([]string, 1, 4)
	EnvFor(2, in)
	if spare := in[:2][1]; spare != "" {
		t.Errorf("EnvFor wrote %q into the caller's spare capacity, concurrent children would race", spare)
	}
}

func TestHolderProcs_TheHolderGetsTheHostLessTheShareOfTheOtherLanes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cpus, width, want int
	}{
		{14, 8, 7},
		{14, 4, 5},
		{14, 2, 7},
		{14, 1, 14},
		{2, 8, 1},
		{14, 0, 0},
		{14, -2, 0},
	}
	for _, c := range cases {
		if got := HolderProcs(c.cpus, c.width); got != c.want {
			t.Errorf("HolderProcs(%d, %d) = %d, want %d", c.cpus, c.width, got, c.want)
		}
	}
}

func TestHolderProcs_NeverFallsBelowTheLaneShare(t *testing.T) {
	t.Parallel()
	for cpus := 1; cpus <= 64; cpus++ {
		for width := 1; width <= 16; width++ {
			if got, floor := HolderProcs(cpus, width), Procs(cpus, width); got < floor || got > cpus {
				t.Errorf("HolderProcs(%d, %d) = %d, want within [%d, %d]", cpus, width, got, floor, cpus)
			}
		}
	}
}

func TestShareFor_AHolderGetsTheHolderShareAndAnyOtherRunTheLaneShare(t *testing.T) {
	inLane(t, 14, "8")
	if got := ShareFor(false); got != 1 {
		t.Errorf("ShareFor(false) = %d, want the lane share 1", got)
	}
	if got := ShareFor(true); got != 7 {
		t.Errorf("ShareFor(true) = %d, want the holder share 7", got)
	}
}

func TestShareFor_AHolderOnTheConsoleStaysUnbudgeted(t *testing.T) {
	inLane(t, 14, "")
	if got := ShareFor(true); got != 0 {
		t.Errorf("ShareFor(true) on the console = %d, want 0", got)
	}
}

func TestHolder_ReadsTheLiveWidthLikeCurrent(t *testing.T) {
	inLane(t, 14, "8")
	path := filepath.Join(t.TempDir(), LiveWidthFile)
	prev := processAlive
	processAlive = func(int) bool { return true }
	t.Cleanup(func() { processAlive = prev })
	if err := WriteLiveWidth(path, 1, 2); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ipcenv.FleetLiveWidthKey, path)
	if got := Holder(); got != 7 {
		t.Errorf("Holder() at live width 2 = %d, want 7", got)
	}
	if got := Current(); got != 7 {
		t.Errorf("Current() at live width 2 = %d, want 7", got)
	}
}
