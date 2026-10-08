package proctree

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type sent struct {
	pid int
	sig syscall.Signal
}

type fakeHost struct {
	tables  [][]Process
	listErr []error
	calls   int
	sent    []sent
	slept   []time.Duration
	refuse  map[int]error
}

func (h *fakeHost) list(context.Context) ([]Process, error) {
	n := h.calls
	h.calls++
	if n < len(h.listErr) && h.listErr[n] != nil {
		return nil, h.listErr[n]
	}
	i := min(n, len(h.tables)-1)
	return h.tables[i], nil
}

func (h *fakeHost) signal(pid int, sig syscall.Signal) error {
	h.sent = append(h.sent, sent{pid, sig})
	return h.refuse[pid]
}

func (h *fakeHost) reaper() Reaper {
	var list Lister = h.list
	var signal Signaler = h.signal
	return Reaper{List: list, Signal: signal, Sleep: func(d time.Duration) { h.slept = append(h.slept, d) }, Grace: 2 * time.Second, Protected: []int{7000, 6999}}
}

const reapID = "01RUN/1835/build/p4242n7"

func tagged(pid, ppid int) Process {
	return proc(pid, ppid, map[string]string{ipcenv.DispatchIDKey: reapID})
}

func TestReap_TermThenKillOnlyTheProcessesThatStillMatch(t *testing.T) {
	t.Parallel()
	a, b, c := tagged(100, 1), tagged(101, 100), proc(102, 1, nil)
	h := &fakeHost{tables: [][]Process{{a, b, c}, {b, c}, {c}}}

	rep := h.reaper().Reap(context.Background(), TaggedWith(reapID))

	want := []sent{{100, syscall.SIGTERM}, {101, syscall.SIGTERM}, {101, syscall.SIGKILL}}
	if !reflect.DeepEqual(h.sent, want) {
		t.Fatalf("signals = %v, want %v: SIGTERM each match, SIGKILL only the one still there after the grace time", h.sent, want)
	}
	if !reflect.DeepEqual(h.slept[0], 2*time.Second) {
		t.Errorf("first wait = %v, want the 2s grace", h.slept)
	}
	if !reflect.DeepEqual(pids(rep.Terminated), []int{100, 101}) || !reflect.DeepEqual(pids(rep.Killed), []int{101}) || len(rep.Survivors) != 0 {
		t.Errorf("report = %+v", rep)
	}
}

func TestReap_ASurvivorOfKillIsReported(t *testing.T) {
	t.Parallel()
	a := tagged(100, 1)
	h := &fakeHost{tables: [][]Process{{a}, {a}, {a}}}

	var rep Report = h.reaper().Reap(context.Background(), TaggedWith(reapID))

	if !reflect.DeepEqual(pids(rep.Survivors), []int{100}) {
		t.Errorf("Survivors = %v, want [100]: a process still there after SIGKILL is a survivor", pids(rep.Survivors))
	}
}

func TestReap_APidReusedByANewProcessIsNotKilled(t *testing.T) {
	t.Parallel()
	old := tagged(100, 1)
	reused := tagged(100, 1)
	reused.Started = t0.Add(5 * time.Second)
	h := &fakeHost{tables: [][]Process{{old}, {reused}, {reused}}}

	rep := h.reaper().Reap(context.Background(), TaggedWith(reapID))

	if want := []sent{{100, syscall.SIGTERM}}; !reflect.DeepEqual(h.sent, want) {
		t.Errorf("signals = %v, want %v: pid 100 now names a process that started later", h.sent, want)
	}
	if len(rep.Survivors) != 0 {
		t.Errorf("Survivors = %v, want none", pids(rep.Survivors))
	}
}

func TestReap_AProcessThatLostItsProofIsNotKilled(t *testing.T) {
	t.Parallel()
	before := tagged(100, 1)
	after := proc(100, 1, map[string]string{})
	h := &fakeHost{tables: [][]Process{{before}, {after}, {after}}}

	h.reaper().Reap(context.Background(), TaggedWith(reapID))

	if want := []sent{{100, syscall.SIGTERM}}; !reflect.DeepEqual(h.sent, want) {
		t.Errorf("signals = %v, want %v: each round checks the proof again", h.sent, want)
	}
}

func TestReap_NeverSignalsInitItselfOrItsParent(t *testing.T) {
	t.Parallel()
	table := []Process{tagged(1, 0), tagged(7000, 6999), tagged(6999, 1), tagged(0, 0), tagged(300, 1)}
	h := &fakeHost{tables: [][]Process{table, {}, {}}}

	rep := h.reaper().Reap(context.Background(), TaggedWith(reapID))

	if want := []sent{{300, syscall.SIGTERM}}; !reflect.DeepEqual(h.sent, want) {
		t.Errorf("signals = %v, want %v: pid 0, pid 1 and the protected pids are never signaled", h.sent, want)
	}
	if len(rep.Errors) != 4 {
		t.Errorf("Errors = %v, want one refusal for each of the 4 protected pids", rep.Errors)
	}
}

func TestReap_SignalsSinglePidsOnly(t *testing.T) {
	t.Parallel()
	h := &fakeHost{tables: [][]Process{{tagged(-300, 1), tagged(300, 1)}, {}, {}}}

	h.reaper().Reap(context.Background(), TaggedWith(reapID))

	for _, s := range h.sent {
		if s.pid <= 1 {
			t.Errorf("signal %v to pid %d: a pid at or below 1 addresses a group or init", s.sig, s.pid)
		}
	}
}

func TestReap_AListingFailureAfterTermSendsNoKill(t *testing.T) {
	t.Parallel()
	h := &fakeHost{tables: [][]Process{{tagged(100, 1)}}, listErr: []error{nil, errors.New("ps: interrupted")}}

	rep := h.reaper().Reap(context.Background(), TaggedWith(reapID))

	if want := []sent{{100, syscall.SIGTERM}}; !reflect.DeepEqual(h.sent, want) {
		t.Errorf("signals = %v, want %v: no SIGKILL without a fresh proof", h.sent, want)
	}
	if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "ps: interrupted") {
		t.Errorf("Errors = %v, want the listing failure", rep.Errors)
	}
}

func TestReap_AFirstListingFailureSignalsNothing(t *testing.T) {
	t.Parallel()
	h := &fakeHost{tables: [][]Process{{tagged(100, 1)}}, listErr: []error{errors.New("ps: not found")}}

	rep := h.reaper().Reap(context.Background(), TaggedWith(reapID))

	if len(h.sent) != 0 || len(rep.Errors) != 1 {
		t.Errorf("signals = %v errors = %v, want no signal and one error", h.sent, rep.Errors)
	}
}

func TestReap_AProcessAlreadyGoneIsNoError(t *testing.T) {
	t.Parallel()
	h := &fakeHost{tables: [][]Process{{tagged(100, 1), tagged(101, 1)}, {}, {}}, refuse: map[int]error{100: syscall.ESRCH, 101: syscall.EPERM}}

	rep := h.reaper().Reap(context.Background(), TaggedWith(reapID))

	if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "101") {
		t.Errorf("Errors = %v, want only the EPERM of pid 101", rep.Errors)
	}
	if !reflect.DeepEqual(pids(rep.Terminated), []int{100}) {
		t.Errorf("Terminated = %v, want [100]", pids(rep.Terminated))
	}
}

func TestReap_NoMatchListsOnceAndSleepsNever(t *testing.T) {
	t.Parallel()
	h := &fakeHost{tables: [][]Process{{proc(100, 1, nil)}}}

	h.reaper().Reap(context.Background(), TaggedWith(reapID))

	if h.calls != 1 || len(h.slept) != 0 || len(h.sent) != 0 {
		t.Errorf("calls=%d slept=%v sent=%v, want one listing and nothing else", h.calls, h.slept, h.sent)
	}
}

func TestReap_NeverSignalsAnAncestorOfTheReaper(t *testing.T) {
	t.Parallel()
	self, parent, grand := tagged(8000, 7990), tagged(7990, 7900), tagged(7900, 1)
	child := tagged(8100, 8000)
	h := &fakeHost{tables: [][]Process{{self, parent, grand, child}, {}, {}}}
	r := h.reaper()
	r.Protected, r.Self = nil, 8000

	r.Reap(context.Background(), TaggedWith(reapID))

	if want := []sent{{8100, syscall.SIGTERM}}; !reflect.DeepEqual(h.sent, want) {
		t.Errorf("signals = %v, want %v: the reaper, its parent and its grandparent are never signaled", h.sent, want)
	}
}

func TestReap_RefusesToRunWhenItsOwnPidIsNotInTheTable(t *testing.T) {
	t.Parallel()
	h := &fakeHost{tables: [][]Process{{tagged(300, 1)}, {}, {}}}
	r := h.reaper()
	r.Self = 8000

	rep := r.Reap(context.Background(), TaggedWith(reapID))

	if len(h.sent) != 0 || len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "8000") {
		t.Errorf("signals=%v errors=%v, want no signal and one error that names pid 8000: a table without the reaper cannot prove its ancestors", h.sent, rep.Errors)
	}
}
