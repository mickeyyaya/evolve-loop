package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLedgerEntry_Unmarshal_IntCycle(t *testing.T) {
	t.Parallel()
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

func TestLedgerEntry_Unmarshal_StringCycle(t *testing.T) {
	t.Parallel()
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

func TestLedgerEntry_Unmarshal_ExplicitCycleLabel(t *testing.T) {
	t.Parallel()
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

func TestLedgerEntry_Marshal_NormalEntry(t *testing.T) {
	t.Parallel()
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

func TestLedgerEntry_Marshal_LabeledEntry(t *testing.T) {
	t.Parallel()
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

func TestLedgerEntry_RoundTrip_StringCycle_NormalizesToLabel(t *testing.T) {
	t.Parallel()
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

func TestLedgerEntry_Unmarshal_MalformedCycle(t *testing.T) {
	t.Parallel()
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

func TestLedgerEntry_Unmarshal_FloatCycle(t *testing.T) {
	t.Parallel()
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

func TestLedgerEntry_Unmarshal_EmptyCycle(t *testing.T) {
	t.Parallel()
	var e LedgerEntry
	if err := json.Unmarshal([]byte(`{"role":"x","entry_seq":5}`), &e); err != nil {
		t.Fatalf("missing cycle should not error: %v", err)
	}
	if e.Cycle != 0 || e.CycleLabel != "" {
		t.Errorf("missing cycle: got Cycle=%d Label=%q; want 0/empty", e.Cycle, e.CycleLabel)
	}
}

func TestLedgerEntry_Unmarshal_CycleOutOfRange(t *testing.T) {
	t.Parallel()
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

func TestLedgerEntry_NullCycleAbsorbed(t *testing.T) {
	raw := `{"ts":"2026-07-22T22:54:50Z","class":"inbox-lifecycle","action":"promote","task_id":"x","cycle":null,"git_sha":null,"reason":"ship-promote-processed"}`
	var e LedgerEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("null cycle must unmarshal like an absent field, got: %v", err)
	}
	if e.Cycle != 0 || e.CycleLabel != "" {
		t.Fatalf("null cycle routed wrong: Cycle=%d CycleLabel=%q, want 0/empty", e.Cycle, e.CycleLabel)
	}
}
