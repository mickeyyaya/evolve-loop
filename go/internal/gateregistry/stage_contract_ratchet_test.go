package gateregistry

import (
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

const genesisRequiredFloor = false

func mustRatchet(t *testing.T) Ratchet {
	t.Helper()
	ratchet, err := StageRatchet()
	if err != nil {
		t.Fatalf("StageRatchet: %v", err)
	}
	return ratchet
}

func metRules(violations []violation) func(string) []string {
	return func(key string) []string {
		broken := map[string]bool{}
		for _, v := range violations {
			if v.Key == key {
				broken[v.Rule] = true
			}
		}
		if broken[ruleNoSignal] {
			for _, rule := range signalRules {
				broken[rule] = true
			}
		}
		if broken[ruleNoEmitSite] {
			broken[ruleFieldMissing] = true
		}
		var met []string
		for _, rule := range allRules {
			if !broken[rule] {
				met = append(met, rule)
			}
		}
		return met
	}
}

var signalRules = []string{ruleUnregistered, ruleNoEmitSite, ruleNoWouldFields, ruleFieldMissing}

var allRules = []string{ruleNoEntry, ruleStringTyped, ruleNoSignal, ruleUnregistered, ruleNoEmitSite, ruleNoWouldFields, ruleFieldMissing}

func TestRegistryRatchet(t *testing.T) {
	t.Parallel()
	ratchet := mustRatchet(t)
	met := metRules(liveViolations(schemaStages(), StageEntries()))
	if problems := ratchetProblems(ratchet, StageEntries(), met); len(problems) > 0 {
		t.Errorf("the registry is looser than stage_contract_ratchet.json:\n%s", strings.Join(problems, "\n"))
	}
	if n := len(ratchet.Entries); n != len(StageEntries()) {
		t.Errorf("the ratchet lists %d entries, the registry has %d; add each new entry to the ratchet", n, len(StageEntries()))
	}
}

func baseRatchetEntry() RatchetEntry {
	return RatchetEntry{
		Key:       "k.stage",
		Rules:     []string{ruleNoEntry, ruleNoSignal},
		Signal:    "SIG",
		Fields:    []string{"a", "b"},
		EmitSites: []gatecontract.EmitSite{{Path: "p.go", Func: "f", Ref: "R"}},
		Promotion: gatecontract.Promotion{MaxEscapes: 2, MaxFalseStops: 2, MinDecisions: 5, WindowCycles: 10, MaxFalseStopPct: 20},
	}
}

func baseRegistryEntry() gatecontract.Entry {
	r := baseRatchetEntry()
	return gatecontract.Entry{Name: r.Key, Shadow: &gatecontract.Shadow{
		PolicyKey: r.Key, Signal: "SIG", Fields: slices.Clone(r.Fields), EmitSites: slices.Clone(r.EmitSites), Promotion: r.Promotion,
	}}
}

func TestRatchetProblems_NameEachWayTheRegistryLoosens(t *testing.T) {
	t.Parallel()
	allMet := func(string) []string { return []string{ruleNoEntry, ruleNoSignal} }
	cases := map[string]struct {
		mutate func(*gatecontract.Entry)
		met    func(string) []string
		want   string
	}{
		"tighter or equal passes": {func(e *gatecontract.Entry) {
			e.Shadow.Promotion = gatecontract.Promotion{MaxEscapes: 1, MaxFalseStops: 0, MinDecisions: 9, WindowCycles: 11, MaxFalseStopPct: 10}
			e.Shadow.Fields = append(e.Shadow.Fields, "c")
		}, allMet, ""},
		"entry removed":      {nil, allMet, "k.stage: the registry has no entry"},
		"rule lost":          {func(*gatecontract.Entry) {}, func(string) []string { return []string{ruleNoEntry} }, "no-signal"},
		"signal changed":     {func(e *gatecontract.Entry) { e.Shadow.Signal = "OTHER" }, allMet, "signal"},
		"field dropped":      {func(e *gatecontract.Entry) { e.Shadow.Fields = []string{"a"} }, allMet, "field b"},
		"emit site dropped":  {func(e *gatecontract.Entry) { e.Shadow.EmitSites = nil }, allMet, "emit site p.go f"},
		"max escapes up":     {func(e *gatecontract.Entry) { e.Shadow.Promotion.MaxEscapes = 3 }, allMet, "MaxEscapes"},
		"max false stops up": {func(e *gatecontract.Entry) { e.Shadow.Promotion.MaxFalseStops = 3 }, allMet, "MaxFalseStops"},
		"min decisions down": {func(e *gatecontract.Entry) { e.Shadow.Promotion.MinDecisions = 4 }, allMet, "MinDecisions"},
		"window down":        {func(e *gatecontract.Entry) { e.Shadow.Promotion.WindowCycles = 9 }, allMet, "WindowCycles"},
		"false stop pct up":  {func(e *gatecontract.Entry) { e.Shadow.Promotion.MaxFalseStopPct = 21 }, allMet, "MaxFalseStopPct"},
		"shadow removed":     {func(e *gatecontract.Entry) { e.Shadow = nil }, allMet, "k.stage: the registry has no entry"},
	}
	for name, c := range cases {
		var entries []gatecontract.Entry
		if c.mutate != nil {
			e := baseRegistryEntry()
			c.mutate(&e)
			entries = []gatecontract.Entry{e}
		}
		got := strings.Join(ratchetProblems(Ratchet{Entries: []RatchetEntry{baseRatchetEntry()}}, entries, c.met), "|")
		if c.want == "" && got != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s: problems = %q, want %q", name, got, c.want)
		}
	}
}

func TestGenesisRequiredIsOneWay(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		floor, current bool
		wantProblem    bool
	}{
		"false stays false": {false, false, false},
		"false to true":     {false, true, false},
		"true stays true":   {true, true, false},
		"true to false":     {true, false, true},
	}
	for name, c := range cases {
		if got := genesisFlipProblem(c.floor, c.current) != ""; got != c.wantProblem {
			t.Errorf("%s: problem = %v, want %v", name, got, c.wantProblem)
		}
	}
	if problem := genesisFlipProblem(genesisRequiredFloor, mustRatchet(t).GenesisRequired); problem != "" {
		t.Error(problem)
	}
}

func TestStageRatchet_HoldsTheMergedBaselineAndTheCeiling(t *testing.T) {
	t.Parallel()
	ratchet := mustRatchet(t)
	if ratchet.BaselineCeiling != len(ratchet.Violations) {
		t.Errorf("ceiling %d, baseline %d pairs", ratchet.BaselineCeiling, len(ratchet.Violations))
	}
}

func TestParseRatchet_RefusesMalformedJSON(t *testing.T) {
	t.Parallel()
	if _, err := parseRatchet([]byte("{")); err == nil {
		t.Error("parseRatchet accepted malformed JSON")
	}
	if _, err := parseRatchet([]byte(`{"baseline_ceiling":1,"unknown":1}`)); err == nil {
		t.Error("parseRatchet accepted an unknown field")
	}
}

func TestMetRules_SkipsTheRulesThatNeverRanForAKeyWithoutASignalOrASite(t *testing.T) {
	t.Parallel()
	met := metRules([]violation{{"a", ruleNoSignal}, {"b", ruleNoEmitSite}})
	if got := met("a"); !slices.Equal(got, []string{ruleNoEntry, ruleStringTyped}) {
		t.Errorf("a: %q", got)
	}
	if got := met("b"); slices.Contains(got, ruleFieldMissing) || !slices.Contains(got, ruleNoSignal) {
		t.Errorf("b: %q", got)
	}
	if got := met("c"); len(got) != len(allRules) {
		t.Errorf("c: %q", got)
	}
}

func TestRatchetViolation_NamesAKeyAndARule(t *testing.T) {
	t.Parallel()
	var first RatchetViolation = mustRatchet(t).Violations[0]
	if first.Key == "" || first.Rule == "" {
		t.Errorf("first baseline pair = %+v", first)
	}
}
