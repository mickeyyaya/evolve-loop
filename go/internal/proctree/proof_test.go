package proctree

import (
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

var t0 = time.Date(2026, 10, 8, 14, 0, 0, 0, time.UTC)

func proc(pid, ppid int, env map[string]string, args ...string) Process {
	return Process{Pid: pid, Ppid: ppid, Pgid: pid, Started: t0, Comm: "x", Args: args, Env: env}
}

func pids(ps []Process) []int {
	out := []int{}
	for _, p := range ps {
		out = append(out, p.Pid)
	}
	return out
}

func TestTaggedWith_MatchesOnlyTheExactDispatchIDInTheEnvironment(t *testing.T) {
	t.Parallel()
	id := "01RUN/1835/build/p4242n7"
	table := []Process{
		proc(10, 1, map[string]string{ipcenv.DispatchIDKey: id}),
		proc(11, 1, map[string]string{ipcenv.DispatchIDKey: id + "x"}),
		proc(12, 1, map[string]string{ipcenv.DispatchIDKey: "01RUN/1835/build/p4242n8"}),
		proc(13, 1, nil, "grep", ipcenv.DispatchIDKey+"="+id),
		proc(14, 1, map[string]string{"EVOLVE_PROJECT_ROOT": id}),
	}

	got := Select(table, TaggedWith(id))

	if !reflect.DeepEqual(pids(got), []int{10}) {
		t.Errorf("TaggedWith selected %v, want [10]", pids(got))
	}
}

func TestTaggedWith_ATagInTheArgumentsIsNoProof(t *testing.T) {
	t.Parallel()
	id := "01RUN/1835/build/p4242n7"
	p := proc(13, 1, map[string]string{}, "/usr/bin/grep", ipcenv.DispatchIDKey+"="+id)

	var proof Proof = TaggedWith(id)
	if proof(p) {
		t.Errorf("TaggedWith matched a process that only names the tag in its arguments")
	}
}

func TestTaggedWith_AnEmptyIDMatchesNothing(t *testing.T) {
	t.Parallel()
	p := proc(10, 1, map[string]string{ipcenv.DispatchIDKey: ""})

	if TaggedWith("")(p) {
		t.Errorf("TaggedWith(\"\") matched a process with an empty tag: an empty id is no proof")
	}
}

func TestDescendants_ReturnsTheRootsAndEveryProcessBelowThem(t *testing.T) {
	t.Parallel()
	table := []Process{
		proc(100, 50, nil),
		proc(101, 100, nil),
		proc(102, 101, nil),
		proc(103, 1, nil),
		proc(104, 103, nil),
		proc(50, 1, nil),
	}

	got := Descendants(table, []int{100})

	if !reflect.DeepEqual(pids(got), []int{100, 101, 102}) {
		t.Errorf("Descendants(100) = %v, want [100 101 102]: the parent 50 and the unrelated 103 tree stay out", pids(got))
	}
}

func TestRecorded_MatchesPidAndStartTimeTogether(t *testing.T) {
	t.Parallel()
	rec := Recorded([]Identity{{Pid: 100, Started: t0}})
	same := proc(100, 1, nil)
	reused := proc(100, 1, nil)
	reused.Started = t0.Add(3 * time.Second)

	if !rec(same) {
		t.Errorf("Recorded refused the recorded process")
	}
	if rec(reused) {
		t.Errorf("Recorded matched pid 100 with a different start time: a reused pid is a different process")
	}
}

func TestAnyOf_IsTheUnionOfItsProofs(t *testing.T) {
	t.Parallel()
	id := "r/1/a/p1n1"
	table := []Process{
		proc(10, 1, map[string]string{ipcenv.DispatchIDKey: id}),
		proc(11, 1, nil),
		proc(12, 1, nil),
	}

	got := Select(table, AnyOf(TaggedWith(id), Recorded([]Identity{{Pid: 11, Started: t0}})))

	if !reflect.DeepEqual(pids(got), []int{10, 11}) {
		t.Errorf("AnyOf selected %v, want [10 11]", pids(got))
	}
}

func TestDescendants_RefusesInitAndPidZeroAsARoot(t *testing.T) {
	t.Parallel()
	table := []Process{proc(1, 0, nil), proc(200, 1, nil), proc(201, 200, nil)}

	got := Descendants(table, []int{0, 1, -5})

	if len(got) != 0 {
		t.Errorf("Descendants(0, 1, -5) = %v, want none: init's tree is every orphan on the host", pids(got))
	}
}

func TestIdentities_KeepsThePidAndTheStartTimeOfEachProcess(t *testing.T) {
	t.Parallel()
	later := proc(11, 1, nil)
	later.Started = t0.Add(time.Minute)

	got := Identities([]Process{proc(10, 1, nil), later})

	want := []Identity{{Pid: 10, Started: t0}, {Pid: 11, Started: t0.Add(time.Minute)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Identities = %+v, want %+v", got, want)
	}
}
