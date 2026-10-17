package lanebudget

import (
	"context"
	"io"
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
