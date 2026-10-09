package filter

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestMatch_SeverityOrdersInfoWarnIncident(t *testing.T) {
	cases := []struct {
		expr string
		want map[signalcenter.Severity]bool
	}{
		{"severity>=WARN", map[signalcenter.Severity]bool{"INFO": false, "WARN": true, "INCIDENT": true}},
		{"severity>WARN", map[signalcenter.Severity]bool{"INFO": false, "WARN": false, "INCIDENT": true}},
		{"severity<=WARN", map[signalcenter.Severity]bool{"INFO": true, "WARN": true, "INCIDENT": false}},
		{"severity<WARN", map[signalcenter.Severity]bool{"INFO": true, "WARN": false, "INCIDENT": false}},
		{"severity=WARN", map[signalcenter.Severity]bool{"INFO": false, "WARN": true, "INCIDENT": false}},
		{"severity!=WARN", map[signalcenter.Severity]bool{"INFO": true, "WARN": false, "INCIDENT": true}},
	}
	for _, tc := range cases {
		f := mustParse(t, tc.expr)
		for sev, want := range tc.want {
			e := sealedEvent()
			e.Severity = sev
			if got := f.Match(signalRecord(e)); got != want {
				t.Errorf("%s on severity %s = %v, want %v", tc.expr, sev, got, want)
			}
		}
	}
}

func TestMatch_NumbersCompareAsNumbers(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{"cycle=01841", true},
		{"cycle>999", true},
		{"cycle<1841", false},
		{"cycle<=1841", true},
		{"attempt>=3", false},
		{"pid=9055", true},
		{"seq>40", true},
		{"seq<41", false},
	}
	for _, tc := range cases {
		if got := mustParse(t, tc.expr).Match(signalRecord(sealedEvent())); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.expr, got, tc.want)
		}
	}
}

func TestMatch_EachKeyReadsItsOwnField(t *testing.T) {
	cases := []string{
		"kind=cycle.sealed", "code=SHIP_GATE_RED", "module=orchestrator", "severity=WARN",
		"cycle=1841", "phase=audit", "run_id=run-7", "origin=cycleRun.completeCycle",
		"attempt=2", "pid=9055", "seq=41", "source=loop", "fields.final_verdict=PASS",
	}
	for _, expr := range cases {
		f := mustParse(t, expr)
		if !f.Match(signalRecord(sealedEvent())) {
			t.Errorf("%s must match the event that holds that value", expr)
		}
		other := sealedEvent()
		other.Kind, other.Code, other.Module, other.Severity = "loop.exit", "LOOP_LOST", "loop", "INFO"
		other.Cycle, other.Phase, other.RunID, other.Origin = 7, "build", "run-8", "other"
		other.Attempt, other.PID, other.Seq = 5, 1, 2
		other.Fields = map[string]string{"final_verdict": "FAIL"}
		rec := signalRecord(other)
		rec.Source = "ship"
		if f.Match(rec) {
			t.Errorf("%s must not match an event with other values", expr)
		}
	}
}

func TestMatch_AnAbsentKeyMatchesOnlyNotEqual(t *testing.T) {
	absent := signalcenter.Event{Module: "loop", Kind: "loop.exit", Severity: "INFO"}
	rec := Record{Signal: &absent}
	for _, key := range []string{"cycle", "attempt"} {
		for _, op := range []string{"=", ">=", ">", "<=", "<"} {
			if mustParse(t, key+op+"0").Match(rec) {
				t.Errorf("%s%s0 matched a record whose %s is 0 (absent)", key, op, key)
			}
		}
		if !mustParse(t, key+"!=0").Match(rec) {
			t.Errorf("%s!=0 must match a record whose %s is absent", key, key)
		}
	}
	for _, term := range []string{"code=SHIP_GATE_RED", "phase=x", "run_id=x", "origin=x", "source=x", "fields.final_verdict=x"} {
		key, val, _ := strings.Cut(term, "=")
		if mustParse(t, term).Match(rec) {
			t.Errorf("%s matched a record without %s", term, key)
		}
		if !mustParse(t, key+"!="+val).Match(rec) {
			t.Errorf("%s!=%s must match a record without %s", key, val, key)
		}
	}
	if mustParse(t, "code=*").Match(rec) {
		t.Error("code=* matched a record without a code")
	}
	noSeverity := Record{Signal: &signalcenter.Event{Kind: "loop.exit"}}
	if mustParse(t, "severity<WARN").Match(noSeverity) || !mustParse(t, "severity!=WARN").Match(noSeverity) {
		t.Error("an absent severity must match only !=")
	}
	if !mustParse(t, "pid=0 seq=0").Match(rec) {
		t.Error("pid and seq are always present, so 0 is a value")
	}
}

func TestMatch_GapRecordsPassEveryFilter(t *testing.T) {
	for _, expr := range []string{"", "kind=loop.exit", "severity>=INCIDENT", "cycle=1 module!=loop", "fields.x=y"} {
		if !mustParse(t, expr).Match(gapRecord()) {
			t.Errorf("filter %q refused a gap record; a gap must never be silent", expr)
		}
	}
}

func TestUntil_AGapRecordNeverMatches(t *testing.T) {
	for _, expr := range []string{"", "kind=loop.exit", "kind!=loop.exit"} {
		if mustParse(t, expr).Until(gapRecord()) {
			t.Errorf("until %q was satisfied by a gap record", expr)
		}
	}
	lost := signalcenter.Event{Module: "loop", Kind: "loop.lost", Code: "LOOP_LOST", Severity: "INCIDENT"}
	until := mustParse(t, "kind=loop.lost")
	if !until.Until(Record{Source: "watch", Signal: &lost}) {
		t.Error("a synthetic loop.lost record must satisfy --until kind=loop.lost")
	}
	if until.Until(signalRecord(sealedEvent())) {
		t.Error("a signal that the filter refuses must not satisfy --until")
	}
}

func TestMatch_AnInvalidSeverityIsAbsent(t *testing.T) {
	invalid := sealedEvent()
	invalid.Severity = "DEBUG"
	rec := signalRecord(invalid)
	for _, expr := range []string{"severity<WARN", "severity<=INCIDENT", "severity>=INFO", "severity=INFO,WARN,INCIDENT"} {
		if mustParse(t, expr).Match(rec) {
			t.Errorf("%s matched a record whose severity DEBUG is not a tier", expr)
		}
	}
	if !mustParse(t, "severity!=INFO").Match(rec) {
		t.Error("severity!=INFO must match a record whose severity DEBUG is not a tier")
	}
}
