package gateregistry

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

//go:embed stage_contract_ratchet.json
var stageContractRatchetJSON []byte

type RatchetEntry struct {
	Key       string                  `json:"key"`
	Rules     []string                `json:"rules"`
	Signal    string                  `json:"signal"`
	Fields    []string                `json:"fields"`
	EmitSites []gatecontract.EmitSite `json:"emit_sites"`
	Promotion gatecontract.Promotion  `json:"promotion"`
}

type RatchetViolation struct {
	Key  string `json:"key"`
	Rule string `json:"rule"`
}

type Ratchet struct {
	GenesisRequired bool               `json:"genesis_required"`
	BaselineCeiling int                `json:"baseline_ceiling"`
	Entries         []RatchetEntry     `json:"entries"`
	Violations      []RatchetViolation `json:"violations"`
}

func StageRatchet() (Ratchet, error) {
	return parseRatchet(stageContractRatchetJSON)
}

func parseRatchet(raw []byte) (Ratchet, error) {
	var r Ratchet
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return Ratchet{}, fmt.Errorf("gateregistry: stage_contract_ratchet.json: %w", err)
	}
	return r, nil
}

func ratchetProblems(r Ratchet, entries []gatecontract.Entry, met func(string) []string) []string {
	byKey := entryByKey(entries)
	var out []string
	for _, want := range r.Entries {
		entry, ok := byKey[want.Key]
		if !ok {
			out = append(out, want.Key+": the registry has no entry")
			continue
		}
		out = append(out, ratchetEntryProblems(want, *entry.Shadow, met(want.Key))...)
	}
	return out
}

func ratchetEntryProblems(want RatchetEntry, got gatecontract.Shadow, met []string) []string {
	var out []string
	for _, rule := range want.Rules {
		if !slices.Contains(met, rule) {
			out = append(out, want.Key+": the rule "+rule+" no longer holds")
		}
	}
	if want.Signal != "" && string(got.Signal) != want.Signal {
		out = append(out, fmt.Sprintf("%s: signal %q, the ratchet names %q", want.Key, got.Signal, want.Signal))
	}
	for _, field := range want.Fields {
		if !slices.Contains(got.Fields, field) {
			out = append(out, want.Key+": lost the field "+field)
		}
	}
	for _, site := range want.EmitSites {
		if !slices.Contains(got.EmitSites, site) {
			out = append(out, fmt.Sprintf("%s: lost the emit site %s %s", want.Key, site.Path, site.Func))
		}
	}
	return append(out, promotionProblems(want.Key, want.Promotion, got.Promotion)...)
}

func promotionProblems(key string, want, got gatecontract.Promotion) []string {
	var out []string
	for _, c := range []struct {
		name        string
		want, got   int
		mayOnlyFall bool
	}{
		{"MaxEscapes", want.MaxEscapes, got.MaxEscapes, true},
		{"MaxFalseStops", want.MaxFalseStops, got.MaxFalseStops, true},
		{"MaxFalseStopPct", want.MaxFalseStopPct, got.MaxFalseStopPct, true},
		{"MinDecisions", want.MinDecisions, got.MinDecisions, false},
		{"WindowCycles", want.WindowCycles, got.WindowCycles, false},
	} {
		if c.mayOnlyFall && c.got > c.want || !c.mayOnlyFall && c.got < c.want {
			out = append(out, fmt.Sprintf("%s: %s moved from %d to %d, the wrong way", key, c.name, c.want, c.got))
		}
	}
	return out
}

func genesisFlipProblem(floor, current bool) string {
	if floor && !current {
		return "genesis_required went from true to false; it may only go from false to true"
	}
	return ""
}

func entryByKey(entries []gatecontract.Entry) map[string]gatecontract.Entry {
	byKey := map[string]gatecontract.Entry{}
	for _, e := range entries {
		if e.Shadow != nil {
			byKey[e.Shadow.PolicyKey] = e
		}
	}
	return byKey
}
