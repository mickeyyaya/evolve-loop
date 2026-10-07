package llmroute

import (
	"slices"
	"testing"
)

func TestDefaultTriggers_IsTheConservativeSet(t *testing.T) {
	got := DefaultTriggers()
	if !slices.Equal(got, []int{80, 81, 85, 87, 124, 127}) {
		t.Fatalf("DefaultTriggers = %v", got)
	}
	got[0] = 1
	if DefaultTriggers()[0] != 80 {
		t.Fatal("DefaultTriggers must return a copy")
	}
}

func TestExitModelMismatch_IsADefaultTriggerThatAdvancesTheChain(t *testing.T) {
	if !slices.Contains(DefaultTriggers(), ExitModelMismatch) || !(Plan{Triggers: DefaultTriggers()}).TriggersFallback(ExitModelMismatch) {
		t.Fatalf("DefaultTriggers = %v: a launch that booted another model family must hand the attempt to the next CLI", DefaultTriggers())
	}
}
