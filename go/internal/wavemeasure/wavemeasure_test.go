package wavemeasure_test

import (
	"encoding/json"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

var runStart = time.Date(2026, 10, 17, 6, 0, 0, 0, time.UTC)

func at(minutes int) time.Time { return runStart.Add(time.Duration(minutes) * time.Minute) }

func lane(from, to int) wavemeasure.Lane {
	return wavemeasure.Lane{Start: at(from), End: at(to), Landed: true}
}

func TestCompute_FourLanesGiveTheExactUtilizationAndTheTimeWeightedMedian(t *testing.T) {
	t.Parallel()
	f := wavemeasure.Facts{
		Start: at(0), End: at(100), WidthSized: 4,
		Lanes: []wavemeasure.Lane{lane(0, 60), lane(0, 40), lane(20, 100), lane(50, 90)},
	}

	got := wavemeasure.Compute(f, at(500))

	if got.Utilization != 0.55 {
		t.Errorf("utilization = %v, want 220 busy minutes / (4 slots x 100 minutes) = 0.55", got.Utilization)
	}
	if got.WidthRealized != 2 {
		t.Errorf("width_realized = %d, want 2: 10 minutes at 1, 60 at 2, 30 at 3 (the mean 2.2 is not the median)", got.WidthRealized)
	}
	if got.WidthSized != 4 || got.Landed != 4 || got.Conflicts != 0 || got.ConflictRate != 0 {
		t.Errorf("measures = %+v, want width_sized 4, 4 landed, no conflicts", got)
	}
}

func TestCompute_TheIdleTimeOfTheRunCountsAsWidthZero(t *testing.T) {
	t.Parallel()
	f := wavemeasure.Facts{Start: at(0), End: at(100), WidthSized: 2, Lanes: []wavemeasure.Lane{lane(0, 40), lane(10, 30)}}

	got := wavemeasure.Compute(f, at(500))

	if got.WidthRealized != 0 || got.Utilization != 0.3 {
		t.Errorf("measures = %+v, want width_realized 0 (60 of 100 minutes idle) and utilization 60/200 = 0.3", got)
	}
}

func TestCompute_ALandedLaneWithAFleetRebaseIsAConflict(t *testing.T) {
	t.Parallel()
	rebased := lane(0, 50)
	rebased.Conflict = true
	failedRebase := wavemeasure.Lane{Start: at(0), End: at(50), Conflict: true}
	f := wavemeasure.Facts{Start: at(0), End: at(50), WidthSized: 3, Lanes: []wavemeasure.Lane{rebased, lane(0, 50), failedRebase}}

	got := wavemeasure.Compute(f, at(500))

	if got.Conflicts != 1 || got.Landed != 2 || got.ConflictRate != 0.5 {
		t.Errorf("conflicts %d / landed %d = %v, want 1/2 = 0.5: a lane that did not land is not in the rate", got.Conflicts, got.Landed, got.ConflictRate)
	}
}

func TestCompute_AnEmptyRunGivesZerosNotNaN(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		facts    wavemeasure.Facts
		realized int
	}{
		"no facts":         {facts: wavemeasure.Facts{}},
		"no lanes":         {facts: wavemeasure.Facts{Start: at(0), End: at(60), WidthSized: 3}},
		"no width":         {facts: wavemeasure.Facts{Start: at(0), End: at(60), Lanes: []wavemeasure.Lane{lane(0, 60)}}, realized: 1},
		"no run time":      {facts: wavemeasure.Facts{Start: at(0), End: at(0), WidthSized: 3, Lanes: []wavemeasure.Lane{lane(0, 0)}}},
		"lane before run":  {facts: wavemeasure.Facts{Start: at(60), End: at(120), WidthSized: 1, Lanes: []wavemeasure.Lane{lane(0, 30)}}},
		"lane of no start": {facts: wavemeasure.Facts{Start: at(0), End: at(60), WidthSized: 1, Lanes: []wavemeasure.Lane{{End: at(30), Landed: true}}}},
	} {
		got := wavemeasure.Compute(tc.facts, at(0))
		for field, v := range map[string]float64{"utilization": got.Utilization, "conflict_rate": got.ConflictRate} {
			if math.IsNaN(v) || math.IsInf(v, 0) || v != 0 {
				t.Errorf("%s: %s = %v, want 0", name, field, v)
			}
		}
		if got.WidthRealized != tc.realized {
			t.Errorf("%s: width_realized = %d, want %d", name, got.WidthRealized, tc.realized)
		}
		if _, err := json.Marshal(got); err != nil {
			t.Errorf("%s: the measures do not encode: %v", name, err)
		}
	}
}

func TestCompute_ALiveRunAndALaneInFlightEndAtTheClock(t *testing.T) {
	t.Parallel()
	inFlight := wavemeasure.Lane{Start: at(0)}
	f := wavemeasure.Facts{Start: at(0), WidthSized: 2, Lanes: []wavemeasure.Lane{inFlight, lane(0, 20)}}

	got := wavemeasure.Compute(f, at(40))

	if got.Utilization != 0.75 || got.WidthRealized != 1 || got.Landed != 1 {
		t.Errorf("measures = %+v, want utilization (40+20)/(2x40) = 0.75, width_realized 1, 1 landed", got)
	}
}

func TestCompute_ALaneIsClippedToTheRunWindow(t *testing.T) {
	t.Parallel()
	f := wavemeasure.Facts{Start: at(10), End: at(30), WidthSized: 1, Lanes: []wavemeasure.Lane{lane(0, 60)}}

	got := wavemeasure.Compute(f, at(500))

	if got.Utilization != 1 || got.WidthRealized != 1 {
		t.Errorf("measures = %+v, want utilization 1 and width_realized 1 inside the run window", got)
	}
}

func TestCompute_CopiesTheCountedSignals(t *testing.T) {
	t.Parallel()
	got := wavemeasure.Compute(wavemeasure.Facts{AdmissionWaits: 5}, at(0))

	if got.AdmissionWaits != 5 {
		t.Errorf("measures = %+v, want admission_waits 5", got)
	}
}

func TestMeasures_TheJSONFieldNamesAreStable(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(wavemeasure.Measures{})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(fields))
	for k := range fields {
		got = append(got, k)
	}
	sort.Strings(got)

	want := []string{"admission_waits", "conflict_rate", "conflicts", "landed", "optimistic_admissions", "optimistic_failure_rate", "optimistic_failures", "utilization", "width_realized", "width_sized"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JSON fields = %v, want %v", got, want)
	}
}

func TestNames_MatchTheSpecSignalsAndTheSizingField(t *testing.T) {
	t.Parallel()
	for got, want := range map[string]string{
		wavemeasure.CodeAdmitWait:   "LOOP_ORCH_ADMIT_WAIT",
		wavemeasure.FieldWidthSized: "width_sized",
	} {
		if got != want {
			t.Errorf("name = %q, want %q (wave-orchestrator.md §11 and §13)", got, want)
		}
	}
}

func TestCompute_CarriesACopyOfTheClassCountsOfTheLastCensusAndOmitsAbsentOnes(t *testing.T) {
	t.Parallel()
	counts := wavemeasure.Supply{Admissible: 3, ConsoleProtected: 40}

	got := wavemeasure.Compute(wavemeasure.Facts{Supply: &counts}, at(0))
	counts.Admissible = 99

	if got.Supply == nil || *got.Supply != (wavemeasure.Supply{Admissible: 3, ConsoleProtected: 40}) {
		t.Errorf("supply = %+v, want the counts as the census left them: Compute must copy", got.Supply)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var fields struct {
		Supply map[string]int `json:"supply"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields.Supply) != 6 || fields.Supply["console_protected"] != 40 {
		t.Errorf("JSON = %s, %v, want a supply object with the six class names", raw, err)
	}
	empty := wavemeasure.Compute(wavemeasure.Facts{}, at(0))
	if rawEmpty, _ := json.Marshal(empty); empty.Supply != nil || strings.Contains(string(rawEmpty), "supply") {
		t.Errorf("a run with no census gave %s, want no supply key", rawEmpty)
	}
}
