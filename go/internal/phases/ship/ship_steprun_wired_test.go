package ship

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

func TestStepRunWired_FalseWithoutStepRunTrueWithOne(t *testing.T) {
	t.Parallel()
	if New(Config{}).StepRunWired() {
		t.Fatal("StepRunWired = true for a Phase built without StepRun")
	}
	run := func(context.Context, string, []baseattr.Key) ([]baseattr.Key, error) { return nil, nil }
	if !New(Config{StepRun: run}).StepRunWired() {
		t.Fatal("StepRunWired = false for a Phase built with StepRun")
	}
}
