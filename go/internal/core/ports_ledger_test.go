package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestLedgerEntry_Unmarshal_IntCycle is the existing happy path: cycle
// arrives as a JSON number, populates Cycle, leaves CycleLabel empty.
func TestLedgerEntry_Unmarshal_IntCycle(t *testing.T) {
	raw := `{"ts":"2026-05-26T00:00:00Z","cycle":107,"role":"build","kind":"phase","exit_code":0,"entry_seq":1865,"prev_hash":"abc"}`
	var e LedgerEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if e.Cycle != 107 {
		t.Errorf("Cycle = %d, want 107", e.Cycle)
	}
	if e.CycleLabel != "" {
		t.Errorf("CycleLabel = %q, want empty (int form)", e.CycleLabel)
	}
}

// TestLedgerEntry_Unmarshal_StringCycle covers the legacy malformed
// entry at .evolve/ledger.jsonl line 1741 from the v10.16.0 manual
// release. cycle="manual-release-v10.16.0" must parse without error and
// land in CycleLabel; Cycle stays 0.
func TestLedgerEntry_Unmarshal_StringCycle(t *testing.T) {
	raw := `{"ts":"2026-05-20T04:15:01Z","cycle":"manual-release-v10.16.0","role":"auditor","kind":"agent_subprocess","exit_code":0,"entry_seq":1740,"prev_hash":"4f288f60"}`
	var e LedgerEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("unmarshal of string-cycle entry must succeed (this is the cycle-107 dispatcher bug), got: %v", err)
	}
	if e.Cycle != 0 {
		t.Errorf("Cycle = %d, want 0 when cycle field is a string", e.Cycle)
	}
	if e.CycleLabel != "manual-release-v10.16.0" {
		t.Errorf("CycleLabel = %q, want %q", e.CycleLabel, "manual-release-v10.16.0")
	}
	if e.EntrySeq != 1740 {
		t.Errorf("EntrySeq = %d, want 1740", e.EntrySeq)
	}
}

// TestLedgerEntry_Unmarshal_ExplicitCycleLabel covers the canonical
// new-writer convention: numeric cycle: 0 + explicit cycle_label field.
func TestLedgerEntry_Unmarshal_ExplicitCycleLabel(t *testing.T) {
	raw := `{"ts":"2026-06-01T00:00:00Z","cycle":0,"cycle_label":"manual-release-v12.2.0","role":"auditor","exit_code":0,"entry_seq":2000,"prev_hash":"deadbeef"}`
	var e LedgerEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if e.Cycle != 0 {
		t.Errorf("Cycle = %d, want 0", e.Cycle)
	}
	if e.CycleLabel != "manual-release-v12.2.0" {
		t.Errorf("CycleLabel = %q, want %q", e.CycleLabel, "manual-release-v12.2.0")
	}
}

// TestLedgerEntry_Marshal_NormalEntry confirms normal entries serialize
// WITHOUT a cycle_label field (omitempty).
func TestLedgerEntry_Marshal_NormalEntry(t *testing.T) {
	e := LedgerEntry{Cycle: 107, Role: "build", EntrySeq: 1865, PrevHash: "abc"}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"cycle":107`) {
		t.Errorf("missing numeric cycle in %s", b)
	}
	if strings.Contains(string(b), "cycle_label") {
		t.Errorf("CycleLabel must be omitempty when unset, got: %s", b)
	}
}

// TestLedgerEntry_Marshal_LabeledEntry confirms manual entries written
// with the new convention round-trip (int cycle + explicit label).
func TestLedgerEntry_Marshal_LabeledEntry(t *testing.T) {
	e := LedgerEntry{Cycle: 0, CycleLabel: "manual-release-v12.2.0", Role: "auditor", EntrySeq: 2000, PrevHash: "deadbeef"}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"cycle":0`) {
		t.Errorf("expected numeric cycle:0, got %s", b)
	}
	if !strings.Contains(string(b), `"cycle_label":"manual-release-v12.2.0"`) {
		t.Errorf("expected cycle_label in output, got %s", b)
	}
}

// TestLedgerEntry_RoundTrip_StringCycle_NormalizesToLabel: read a legacy
// string-cycle entry, then marshal it back. The result is the new
// canonical form (cycle:0 + cycle_label) — NOT the original string form
// — so future writers and consumers see normalized data.
func TestLedgerEntry_RoundTrip_StringCycle_NormalizesToLabel(t *testing.T) {
	raw := `{"ts":"2026-05-20T04:15:01Z","cycle":"manual-release-v10.16.0","role":"auditor","exit_code":0,"entry_seq":1740,"prev_hash":"abc"}`
	var e LedgerEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, `"cycle":0`) {
		t.Errorf("normalized form must use cycle:0, got %s", got)
	}
	if !strings.Contains(got, `"cycle_label":"manual-release-v10.16.0"`) {
		t.Errorf("normalized form must carry cycle_label, got %s", got)
	}
}

// TestLedgerEntry_Unmarshal_MalformedCycle exercises non-int, non-string
// values for the cycle field (e.g., null, object). These must surface
// as errors so corrupt entries aren't silently absorbed.
func TestLedgerEntry_Unmarshal_MalformedCycle(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"object cycle", `{"cycle":{"nested":true},"role":"x"}`},
		{"array cycle", `{"cycle":[1,2,3],"role":"x"}`},
		{"bool cycle", `{"cycle":true,"role":"x"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var e LedgerEntry
			if err := json.Unmarshal([]byte(tc.raw), &e); err == nil {
				t.Errorf("expected error for cycle=%s, got nil (Cycle=%d, Label=%q)", tc.raw, e.Cycle, e.CycleLabel)
			}
		})
	}
}

// TestLedgerEntry_Unmarshal_FloatCycle: JSON numbers can be float in
// theory. Real ledger entries never use floats, but the unmarshaler
// should accept whole-number floats (102.0 → 102) and reject fractional.
func TestLedgerEntry_Unmarshal_FloatCycle(t *testing.T) {
	t.Run("whole-number float accepted", func(t *testing.T) {
		var e LedgerEntry
		if err := json.Unmarshal([]byte(`{"cycle":107.0,"role":"x"}`), &e); err != nil {
			t.Fatalf("whole-number float should parse: %v", err)
		}
		if e.Cycle != 107 {
			t.Errorf("Cycle = %d, want 107", e.Cycle)
		}
	})
	t.Run("fractional float rejected", func(t *testing.T) {
		var e LedgerEntry
		if err := json.Unmarshal([]byte(`{"cycle":107.5,"role":"x"}`), &e); err == nil {
			t.Errorf("fractional float must error, got Cycle=%d", e.Cycle)
		}
	})
}

// TestLedgerEntry_Unmarshal_EmptyCycle: cycle field absent entirely is
// the pre-v8.37 case; LedgerEntry.Cycle defaults to 0 and parses cleanly.
func TestLedgerEntry_Unmarshal_EmptyCycle(t *testing.T) {
	var e LedgerEntry
	if err := json.Unmarshal([]byte(`{"role":"x","entry_seq":5}`), &e); err != nil {
		t.Fatalf("missing cycle should not error: %v", err)
	}
	if e.Cycle != 0 || e.CycleLabel != "" {
		t.Errorf("missing cycle: got Cycle=%d Label=%q; want 0/empty", e.Cycle, e.CycleLabel)
	}
}

// TestLedgerEntry_Unmarshal_CycleOutOfRange exercises the int32 range
// guard so behaviour is identical on 32- and 64-bit targets — without
// the guard, very-large cycle numbers would silently truncate on a
// 32-bit builder.
func TestLedgerEntry_Unmarshal_CycleOutOfRange(t *testing.T) {
	cases := []string{
		`{"cycle":2147483648,"role":"x"}`,  // MaxInt32 + 1
		`{"cycle":-2147483649,"role":"x"}`, // MinInt32 - 1
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			var e LedgerEntry
			if err := json.Unmarshal([]byte(raw), &e); err == nil {
				t.Errorf("expected out-of-range error, got Cycle=%d", e.Cycle)
			}
		})
	}
}
