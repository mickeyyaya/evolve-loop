package recurrence

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

func TestRecurrence_DenylistNamesTheEnginesDefaultClass(t *testing.T) {
	if !IsGeneric(cyclestate.ClassificationMidExecutionFail, "") {
		t.Fatalf("%q must be generic (the denylist projects the engine's default class)", cyclestate.ClassificationMidExecutionFail)
	}
	if !genericPatternDenylist[cyclestate.ClassificationMidExecutionFail] {
		t.Fatal("the denylist key must BE the constant, not a second literal")
	}
}

func TestRecurrence_DenylistProjectsTheOwnedClassifications(t *testing.T) {
	for _, c := range []failurelog.Classification{failurelog.OperatorReset, failurelog.LoopFatal, failurelog.UnknownClassification} {
		if !genericPatternDenylist[string(c)] {
			t.Fatalf("%q must be denylisted through its owner's constant", c)
		}
	}
}
