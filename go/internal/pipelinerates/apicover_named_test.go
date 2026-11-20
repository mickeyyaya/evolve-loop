package pipelinerates

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func TestComparison_ReadingCarriesTheDifferenceAsTheCentreOfTheMDEBand(t *testing.T) {
	t.Parallel()
	c := Comparison{A: Rate{K: 1, N: 4}, B: Rate{K: 3, N: 4}, Diff: 0.5, MDE: 0.2, Status: qualitypanel.StatusOK}

	r := c.Reading("w")

	if r.Interval != (qualitypanel.Interval{Value: 0.5, Lo: 0.3, Hi: 0.7}) || r.N != 4 || r.Status != qualitypanel.StatusOK || r.Window != "w" {
		t.Errorf("reading = %+v", r)
	}
}

func TestShipPaths_AFakeListerFeedsPackagesPerShip(t *testing.T) {
	t.Parallel()
	var show ShipPaths = func(string) ([]string, error) { return []string{"go/x/a.go", "go/y/b.go"}, nil }
	readings, err := PackagesPerShip([]Cycle{{ID: 1, Wave: 1, Shipped: true, Commit: "c"}}, show)
	if err != nil || len(readings) != 1 || readings[0].Interval.Value != 2 {
		t.Errorf("readings %+v, %v; want a median of 2 packages", readings, err)
	}
}

func TestRecoveryExports_AreNamed(t *testing.T) {
	t.Parallel()
	var in RecoveryInput
	var res RecoveryResult = MeasureRecovery(in)
	if res.Recoveries != 0 || res.OQ4.Index != "OQ4" || len(res.Events(qualitypanel.StageShadow)) == 0 {
		t.Errorf("result = %+v, want an empty OQ4 reading", res)
	}
}
