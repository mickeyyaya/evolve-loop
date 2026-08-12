package contextfillcorrelate

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
)

// TestCorrelateBucketsByPeakFill is the table-driven core of the join: each case
// pins which band a cycle's PEAK per-phase ratio lands in, including the
// open-ended top band whose lower bound is contextfill.HotThreshold.
func TestCorrelateBucketsByPeakFill(t *testing.T) {
	cases := []struct {
		name     string
		fills    []float64
		wantBand float64 // expected bucket Min
	}{
		{"low band takes the peak, not the first entry", []float64{0.05, 0.42}, 0.00},
		{"mid band", []float64{0.10, 0.55}, 0.50},
		{"upper-cold band", []float64{0.71}, 0.70},
		{"hot boundary is inclusive", []float64{contextfill.HotThreshold}, contextfill.HotThreshold},
		{"overrun stays in the open-ended top band", []float64{1.40}, contextfill.HotThreshold},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := Correlate([]CycleFill{{Cycle: 1, Verdict: "FAIL", Fills: tc.fills}})
			if rep.CyclesJoined != 1 {
				t.Fatalf("CyclesJoined = %d, want 1", rep.CyclesJoined)
			}
			for _, b := range rep.Buckets {
				if b.Cycles == 0 {
					continue
				}
				if math.Abs(b.Min-tc.wantBand) > 1e-9 {
					t.Errorf("cycle landed in band Min=%v, want %v", b.Min, tc.wantBand)
				}
			}
		})
	}
}

// TestCorrelateNeverFabricatesAZeroRatio pins the negative case at the unit
// level: absent fill data and an absent verdict are both no-data, and the
// lowest band must stay untouched by them.
func TestCorrelateNeverFabricatesAZeroRatio(t *testing.T) {
	rep := Correlate([]CycleFill{
		{Cycle: 1, Verdict: "PASS"},
		{Cycle: 2, Verdict: "", Fills: []float64{0.9}},
		{Cycle: 3, Verdict: "PASS", Fills: []float64{0}}, // omitempty zero == absent
	})
	if rep.CyclesJoined != 0 {
		t.Errorf("CyclesJoined = %d, want 0 — none of these rows is joinable", rep.CyclesJoined)
	}
	if len(rep.NoData) != 3 {
		t.Errorf("NoData = %v, want all three cycles", rep.NoData)
	}
	for _, b := range rep.Buckets {
		if b.Cycles != 0 || b.FailRate != 0 {
			t.Errorf("bucket %q = %+v, want empty with a finite 0 rate", b.Label, b)
		}
	}
}

// TestCorrelateTalliesEveryRecognisedVerdict exercises all three arms of the
// tally — WARN included, which no test, predicate, or corpus row previously
// reached even though Bucket.Warn and the report's warn column are exported.
func TestCorrelateTalliesEveryRecognisedVerdict(t *testing.T) {
	rep := Correlate([]CycleFill{
		{Cycle: 1, Verdict: "FAIL", Fills: []float64{0.10}},
		{Cycle: 2, Verdict: "PASS", Fills: []float64{0.10}},
		{Cycle: 3, Verdict: "WARN", Fills: []float64{0.10}},
		{Cycle: 4, Verdict: "warn", Fills: []float64{0.10}}, // case-folded, same arm
	})
	if rep.CyclesJoined != 4 {
		t.Fatalf("CyclesJoined = %d, want 4", rep.CyclesJoined)
	}
	b := rep.Buckets[0]
	if b.Fail != 1 || b.Pass != 1 || b.Warn != 2 {
		t.Errorf("tally = fail:%d pass:%d warn:%d, want 1/1/2", b.Fail, b.Pass, b.Warn)
	}
	if b.Fail+b.Pass+b.Warn != b.Cycles {
		t.Errorf("tally sums to %d but Cycles = %d — a counted cycle went untallied",
			b.Fail+b.Pass+b.Warn, b.Cycles)
	}
	// WARN is not a failure: it must not move the FAIL rate.
	if b.FailRate != 0.25 {
		t.Errorf("FailRate = %v, want 0.25 (1 FAIL of 4)", b.FailRate)
	}
}

// TestCorrelateExcludesUnrecognisedVerdicts is the D1 regression: an unknown
// verdict must land in NoData, not raise a bucket's denominator. Before the
// fix it was counted as a cycle that could never be a FAIL, diluting FailRate.
func TestCorrelateExcludesUnrecognisedVerdicts(t *testing.T) {
	rep := Correlate([]CycleFill{
		{Cycle: 1, Verdict: "FAIL", Fills: []float64{0.10}},
		{Cycle: 2, Verdict: "BLOCKED", Fills: []float64{0.10}}, // schema drift
	})
	if rep.CyclesJoined != 1 {
		t.Errorf("CyclesJoined = %d, want 1 — BLOCKED is not a tallyable verdict", rep.CyclesJoined)
	}
	if len(rep.NoData) != 1 || rep.NoData[0] != 2 {
		t.Errorf("NoData = %v, want [2]", rep.NoData)
	}
	if got := rep.Buckets[0].FailRate; got != 1.0 {
		t.Errorf("FailRate = %v, want 1.0 — the unrecognised row diluted the rate", got)
	}
	if got := rep.Hot.Cycles + rep.Cold.Cycles; got != 1 {
		t.Errorf("hot+cold cycles = %d, want 1 — the unrecognised row leaked into the split", got)
	}
}

// TestLoadRejectsADuplicateBodyCycleID is the D2 regression: two dossiers whose
// bodies claim one id must stay two distinct cycles, identified by their
// (unique-by-construction) filenames.
func TestLoadRejectsADuplicateBodyCycleID(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"cycle-11.json", "cycle-12.json"} {
		// Both bodies lie, claiming to be cycle 11.
		body := []byte(`{"cycle": 11, "final_verdict": "FAIL"}`)
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	rows, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	ids := map[int]bool{}
	for _, r := range rows {
		if ids[r.Cycle] {
			t.Fatalf("cycle %d appears twice — one cycle would be double-counted", r.Cycle)
		}
		ids[r.Cycle] = true
	}
	if !ids[11] || !ids[12] {
		t.Errorf("ids = %v, want the filename identities 11 and 12", ids)
	}
}

// TestLoadRequiresACorpus pins the loud failure: an empty project root is an
// error, not an empty report.
func TestLoadRequiresACorpus(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("Load over a corpus-less root returned nil error, want a failure")
	}
}

// TestLoadAccountsForACorruptDossier pins conservation against the ugly case: a
// dossier that will not parse still occupies its slot, as no-data.
func TestLoadAccountsForACorruptDossier(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "knowledge-base", "cycles", "cycle-9.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	rows, err := Load(root)
	if err != nil {
		t.Fatalf("Load returned error %v, want nil", err)
	}
	if len(rows) != 1 || rows[0].Cycle != 9 {
		t.Fatalf("rows = %+v, want one row identified as cycle 9 from its filename", rows)
	}
	rep := Correlate(rows)
	if rep.CyclesJoined+len(rep.NoData) != 1 {
		t.Errorf("joined(%d)+no-data(%d) != 1 dossier — the corrupt file was dropped", rep.CyclesJoined, len(rep.NoData))
	}
}
