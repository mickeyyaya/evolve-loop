package filter

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var presentKeyValues = map[string][]string{
	"kind":           {"cycle.sealed", "loop.exit", "ship.landed", "ship.*", "*.exit"},
	"code":           {"LOOP_LOST", "SHIP_GATE_RED", "SHIP_*", "*_RED"},
	"module":         {"loop", "orchestrator", "ship"},
	"severity":       {"INFO", "WARN", "INCIDENT"},
	"cycle":          {"1", "2", "3"},
	"phase":          {"audit", "build"},
	"run_id":         {"run-1", "run-2"},
	"origin":         {"a.B", "c.D"},
	"attempt":        {"1", "2"},
	"pid":            {"0", "7", "9"},
	"seq":            {"0", "1", "5"},
	"source":         {"loop", "watch"},
	"fields.verdict": {"PASS", "FAIL"},
}

func drawPresentRecord(rt *rapid.T) Record {
	pick := func(key string) string {
		values := []string{}
		for _, v := range presentKeyValues[key] {
			if !strings.Contains(v, "*") {
				values = append(values, v)
			}
		}
		return rapid.SampledFrom(values).Draw(rt, key)
	}
	atoi := func(s string) int { return map[string]int{"0": 0, "1": 1, "2": 2, "3": 3, "5": 5, "7": 7, "9": 9}[s] }
	e := signalcenter.Event{
		Kind: signalcenter.Kind(pick("kind")), Code: signalcenter.Code(pick("code")),
		Module: signalcenter.Module(pick("module")), Severity: signalcenter.Severity(pick("severity")),
		Cycle: atoi(pick("cycle")), Phase: pick("phase"), RunID: pick("run_id"), Origin: pick("origin"),
		Attempt: atoi(pick("attempt")), PID: atoi(pick("pid")), Seq: uint64(atoi(pick("seq"))),
		Fields: map[string]string{"verdict": pick("fields.verdict")},
	}
	return Record{Source: pick("source"), Signal: &e}
}

func TestMatch_NotEqualIsTheComplementOfEqualForPresentKeys(t *testing.T) {
	keys := make([]string, 0, len(presentKeyValues))
	for k := range presentKeyValues {
		keys = append(keys, k)
	}
	rapid.Check(t, func(rt *rapid.T) {
		key := rapid.SampledFrom(keys).Draw(rt, "key")
		values := rapid.SliceOfNDistinct(rapid.SampledFrom(presentKeyValues[key]), 1, 3, rapid.ID[string]).Draw(rt, "values")
		rec := drawPresentRecord(rt)
		list := strings.Join(values, ",")
		eq, _, err := Parse(key+"="+list, testCatalog())
		if err != nil {
			rt.Fatalf("Parse(%s=%s): %v", key, list, err)
		}
		ne, _, err := Parse(key+"!="+list, testCatalog())
		if err != nil {
			rt.Fatalf("Parse(%s!=%s): %v", key, list, err)
		}
		if eq.Match(rec) == ne.Match(rec) {
			rt.Fatalf("%s=%s and %s!=%s both gave %v on a record where the key is present", key, list, key, list, eq.Match(rec))
		}
	})
}
