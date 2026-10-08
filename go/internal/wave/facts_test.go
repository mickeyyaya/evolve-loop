package wave_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type plane struct {
	t    *testing.T
	root string
}

func newPlane(t *testing.T) plane {
	t.Helper()
	return plane{t: t, root: t.TempDir()}
}

func (p plane) write(rel, body string) {
	p.t.Helper()
	path := filepath.Join(p.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func signalLine(kind, phase string, fields string) string {
	return `{"schema_version":"1.0","seq":1,"pid":1,"ts":"2026-10-08T12:00:00Z","module":"orchestrator","origin":"x","kind":"` +
		kind + `","phase":"` + phase + `","severity":"INFO","reason":"r","fields":{` + fields + `}}` + "\n"
}

func TestReadCycles_JoinsStateSignalsLandingAndDossiers(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-4/cycle-state.json", `{"cycle_id":4,"phase":"ship"}`)
	p.write(".evolve/runs/cycle-5/cycle-state.json", `{"cycle_id":5,"phase":"build"}`)
	p.write(".evolve/runs/cycle-5/signals.ndjson",
		signalLine("quota.paused", "scout", `"phase":"scout"`)+"not json\n"+signalLine("quota.paused", "tdd", `"phase":"tdd"`))
	p.write(".evolve/runs/cycle-6/run.json", `{"cycle_id":6,"phase":"retrospective"}`)
	p.write(".evolve/runs/cycle-6/signals.ndjson", signalLine("cycle.sealed", "", `"final_verdict":"PASS"`))
	p.write(".evolve/landing/cycle-6.json", `{"cycle":6,"status":"complete"}`)
	p.write(".evolve/landing/cycle-8.json", `{"cycle":8,"status":"prepared"}`)
	p.write("knowledge-base/cycles/cycle-7.json", `{"cycle":7,"final_verdict":"FAIL"}`)
	p.write("knowledge-base/cycles/cycle-9.json", `{"cycle":9,"final_verdict":"PASS","commit_sha":"abc"}`)
	p.write(".evolve/runs/cycle-10/cycle-state.json", `{"cycle_id":10,"phase":"scout"}`)

	got, warnings := wave.ReadCycles(p.root, 4, 9)

	want := []wave.Cycle{
		{ID: 5, Phase: "build", QuotaPauses: 2},
		{ID: 6, Phase: "retrospective", Verdict: "PASS", Shipped: true},
		{ID: 7, Verdict: "FAIL"},
		{ID: 8},
		{ID: 9, Verdict: "PASS", Shipped: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadCycles(floor 4, ceiling 9) =\n%+v\nwant\n%+v", got, want)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "cycle-5") {
		t.Errorf("warnings = %q, want one that names the malformed line in cycle-5", warnings)
	}
}

func TestReadCycles_GlobalCycleStateIsTheLivePhase(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-3/run.json", `{"cycle_id":3,"phase":"scout"}`)
	p.write(".evolve/cycle-state.json", `{"cycle_id":3,"phase":"audit","shipped":true}`)

	got, _ := wave.ReadCycles(p.root, 0, 0)

	want := []wave.Cycle{{ID: 3, Phase: "audit", Shipped: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadCycles = %+v, want %+v", got, want)
	}
}

func TestReadCycles_ACeilingOfZeroHasNoUpperBound(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-2/run.json", `{"cycle_id":2,"phase":"scout"}`)
	p.write(".evolve/runs/cycle-1200/run.json", `{"cycle_id":1200,"phase":"tdd"}`)
	p.write(".evolve/runs/cycle-x/run.json", `{}`)

	got, _ := wave.ReadCycles(p.root, 1, 0)

	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 1200 {
		t.Errorf("ReadCycles(floor 1, no ceiling) = %+v, want cycles 2 and 1200", got)
	}
}

func TestLastCycleNumber(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	evolveDir := filepath.Join(p.root, ".evolve")
	if n, err := wave.LastCycleNumber(evolveDir); err != nil || n != 0 {
		t.Errorf("LastCycleNumber(no state.json) = %d, %v; want 0, nil", n, err)
	}
	p.write(".evolve/state.json", `{"lastCycleNumber":1836,"failedApproaches":[]}`)
	if n, err := wave.LastCycleNumber(evolveDir); err != nil || n != 1836 {
		t.Errorf("LastCycleNumber = %d, %v; want 1836, nil", n, err)
	}
	p.write(".evolve/state.json", `{`)
	if _, err := wave.LastCycleNumber(evolveDir); err == nil {
		t.Error("LastCycleNumber read a malformed state.json without an error")
	}
}

func TestReadCycles_ACorruptStateIsAWarningNotSilence(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-5/cycle-state.json", `{`)
	p.write(".evolve/cycle-state.json", `{"cycle_id":`)

	got, warnings := wave.ReadCycles(p.root, 0, 0)

	if len(got) != 1 || got[0].ID != 5 || got[0].Phase != "" {
		t.Errorf("ReadCycles = %+v, want cycle 5 with no phase", got)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "cycle-5") || !strings.Contains(joined, "cycle-state.json") || len(warnings) != 2 {
		t.Errorf("warnings = %q, want one for the corrupt cycle-5 state and one for the corrupt global state", warnings)
	}
}

func TestReadCycles_ACycleAtTheFloorBelongsToTheEarlierWave(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-1836/run.json", `{"cycle_id":1836,"phase":"build"}`)
	p.write(".evolve/runs/cycle-1837/run.json", `{"cycle_id":1837,"phase":"scout"}`)

	later, _ := wave.ReadCycles(p.root, 1836, 0)
	earlier, _ := wave.ReadCycles(p.root, 1834, 1836)

	if len(later) != 1 || later[0].ID != 1837 || len(earlier) != 1 || earlier[0].ID != 1836 {
		t.Errorf("a resumed cycle 1836 at floor 1836: later wave %+v, earlier wave %+v; want 1837 and 1836", later, earlier)
	}
}
