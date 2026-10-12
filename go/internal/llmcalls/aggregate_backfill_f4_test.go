package llmcalls

import (
	"sort"
	"testing"
)

func TestLessPerformance_EachFieldBreaksTheTieOfTheOnesBeforeIt(t *testing.T) {
	base := Performance{CLI: "c", Model: "m", RequestedModel: "r", DispatchSource: "d", MeasurementSource: "s", TimingScope: "t", Outcome: "o"}
	vary := map[string]func(Performance) Performance{
		"CLI":               func(p Performance) Performance { p.CLI = "d"; return p },
		"Model":             func(p Performance) Performance { p.Model = "n"; return p },
		"RequestedModel":    func(p Performance) Performance { p.RequestedModel = "s"; return p },
		"DispatchSource":    func(p Performance) Performance { p.DispatchSource = "e"; return p },
		"MeasurementSource": func(p Performance) Performance { p.MeasurementSource = "u"; return p },
		"TimingScope":       func(p Performance) Performance { p.TimingScope = "u"; return p },
		"Outcome":           func(p Performance) Performance { p.Outcome = "p"; return p },
	}
	for field, bump := range vary {
		larger := bump(base)
		if !lessPerformance(base, larger) {
			t.Errorf("%s: a smaller %s must sort first", field, field)
		}
		if lessPerformance(larger, base) {
			t.Errorf("%s: a larger %s must not sort first", field, field)
		}
	}
	if lessPerformance(base, base) {
		t.Error("an equal pair is not less than itself")
	}
}

func TestLessPerformance_EarlierFieldsDominateLaterOnes(t *testing.T) {
	a := Performance{CLI: "a", Model: "z", Outcome: "z"}
	b := Performance{CLI: "b", Model: "a", Outcome: "a"}
	if !lessPerformance(a, b) || lessPerformance(b, a) {
		t.Fatal("CLI decides before Model and Outcome")
	}
	recs := []Performance{
		{CLI: "x", Model: "2", Outcome: "ok"},
		{CLI: "x", Model: "1", Outcome: "ok"},
		{CLI: "a", Model: "9", Outcome: "ok"},
		{CLI: "x", Model: "1", Outcome: "failed"},
	}
	sort.Slice(recs, func(i, j int) bool { return lessPerformance(recs[i], recs[j]) })
	want := []string{"a/9/ok", "x/1/failed", "x/1/ok", "x/2/ok"}
	for i, r := range recs {
		if got := r.CLI + "/" + r.Model + "/" + r.Outcome; got != want[i] {
			t.Errorf("position %d = %s, want %s", i, got, want[i])
		}
	}
}
