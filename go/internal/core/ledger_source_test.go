package core

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestLedgerEntrySource_FieldPresent(t *testing.T) {
	t.Parallel()
	f, ok := reflect.TypeOf(LedgerEntry{}).FieldByName("Source")
	if !ok {
		t.Fatal("LedgerEntry has no Source field — skip-source attribution missing")
	}
	if f.Type.Kind() != reflect.String {
		t.Errorf("LedgerEntry.Source kind = %s, want string", f.Type.Kind())
	}
	if tag := f.Tag.Get("json"); tag != "source,omitempty" {
		t.Errorf("LedgerEntry.Source json tag = %q, want \"source,omitempty\"", tag)
	}
}

func TestLedgerEntrySource_RouterStamp(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, led, buildRunners(nil))
	cs := CycleState{WorkspacePath: t.TempDir()}
	dec := router.RouterDecision{SkipPhases: []string{"bug-reproduction", "spec-verify"}}

	o.recordRoutingDecision(context.Background(), 230, cs, 1, dec)

	skipped := 0
	for _, e := range led.entries {
		if e.Kind != "phase_skipped" {
			continue
		}
		skipped++
		buf, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal phase_skipped entry: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(buf, &m); err != nil {
			t.Fatalf("unmarshal phase_skipped entry: %v", err)
		}
		if got, _ := m["source"].(string); got != "router" {
			t.Errorf("phase_skipped role=%s source = %q, want \"router\"", e.Role, got)
		}
	}
	if skipped != 2 {
		t.Fatalf("phase_skipped entries appended = %d, want 2 (one per SkipPhases element)", skipped)
	}
}

func TestLedgerEntrySource_RoundTrip(t *testing.T) {
	t.Parallel()
	var e LedgerEntry
	if err := json.Unmarshal([]byte(`{"ts":"2026-06-06T00:00:00Z","cycle":230,"role":"bug-reproduction","kind":"phase_skipped","source":"router","entry_seq":1,"prev_hash":"x"}`), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	buf, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if got, _ := m["source"].(string); got != "router" {
		t.Errorf("round-tripped source = %q, want \"router\" (UnmarshalJSON must route the field)", got)
	}

	// omitempty negative: no source in → no "source" key out.
	plain, err := json.Marshal(LedgerEntry{TS: "2026-06-06T00:00:00Z", Cycle: 230, Role: "scout", Kind: "phase_start"})
	if err != nil {
		t.Fatalf("marshal plain: %v", err)
	}
	var pm map[string]any
	if err := json.Unmarshal(plain, &pm); err != nil {
		t.Fatalf("unmarshal plain: %v", err)
	}
	if _, present := pm["source"]; present {
		t.Error("entry without source serialized a \"source\" key — omitempty missing, hash-chain bytes would shift")
	}
}
