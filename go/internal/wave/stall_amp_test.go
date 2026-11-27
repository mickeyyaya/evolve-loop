package wave_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestAmpStallEvent_StringNamesTriggerLanesAndEvidence(t *testing.T) {
	ev := wave.Event{
		Kind:     wave.EventStall,
		Trigger:  "lane-stalls",
		Lanes:    []string{"lane-a", "lane-b"},
		Evidence: []string{".evolve/signals.ndjson"},
	}
	got := ev.String()
	for _, want := range []string{"lane-stalls", "lane-a", "lane-b", ".evolve/signals.ndjson"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, missing %q", got, want)
		}
	}
}

func TestAmpStallEvent_EmptyLanesAndEvidenceDoNotPanic(t *testing.T) {
	ev := wave.Event{Kind: wave.EventStall, Trigger: "no-ship"}
	if got := ev.String(); !strings.Contains(got, "no-ship") {
		t.Errorf("String() = %q, missing trigger", got)
	}
}

func TestAmpStallEvent_JSONUsesContractFieldNames(t *testing.T) {
	ev := wave.Event{
		Kind:     wave.EventStall,
		Trigger:  "abnormal-kind",
		Lanes:    []string{"cycle-7"},
		Evidence: []string{"a/abnormal-events.jsonl"},
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["kind"] != "stall" || m["trigger"] != "abnormal-kind" {
		t.Errorf("json = %s", raw)
	}
	for _, k := range []string{"lanes", "evidence"} {
		if _, ok := m[k]; !ok {
			t.Errorf("json %s lacks %q", raw, k)
		}
	}
}

func TestAmpNonStallEvent_JSONOmitsStallFields(t *testing.T) {
	raw, err := json.Marshal(wave.Event{Kind: wave.EventPhase, Cycle: 3, Detail: "scout"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"trigger", "lanes", "evidence"} {
		if _, ok := m[k]; ok {
			t.Errorf("non-stall json %s has %q", raw, k)
		}
	}
}
