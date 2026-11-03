package inboxbatch

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestItem_DecodesTypedProbes(t *testing.T) {
	raw := `{"id":"a","probes":[{"kind":"test_exists","target":"TestA"},{"kind":"policy_value","target":"inbox.curator.stage","value":"enforce"}]}`
	var it Item
	if err := json.Unmarshal([]byte(raw), &it); err != nil {
		t.Fatal(err)
	}
	want := []Probe{{Kind: ProbeTestExists, Target: "TestA"}, {Kind: ProbePolicyValue, Target: "inbox.curator.stage", Value: "enforce"}}
	if !slices.Equal(it.Probes, want) {
		t.Errorf("probes = %+v, want %+v", it.Probes, want)
	}
}

func TestProbeKinds_ListsTheFourKindsInOrder(t *testing.T) {
	want := []ProbeKind{ProbeTestExists, ProbeSymbolExists, ProbeTextAbsent, ProbePolicyValue}
	if got := ProbeKinds(); !slices.Equal(got, want) {
		t.Errorf("ProbeKinds() = %v, want %v", got, want)
	}
}
