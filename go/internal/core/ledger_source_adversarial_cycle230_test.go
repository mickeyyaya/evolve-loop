package core

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestLedgerEntrySource_LargeSkipList_Amp(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, led, buildRunners(nil))
	cs := CycleState{WorkspacePath: t.TempDir()}
	dec := router.RouterDecision{
		SkipPhases: []string{
			"bug-reproduction",
			"spec-verify",
			"security-scan",
			"dependency-audit",
			"performance-check",
		},
	}

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
			t.Errorf("phase_skipped[%d] role=%s source=%q, want \"router\"", skipped, e.Role, got)
		}
	}
	if skipped != 5 {
		t.Fatalf("phase_skipped count = %d, want 5", skipped)
	}
}

func TestLedgerEntrySource_NonRouterSourcePreserved_Amp(t *testing.T) {
	t.Parallel()
	customSources := []string{"psmas", "content", "user-defined-value"}
	for _, src := range customSources {
		var e LedgerEntry
		raw := `{"ts":"2026-06-06T00:00:00Z","cycle":230,"role":"spec-verify","kind":"phase_skipped","source":"` + src + `","entry_seq":2,"prev_hash":"y"}`
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			t.Fatalf("unmarshal with source=%q: %v", src, err)
		}
		buf, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal with source=%q: %v", src, err)
		}
		var m map[string]any
		if err := json.Unmarshal(buf, &m); err != nil {
			t.Fatalf("re-unmarshal with source=%q: %v", src, err)
		}
		if got, _ := m["source"].(string); got != src {
			t.Errorf("source=%q: round-tripped value = %q, want %q (UnmarshalJSON must not hardcode \"router\")", src, got, src)
		}
	}
}

// Uses reflection (same pattern as ledger_source_test.go) so this file
// compiles even if the field is absent.
func TestLedgerEntrySource_DirectMarshal_Amp(t *testing.T) {
	t.Parallel()
	var e LedgerEntry
	v := reflect.ValueOf(&e).Elem()
	sf := v.FieldByName("Source")
	if !sf.IsValid() {
		t.Skip("Source field not present — covered by TestLedgerEntrySource_FieldPresent")
	}
	sf.SetString("router")

	buf, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if got, _ := m["source"].(string); got != "router" {
		t.Errorf("direct-marshal source = %q, want \"router\" (Source field must be serialized when non-empty)", got)
	}
}
