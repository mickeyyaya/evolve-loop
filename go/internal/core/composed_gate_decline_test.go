package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// realisticGateTail is a 20-line, >2 KB gate tail shaped like `make test` output. Its lines carry no
// tabs: the Signal Center turns control characters into spaces, and the last line must compare verbatim.
func realisticGateTail(gate string) (tail, lastLine string) {
	lines := make([]string, 0, 20)
	for i := 1; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("    %s_contract_test.go:%d: gate %s line %02d: the composed tree regressed against the audited fixture output here", gate, 100+i, gate, i))
	}
	lastLine = fmt.Sprintf("FAIL github.com/mickeyyaya/evolve-loop/go/internal/%s 41.207s %s_TAIL_LAST_LINE", gate, strings.ToUpper(gate))
	return strings.Join(append(lines, lastLine), "\n"), lastLine
}

func declineEvents(events []signalcenter.Event) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Code == CodeComposedGateDeclined {
			out = append(out, e)
		}
	}
	return out
}

// assertDeclineStderrTagged fails when a composed-gate decline dumps any line of a gate's tail to stderr, or
// writes a line about the gates that names neither the cycle nor the decline's event code.
func assertDeclineStderrTagged(t *testing.T, stderr string, cycle int, tails ...string) {
	t.Helper()
	cycleTag := regexp.MustCompile(fmt.Sprintf(`\bcycle[ =:-]?%d\b`, cycle))
	for _, line := range strings.Split(stderr, "\n") {
		for _, tail := range tails {
			for _, tailLine := range strings.Split(tail, "\n") {
				if tailLine = strings.TrimSpace(tailLine); tailLine != "" && strings.Contains(line, tailLine) {
					t.Errorf("stderr line %q dumps the gate output line %q; the tail belongs in the coded event only", line, tailLine)
				}
			}
		}
		if strings.Contains(strings.ToLower(line), "gate") &&
			!(cycleTag.MatchString(line) && strings.Contains(line, string(CodeComposedGateDeclined))) {
			t.Errorf("stderr line %q reports the gate decline without naming cycle %d and code %s", line, cycle, CodeComposedGateDeclined)
		}
	}
}

func declineOrchestrator(t *testing.T, center *signalcenter.Center, diff []byte, patchID string, outcomes map[string]ciparity.GateOutcome, extra ...Option) *Orchestrator {
	t.Helper()
	opts := append([]Option{
		WithSignalCenter(center),
		WithCompositionSnapshot(snapshotFromFixture(diff, patchID)),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome { return outcomes }),
		WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error {
			t.Error("writer must not be called after a composed-gate decline")
			return nil
		}),
	}, extra...)
	return NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opts...)
}

func TestCompositionCarryForward_RealisticTails_EveryFailingGatesLastLineReachesTheEvent(t *testing.T) {
	for name, failing := range map[string][]string{
		"two failing gates":              {"test", "acs"},
		"a compile break fails all four": {"compile", "test", "acs", "apicover"},
	} {
		t.Run(name, func(t *testing.T) {
			worktree, diff, patchID := divergedCompositionFixture(t)
			outcomes := allComposedGatesPass()
			lastLines := map[string]string{}
			for _, gate := range failing {
				tail, last := realisticGateTail(gate)
				if len(tail) < 2048 {
					t.Fatalf("fixture tail for %s is %d bytes, want >= 2048", gate, len(tail))
				}
				outcomes[gate] = ciparity.GateOutcome{Status: "fail", Tail: tail}
				lastLines[gate] = last
			}
			center := signalcenter.New()
			var events []signalcenter.Event
			center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
			o := declineOrchestrator(t, center, diff, patchID, outcomes)

			if o.compositionCarryForward(context.Background(), 42, CycleState{ActiveWorktree: worktree, RunID: "run-tails"}, "") {
				t.Fatal("red composed gates must not carry forward")
			}

			if len(events) != 1 {
				t.Fatalf("events = %+v, want exactly one coded decline event", events)
			}
			e := events[0]
			if e.Code != CodeComposedGateDeclined || e.Cycle != 42 {
				t.Errorf("event code=%q cycle=%d, want %q for cycle 42", e.Code, e.Cycle, CodeComposedGateDeclined)
			}
			for _, gate := range failing {
				if !strings.Contains(e.Reason, gate) {
					t.Errorf("event.Reason = %q, must name the failing gate %q", e.Reason, gate)
				}
				if !strings.Contains(eventText(e), lastLines[gate]) {
					t.Errorf("gate %s: the tail's last line %q did not survive into the delivered event %+v", gate, lastLines[gate], e)
				}
			}
		})
	}
}

func TestCompositionCarryForward_AbsentGateIsNamedInTheDeclineEvent(t *testing.T) {
	worktree, diff, patchID := divergedCompositionFixture(t)
	outcomes := allComposedGatesPass()
	delete(outcomes, "apicover")
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	o := declineOrchestrator(t, center, diff, patchID, outcomes)

	if o.compositionCarryForward(context.Background(), 45, CycleState{ActiveWorktree: worktree, RunID: "run-absent"}, "") {
		t.Fatal("a gate the runner never reported must not carry forward")
	}

	if len(events) != 1 || !strings.Contains(events[0].Reason, "apicover") {
		t.Fatalf("events = %+v, want one decline event naming the unreported gate apicover", events)
	}
}

func TestCompositionCarryForward_GateDeclineWritesNoUntaggedStderr(t *testing.T) {
	worktree, diff, patchID := divergedCompositionFixture(t)
	testTail, _ := realisticGateTail("test")
	acsTail, _ := realisticGateTail("acs")
	outcomes := allComposedGatesPass()
	outcomes["test"] = ciparity.GateOutcome{Status: "fail", Tail: testTail}
	outcomes["acs"] = ciparity.GateOutcome{Status: "fail", Tail: acsTail}
	o := declineOrchestrator(t, signalcenter.New(), diff, patchID, outcomes)

	stderr := captureStderr(t, func() {
		if o.compositionCarryForward(context.Background(), 42, CycleState{ActiveWorktree: worktree, RunID: "run-stderr"}, "") {
			t.Error("red composed gates must not carry forward")
		}
	})

	assertDeclineStderrTagged(t, stderr, 42, testTail, acsTail)
}

func TestScopedMergeCarryForward_GateDeclineWritesNoUntaggedStderr(t *testing.T) {
	worktree, diff, patchID := divergedCompositionFixture(t)
	acsTail, _ := realisticGateTail("acs")
	outcomes := allComposedGatesPass()
	outcomes["acs"] = ciparity.GateOutcome{Status: "fail", Tail: acsTail}
	o := declineOrchestrator(t, signalcenter.New(), diff, patchID, outcomes,
		WithScopedMergeReviewer(func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
			return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}
		}))

	stderr := captureStderr(t, func() {
		if o.scopedMergeCarryForward(context.Background(), 7, CycleState{ActiveWorktree: worktree, RunID: "run-scoped-stderr"}, "") {
			t.Error("a red composed gate must not carry forward via the scoped-merge rung")
		}
	})

	assertDeclineStderrTagged(t, stderr, 7, acsTail)
}

func TestIdentityCarryForward_GateDeclineEmitsOneCodedSignalEvent(t *testing.T) {
	h := pendedIdenticalLane(t)
	testTail, testLast := realisticGateTail("test")
	acsTail, acsLast := realisticGateTail("acs")
	h.gates["test"] = ciparity.GateOutcome{Status: "fail", Tail: testTail}
	h.gates["acs"] = ciparity.GateOutcome{Status: "fail", Tail: acsTail}
	h.signals = signalcenter.New()
	var events []signalcenter.Event
	h.signals.Subscribe(func(e signalcenter.Event) { events = append(events, e) })

	var next Phase
	stderr := captureStderr(t, func() { next, _, _ = h.route(t, h.auditRow()) })

	if next != PhaseAudit {
		t.Fatalf("route = %s, want Audit: a red composed gate on the identity carry is re-audited", next)
	}
	declines := declineEvents(events)
	if len(declines) != 1 {
		t.Fatalf("decline events = %+v (all events %+v), want exactly one coded identity-carry decline", declines, events)
	}
	e := declines[0]
	if mod, ok := signalcenter.IsRegistered(e.Code); !ok || mod != signalcenter.ModuleOrchestrator {
		t.Errorf("event.Code %q is not registered under signalcenter.ModuleOrchestrator (mod=%q ok=%v)", e.Code, mod, ok)
	}
	if e.Kind != signalcenter.KindGateRejected || e.Cycle != unwindCycle || e.Origin != "Orchestrator.identityCarryForward" {
		t.Errorf("event kind=%q cycle=%d origin=%q, want %q, cycle %d, Orchestrator.identityCarryForward", e.Kind, e.Cycle, e.Origin, signalcenter.KindGateRejected, unwindCycle)
	}
	for gate, last := range map[string]string{"test": testLast, "acs": acsLast} {
		if !strings.Contains(e.Reason, gate) {
			t.Errorf("event.Reason = %q, must name the failing gate %q", e.Reason, gate)
		}
		if !strings.Contains(eventText(e), last) {
			t.Errorf("gate %s: the tail's last line %q did not survive into the delivered event %+v", gate, last, e)
		}
	}
	assertDeclineStderrTagged(t, stderr, unwindCycle, testTail, acsTail)
}

func TestIdentityCarryForward_NonGateOutcomesEmitNoDeclineEvent(t *testing.T) {
	for name, tc := range map[string]struct {
		arrange  func(h *carryHarness)
		wantNext Phase
	}{
		"green gates carry":                      {func(*carryHarness) {}, PhaseShip},
		"a writer failure is not a gate decline": {func(h *carryHarness) { h.writeErr = fmt.Errorf("ledger sealed") }, PhaseAudit},
	} {
		t.Run(name, func(t *testing.T) {
			h := pendedIdenticalLane(t)
			tc.arrange(h)
			h.signals = signalcenter.New()
			var events []signalcenter.Event
			h.signals.Subscribe(func(e signalcenter.Event) { events = append(events, e) })

			next, _, _ := h.route(t, h.auditRow())

			if next != tc.wantNext {
				t.Fatalf("route = %s, want %s", next, tc.wantNext)
			}
			if declines := declineEvents(events); len(declines) != 0 {
				t.Errorf("decline events = %+v, want none: no composed gate failed", declines)
			}
		})
	}
}
