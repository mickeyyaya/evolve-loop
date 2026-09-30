package core

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// divergedCompositionFixture builds a two-commit repo whose "cycle" branch has
// diverged from "main" by one commit, and returns the worktree dir plus the
// composed diff and its patch-id — exactly what compositionCarryForward and
// scopedMergeCarryForward independently recompute via `git diff main...HEAD`,
// so a snapshot built from these values always matches at the patch-id check.
func divergedCompositionFixture(t *testing.T) (worktree string, diff []byte, patchID string) {
	t.Helper()
	r := gittest.Fixture(t)
	writeFile(t, filepath.Join(r.Dir, "file.txt"), "base\n")
	r.Git("add", "file.txt")
	r.Git("commit", "-q", "-m", "base")
	r.Git("checkout", "-q", "-b", "cycle")
	writeFile(t, filepath.Join(r.Dir, "file.txt"), "base\nchanged\n")
	r.Git("commit", "-aq", "-m", "change")
	got, exit, err := gitCapture(context.Background(), r.Dir, "diff", "main...HEAD")
	if err != nil || exit != 0 {
		t.Fatalf("fixture: git diff main...HEAD exit=%d err=%v", exit, err)
	}
	pid, err := compositionPatchID([]byte(got))
	if err != nil {
		t.Fatalf("fixture: patch-id: %v", err)
	}
	return r.Dir, []byte(got), pid
}

func snapshotFromFixture(diff []byte, patchID string) func(context.Context, string, string) (CompositionAuditSnapshot, error) {
	return func(context.Context, string, string) (CompositionAuditSnapshot, error) {
		return CompositionAuditSnapshot{LaneAuditRef: "test-ref", AuditedBase: "main", Diff: diff, PatchID: patchID}, nil
	}
}

func assertOneOrchestratorDeclineEvent(t *testing.T, events []signalcenter.Event, cycle int, wantGate, wantTail string) {
	t.Helper()
	if len(events) != 1 {
		t.Fatalf("events = %+v, want exactly one coded decline event", events)
	}
	e := events[0]
	if e.Code != CodeComposedGateDeclined {
		t.Errorf("event.Code = %q, want %q", e.Code, CodeComposedGateDeclined)
	}
	if mod, ok := signalcenter.IsRegistered(e.Code); !ok || mod != signalcenter.ModuleOrchestrator {
		t.Errorf("event.Code %q is not registered under signalcenter.ModuleOrchestrator (mod=%q ok=%v)", e.Code, mod, ok)
	}
	if e.Kind != signalcenter.KindGateRejected {
		t.Errorf("event.Kind = %q, want %q", e.Kind, signalcenter.KindGateRejected)
	}
	if e.Cycle != cycle {
		t.Errorf("event.Cycle = %d, want %d", e.Cycle, cycle)
	}
	if !strings.Contains(e.Reason, wantGate) {
		t.Errorf("event.Reason = %q, must name the failing gate %q", e.Reason, wantGate)
	}
	if !strings.Contains(eventText(e), wantTail) {
		t.Errorf("event = %+v, must carry the failing gate's captured tail %q in its Reason or Fields", e, wantTail)
	}
}

// eventText is everything a delivered event says: its Reason plus every Fields key and value.
func eventText(e signalcenter.Event) string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{e.Reason}
	for _, k := range keys {
		parts = append(parts, k+"="+e.Fields[k])
	}
	return strings.Join(parts, "\n")
}

func TestCompositionCarryForward_DeclineEmitsCodedSignalEvent(t *testing.T) {
	worktree, diff, patchID := divergedCompositionFixture(t)
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })

	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithSignalCenter(center),
		WithCompositionSnapshot(snapshotFromFixture(diff, patchID)),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
			return map[string]ciparity.GateOutcome{
				"compile":  {Status: "pass"},
				"test":     {Status: "fail", Tail: "TAIL_MARKER_ONE"},
				"acs":      {Status: "pass"},
				"apicover": {Status: "pass"},
			}
		}),
		WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error {
			t.Fatal("writer must not be called after a composed-gate decline")
			return nil
		}),
	)

	if o.compositionCarryForward(context.Background(), 42, CycleState{ActiveWorktree: worktree, RunID: "run-1"}, "") {
		t.Fatal("a red composed gate must not carry forward")
	}
	assertOneOrchestratorDeclineEvent(t, events, 42, "test", "TAIL_MARKER_ONE")
}

func TestCompositionCarryForward_MultiGateFailure_NamesAllInOneEvent(t *testing.T) {
	worktree, diff, patchID := divergedCompositionFixture(t)
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })

	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithSignalCenter(center),
		WithCompositionSnapshot(snapshotFromFixture(diff, patchID)),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
			return map[string]ciparity.GateOutcome{
				"compile":  {Status: "pass"},
				"test":     {Status: "fail", Tail: "TAIL_MARKER_TEST"},
				"acs":      {Status: "fail", Tail: "TAIL_MARKER_ACS"},
				"apicover": {Status: "pass"},
			}
		}),
		WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error {
			t.Fatal("writer must not be called after a composed-gate decline")
			return nil
		}),
	)

	if o.compositionCarryForward(context.Background(), 43, CycleState{ActiveWorktree: worktree, RunID: "run-1b"}, "") {
		t.Fatal("a red composed gate must not carry forward")
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want exactly ONE event naming both failing gates", events)
	}
	e := events[0]
	for _, gate := range []string{"test", "acs"} {
		if !strings.Contains(e.Reason, gate) {
			t.Errorf("event.Reason = %q, must name every failing gate (missing %q)", e.Reason, gate)
		}
	}
	for _, tail := range []string{"TAIL_MARKER_TEST", "TAIL_MARKER_ACS"} {
		if !strings.Contains(eventText(e), tail) {
			t.Errorf("event = %+v, must carry every failing gate's tail (missing %q)", e, tail)
		}
	}
}

func TestCompositionCarryForward_AllGatesGreen_NoDeclineEvent(t *testing.T) {
	worktree, diff, patchID := divergedCompositionFixture(t)
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })

	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithSignalCenter(center),
		WithCompositionSnapshot(snapshotFromFixture(diff, patchID)),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
			return map[string]ciparity.GateOutcome{
				"compile":  {Status: "pass"},
				"test":     {Status: "pass"},
				"acs":      {Status: "pass"},
				"apicover": {Status: "pass"},
			}
		}),
		WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error { return nil }),
	)

	if !o.compositionCarryForward(context.Background(), 44, CycleState{ActiveWorktree: worktree, RunID: "run-1c"}, "") {
		t.Fatal("all-green composed gates must carry forward")
	}
	if len(events) != 0 {
		t.Fatalf("events = %+v, want zero events on an all-green composed tree (no spurious decline)", events)
	}
}
