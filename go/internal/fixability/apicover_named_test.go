package fixability

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestSignalNames_AreTheSpellingThatTheReadersParse(t *testing.T) {
	t.Parallel()
	for got, want := range map[string]string{
		SignalCode: "REPAIR_HANDOFF", FieldDecision: "decision", FieldReason: "reason", FieldBlockers: "blockers",
		DecisionNone: "none", DecisionWould: "would_stop", DecisionStop: "stop",
		ReasonConsoleCompanionPending: "console_companion_pending",
	} {
		if got != want {
			t.Errorf("name = %q, want %q", got, want)
		}
	}
}

func TestDecision_EachBlockRuleNamesItself(t *testing.T) {
	t.Parallel()
	in := Input{
		Defects:      []string{"M1: go/x/a.go is wrong; " + mergeConsole, "H2: see " + mergeConsole, "M3: the tests fail; " + mergeConsole},
		GateFailures: []string{"go/x/a.go: FAIL"}, OpenConsoleItems: []string{mergeConsole},
	}

	var d Decision = Classify(in)

	want := []string{BlocksGate, BlocksSeverity, BlocksRedTest}
	if len(d.Blockers) != 3 {
		t.Fatalf("blockers = %+v, want 3", d.Blockers)
	}
	for i, f := range d.Blockers {
		if f.Blocks != want[i] {
			t.Errorf("%s blocks by %q, want %q", f.ID, f.Blocks, want[i])
		}
	}
	if items := OpenConsole([]inboxbatch.Item{{ID: mergeConsole, Route: "console-manual"}}, nil); len(items) != 1 {
		t.Errorf("open console = %+v", items)
	}
}
