package qualitypanel

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func shipRateReading(k, n int) Reading {
	return Reading{Index: "PQ1", Interval: Wilson(k, n), Method: MethodWilson95, N: n, Window: "waves:136-140", Source: ".evolve/runs"}
}

func TestEvents_AKnownKAndNEmitsAReadingWithTheExactWilsonBounds(t *testing.T) {
	t.Parallel()

	events := Events(shipRateReading(3, 5), StageShadow)

	if len(events) != 1 {
		t.Fatalf("events = %d, want one quality.reading: %+v", len(events), events)
	}
	e := events[0]
	if e.Module != signalcenter.ModuleQuality || e.Kind != signalcenter.KindQualityReading || e.Severity != signalcenter.SeverityInfo || e.Code != "" {
		t.Errorf("event = %s/%s/%s code %q, want quality/quality.reading/INFO with no code", e.Module, e.Kind, e.Severity, e.Code)
	}
	want := map[string]string{"index": "PQ1", "value": "0.600", "lo": "0.231", "hi": "0.882", "method": "wilson95", "n": "5", "window": "waves:136-140", "stage": "shadow", "threshold": "0.400", "status": "inconclusive", "source": ".evolve/runs"}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q", k, e.Fields[k], v)
		}
	}
	if len(e.Fields) != len(want) {
		t.Errorf("fields = %v, want exactly the %d keys of spec §5.2", e.Fields, len(want))
	}
}

func TestEvents_StageDecidesWhetherABreachIsSignalled(t *testing.T) {
	t.Parallel()
	breach := Reading{Index: "PQ1", Interval: Wilson(5, 60), Method: MethodWilson95, N: 60, Window: "w", Source: "s"}
	cases := []struct {
		stage      Stage
		wantBreach bool
	}{{StageShadow, false}, {StageReport, true}, {StageGate, true}}
	for _, c := range cases {
		events := Events(breach, c.stage)
		got := len(events) == 2 && events[1].Kind == signalcenter.KindQualityBreach
		if got != c.wantBreach {
			t.Errorf("stage %s: events %+v, want breach=%v", c.stage, events, c.wantBreach)
		}
		if got && (events[1].Code != "QUALITY_PQ1_BREACH" || events[1].Severity != signalcenter.SeverityWarn || events[1].Fields["status"] != "breach") {
			t.Errorf("stage %s: breach event %+v, want WARN QUALITY_PQ1_BREACH with status breach", c.stage, events[1])
		}
	}
}

func TestEvents_AnIntervalAcrossTheThresholdIsInconclusiveAndNeverABreach(t *testing.T) {
	t.Parallel()

	events := Events(Reading{Index: "PQ1", Interval: Interval{Value: 0.38, Lo: 0.30, Hi: 0.47}, Method: MethodWilson95, N: 80}, StageReport)

	if len(events) != 1 || events[0].Fields["status"] != "inconclusive" {
		t.Errorf("events = %+v, want one reading with status inconclusive", events)
	}
}

func TestEvents_AnEmptySampleWarnsVacuousDenominator(t *testing.T) {
	t.Parallel()

	events := Events(Reading{Index: "JQ3", Method: MethodWilson95, Window: "reviews:none", Source: "reviews"}, StageReport)

	if len(events) != 2 || events[0].Fields["status"] != "no_data" {
		t.Fatalf("events = %+v, want a no_data reading and a warning", events)
	}
	w := events[1]
	if w.Kind != signalcenter.KindQualityWarning || w.Code != CodeVacuousDenominator || w.Severity != signalcenter.SeverityWarn || w.Fields["index"] != "JQ3" {
		t.Errorf("warning = %+v, want WARN quality.warning QUALITY_VACUOUS_DENOMINATOR for JQ3", w)
	}
}

func TestEvents_AnIndexWithNoThresholdNeverBreaches(t *testing.T) {
	t.Parallel()

	events := Events(Reading{Index: "JQ4", Interval: Wilson(0, 234), Method: MethodWilson95, N: 234}, StageGate)

	if len(events) != 1 || events[0].Fields["status"] != "ok" || events[0].Fields["threshold"] != "" {
		t.Errorf("events = %+v, want one ok reading with an empty threshold", events)
	}
}

func TestEvents_ACensusHasNoIntervalAndKeepsCycleAndOrigin(t *testing.T) {
	t.Parallel()

	e := Events(Reading{Index: "CQ1", Interval: Interval{Value: 0.888}, Method: MethodCensus, N: 11296, Cycle: 2028, Origin: "qualitypanel.UnitConformance"}, StageReport)[0]

	if e.Fields["lo"] != "" || e.Fields["hi"] != "" || e.Fields["value"] != "0.888" || e.Fields["status"] != "breach" {
		t.Errorf("fields = %v, want a census with empty bounds that breaches 0.900", e.Fields)
	}
	if e.Cycle != 2028 || e.Origin != "qualitypanel.UnitConformance" {
		t.Errorf("cycle %d origin %q, want 2028 and the reader's origin", e.Cycle, e.Origin)
	}
}

func TestEvents_EveryFieldSurvivesNormalize(t *testing.T) {
	t.Parallel()

	for _, e := range Events(Reading{Index: "PQ4", Interval: Wilson(31, 186), Method: MethodWilson95, N: 186, Window: "waves:82-140", Source: ".evolve/runs"}, StageReport) {
		got, drift := signalcenter.Normalize(e)
		if len(drift) != 0 || len(got.Fields) != len(e.Fields) || len(e.Fields) > signalcenter.MaxFields-1 {
			t.Errorf("normalize drift %v, fields %d -> %d, want no drift, no loss and at most %d keys", drift, len(e.Fields), len(got.Fields), signalcenter.MaxFields-1)
		}
	}
}

func TestJudge_DirectionAndSampleSize(t *testing.T) {
	t.Parallel()
	lower, _ := Lookup("PQ4")
	higher, _ := Lookup("PQ1")
	cases := []struct {
		name string
		ix   Index
		r    Reading
		want Status
	}{
		{"lower breach", lower, Reading{N: 100, Interval: Interval{Lo: 0.21, Hi: 0.30}}, StatusBreach},
		{"lower ok", lower, Reading{N: 100, Interval: Interval{Lo: 0.10, Hi: 0.20}}, StatusOK},
		{"lower across", lower, Reading{N: 100, Interval: Interval{Lo: 0.15, Hi: 0.25}}, StatusInconclusive},
		{"higher ok", higher, Reading{N: 100, Interval: Interval{Lo: 0.40, Hi: 0.60}}, StatusOK},
		{"breach under n_min", higher, Reading{N: 39, Interval: Interval{Lo: 0.10, Hi: 0.30}}, StatusInconclusive},
		{"breach at n_min", higher, Reading{N: 40, Interval: Interval{Lo: 0.10, Hi: 0.30}}, StatusBreach},
	}
	for _, c := range cases {
		if got := Judge(c.ix, c.r); got != c.want {
			t.Errorf("%s: Judge = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestSourceWarning_NamesTheIndexAndTheSource(t *testing.T) {
	t.Parallel()

	e := SourceWarning("PQ1", CodeSourceAbsent, ".evolve/waves", "no wave records")

	if e.Kind != signalcenter.KindQualityWarning || e.Code != CodeSourceAbsent || e.Severity != signalcenter.SeverityWarn || e.Fields["index"] != "PQ1" || e.Fields["source"] != ".evolve/waves" || e.Reason != "no wave records" {
		t.Errorf("warning = %+v", e)
	}
}

func TestEvents_AnUnknownIndexIsAWarningNotAReading(t *testing.T) {
	t.Parallel()

	events := Events(Reading{Index: "ZQ9", N: 4, Source: "fixture"}, StageReport)

	if len(events) != 1 || events[0].Kind != signalcenter.KindQualityWarning || events[0].Code != CodeSourceUnreadable || events[0].Fields["index"] != "ZQ9" {
		t.Errorf("events = %+v, want one QUALITY_SOURCE_UNREADABLE warning naming ZQ9", events)
	}
}
