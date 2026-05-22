package acsrunner

import (
	"encoding/json"
	"strings"
	"testing"
)

// ParseTestJSON consumes `go test -json` output and aggregates per-test
// verdicts into a Verdict struct. This is the pure parser; Run() in
// runner.go wraps it with go-test invocation.

func TestParseTestJSON_SimplePassFail(t *testing.T) {
	input := strings.Join([]string{
		`{"Time":"2026-05-22T10:00:00Z","Action":"run","Package":"acs/cycle-104","Test":"TestPredicateA"}`,
		`{"Time":"2026-05-22T10:00:00Z","Action":"pass","Package":"acs/cycle-104","Test":"TestPredicateA","Elapsed":0.123}`,
		`{"Time":"2026-05-22T10:00:00Z","Action":"run","Package":"acs/cycle-104","Test":"TestPredicateB"}`,
		`{"Time":"2026-05-22T10:00:00Z","Action":"output","Package":"acs/cycle-104","Test":"TestPredicateB","Output":"--- FAIL: TestPredicateB\n"}`,
		`{"Time":"2026-05-22T10:00:00Z","Action":"fail","Package":"acs/cycle-104","Test":"TestPredicateB","Elapsed":0.456}`,
	}, "\n") + "\n"
	v, err := ParseTestJSON(strings.NewReader(input), 104)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Cycle != 104 {
		t.Errorf("cycle=%d, want 104", v.Cycle)
	}
	if v.Total != 2 {
		t.Errorf("total=%d, want 2", v.Total)
	}
	if v.RedCount != 1 {
		t.Errorf("redCount=%d, want 1", v.RedCount)
	}
	if len(v.Predicates) != 2 {
		t.Fatalf("predicates=%d, want 2", len(v.Predicates))
	}
	// Find each by name.
	got := map[string]Predicate{}
	for _, p := range v.Predicates {
		got[p.Name] = p
	}
	if got["TestPredicateA"].Verdict != "PASS" {
		t.Errorf("A verdict=%q", got["TestPredicateA"].Verdict)
	}
	if got["TestPredicateA"].DurationMS != 123 {
		t.Errorf("A duration_ms=%d, want 123", got["TestPredicateA"].DurationMS)
	}
	if got["TestPredicateB"].Verdict != "FAIL" {
		t.Errorf("B verdict=%q", got["TestPredicateB"].Verdict)
	}
	if !strings.Contains(got["TestPredicateB"].Output, "--- FAIL") {
		t.Errorf("B output missing FAIL line: %q", got["TestPredicateB"].Output)
	}
}

func TestParseTestJSON_SkipsPackageLevelEvents(t *testing.T) {
	// Lines without Test name are package-level (run/pass for the
	// package itself) and must not be counted.
	input := strings.Join([]string{
		`{"Action":"start","Package":"acs/cycle-1"}`,
		`{"Action":"run","Package":"acs/cycle-1","Test":"TestX"}`,
		`{"Action":"pass","Package":"acs/cycle-1","Test":"TestX","Elapsed":0.001}`,
		`{"Action":"pass","Package":"acs/cycle-1","Elapsed":0.5}`,
	}, "\n") + "\n"
	v, err := ParseTestJSON(strings.NewReader(input), 1)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Total != 1 {
		t.Errorf("total=%d, want 1 (package-level pass must not count)", v.Total)
	}
}

func TestParseTestJSON_SkipAction(t *testing.T) {
	input := `{"Action":"run","Package":"x","Test":"TestS"}
{"Action":"skip","Package":"x","Test":"TestS","Elapsed":0}
`
	v, err := ParseTestJSON(strings.NewReader(input), 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Total != 1 {
		t.Errorf("total=%d, want 1", v.Total)
	}
	if v.RedCount != 0 {
		t.Errorf("redCount=%d, want 0 (skip is not red)", v.RedCount)
	}
	if v.Predicates[0].Verdict != "SKIP" {
		t.Errorf("verdict=%q, want SKIP", v.Predicates[0].Verdict)
	}
}

func TestParseTestJSON_InvalidLineIgnored(t *testing.T) {
	// Mixed valid + invalid; the parser should skip invalid lines
	// without bailing the whole verdict.
	input := strings.Join([]string{
		`not json`,
		`{"Action":"run","Package":"x","Test":"TestA"}`,
		`{"Action":"pass","Package":"x","Test":"TestA","Elapsed":0.1}`,
		``,
	}, "\n") + "\n"
	v, err := ParseTestJSON(strings.NewReader(input), 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Total != 1 || v.RedCount != 0 {
		t.Errorf("aggregate wrong: %+v", v)
	}
}

func TestVerdict_JSONShape(t *testing.T) {
	v := Verdict{
		Cycle:    42,
		Total:    3,
		RedCount: 1,
		Predicates: []Predicate{
			{Name: "TestA", Verdict: "PASS", DurationMS: 100},
			{Name: "TestB", Verdict: "FAIL", DurationMS: 200, Output: "msg"},
		},
	}
	raw, _ := json.Marshal(v)
	for _, want := range []string{
		`"cycle":42`,
		`"total":3`,
		`"red_count":1`,
		`"predicates"`,
		`"duration_ms":100`,
		`"verdict":"PASS"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("missing %q in: %s", want, raw)
		}
	}
}

func TestParseTestJSON_OutputAccumulates(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"run","Package":"x","Test":"T"}`,
		`{"Action":"output","Package":"x","Test":"T","Output":"line1\n"}`,
		`{"Action":"output","Package":"x","Test":"T","Output":"line2\n"}`,
		`{"Action":"fail","Package":"x","Test":"T","Elapsed":0.001}`,
	}, "\n") + "\n"
	v, _ := ParseTestJSON(strings.NewReader(input), 0)
	if len(v.Predicates) != 1 {
		t.Fatalf("preds=%d", len(v.Predicates))
	}
	if !strings.Contains(v.Predicates[0].Output, "line1") || !strings.Contains(v.Predicates[0].Output, "line2") {
		t.Errorf("output not accumulated: %q", v.Predicates[0].Output)
	}
}
