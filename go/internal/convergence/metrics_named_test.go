package convergence_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func TestMeasure_ReturnsTheMetricsDocumentType(t *testing.T) {
	var m convergence.Metrics = convergence.Measure([]convergence.Input{consoleLaneThreeRounds()})

	if len(m.RoundsToLand) != 1 || m.RoundsToLand[0].Runs != 1 || m.RoundsToLand[0].Max != 2 {
		t.Fatalf("Metrics = %+v, want one console-lane run that landed in round 2", m)
	}
}
