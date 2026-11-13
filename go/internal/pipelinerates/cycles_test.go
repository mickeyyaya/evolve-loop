package pipelinerates

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func cycleByID(cs []Cycle, id int) Cycle {
	for _, c := range cs {
		if c.ID == id {
			return c
		}
	}
	return Cycle{}
}

func TestLoadCycles_ARunJSONPassWithAFailSealAndNoShipIsNotShipped(t *testing.T) {
	t.Parallel()
	root := wavesFixture(t)

	cycles, err := LoadCycles(root)

	if err != nil {
		t.Fatalf("LoadCycles: %v", err)
	}
	c := cycleByID(cycles, 2024)
	if c.ID != 2024 || c.Shipped || !c.Sealed || c.Wave != 140 {
		t.Errorf("cycle 2024 = %+v, want wave 140, sealed, not shipped: run.json final_verdict PASS is not a ship", c)
	}
}

func TestLoadCycles_ReadsTheItemAndTheShipCommit(t *testing.T) {
	t.Parallel()
	root := wavesFixture(t)
	writeCycle(t, root, fixtureCycle{id: 2028, shipLanded: true, commit: "90ceb5aa", item: "cli-ship-manual-paths-lane", seal: "final verdict PASS"})

	c := cycleByID(loaded(t, root), 2028)

	if c.Item != "cli-ship-manual-paths-lane" || c.Commit != "90ceb5aa" || !c.Shipped {
		t.Errorf("cycle 2028 = %+v, want its item, commit and ship", c)
	}
}

func TestLoadCycles_SkipsACycleThatIsStillRunning(t *testing.T) {
	t.Parallel()
	root := wavesFixture(t)
	if err := runlease.Write(filepath.Join(root, ".evolve", "runs", "cycle-2028"), runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}

	cycles := loaded(t, root)

	if len(cycles) != 20 || cycleByID(cycles, 2028).ID != 0 {
		t.Errorf("cycles = %d, want 20: a cycle with a live lease has no outcome yet", len(cycles))
	}
}

func TestLoadCycles_ReadsTheShipTimeFromTheShipLandedSignal(t *testing.T) {
	t.Parallel()
	root := wavesFixture(t)

	c := cycleByID(loaded(t, root), 2009)

	if want := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC); !c.ShippedAt.Equal(want) {
		t.Errorf("ShippedAt = %v, want %v", c.ShippedAt, want)
	}
}

func TestLoadCycles_AShipWithNoSignalTakesItsShipTimeFromRunJSON(t *testing.T) {
	t.Parallel()
	root := wavesFixture(t)
	writeCycle(t, root, fixtureCycle{id: 2028, runShipped: true, seal: "final verdict PASS"})
	stamp := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(root, ".evolve", "runs", "cycle-2028", "run.json"), stamp, stamp); err != nil {
		t.Fatal(err)
	}

	c := cycleByID(loaded(t, root), 2028)

	if !c.Shipped || !c.ShippedAt.Equal(stamp) {
		t.Errorf("cycle 2028 = %+v, want ShippedAt %v from the run.json mtime", c, stamp)
	}
}

func TestWithSignals_TheFirstShipLandedSignalGivesTheShipTime(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "signals.ndjson")
	writeLines(t, path,
		`{"ts":"2026-10-21T10:00:00Z","kind":"ship.landed","phase":"ship","fields":{"outcome":"landed"}}`,
		`{"ts":"2026-10-21T11:00:00Z","kind":"ship.landed","phase":"ship","fields":{"outcome":"landed"}}`)

	c, err := withSignals(Cycle{}, path)

	if want := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC); err != nil || !c.ShippedAt.Equal(want) {
		t.Errorf("ShippedAt = %v, %v; want the first signal at %v", c.ShippedAt, err, want)
	}
}
