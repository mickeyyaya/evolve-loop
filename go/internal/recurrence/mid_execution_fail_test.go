package recurrence

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

// ADR-0103 unit 03b: the generic-pattern denylist is a CONSUMER of the
// engine's default class — the key is the projected constant, so a re-spelling
// on either side turns this red instead of silently promoting the class into
// the recurrence counts.
func TestRecurrence_DenylistNamesTheEnginesDefaultClass(t *testing.T) {
	if !IsGeneric(cyclestate.ClassificationMidExecutionFail, "") {
		t.Fatalf("%q must be generic (the denylist projects the engine's default class)", cyclestate.ClassificationMidExecutionFail)
	}
	if !genericPatternDenylist[cyclestate.ClassificationMidExecutionFail] {
		t.Fatal("the denylist key must BE the constant, not a second literal")
	}
}

// The denylist projects every classification that HAS an owner: failurelog's
// typed constants beside the engine's default class. cycle-fatal and unknown
// have no owner and stay literals.
func TestRecurrence_DenylistProjectsTheOwnedClassifications(t *testing.T) {
	for _, c := range []failurelog.Classification{failurelog.OperatorReset, failurelog.LoopFatal, failurelog.UnknownClassification} {
		if !genericPatternDenylist[string(c)] {
			t.Fatalf("%q must be denylisted through its owner's constant", c)
		}
	}
}
