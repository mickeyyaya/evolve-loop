package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// ports_ledger_runid_test.go — CA.2 (concurrency-factory plan, Track C-A):
// LedgerEntry gains an event-sourced run identity (`run_id`, omitempty).
// Single-mode byte-stability is the invariant: entries without a RunID
// marshal byte-identically to pre-CA.2 lines (additive field only), and
// legacy lines (no run_id key) decode unchanged.

func TestLedgerEntry_RunID_RoundTrip(t *testing.T) {
	in := LedgerEntry{TS: "2026-06-11T00:00:00Z", Cycle: 300, Role: "build", Kind: "phase", RunID: "01JXAXAMPLE0000000000000000"}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"run_id":"01JXAXAMPLE0000000000000000"`) {
		t.Fatalf("marshal lost run_id: %s", data)
	}
	var out LedgerEntry
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.RunID != in.RunID {
		t.Errorf("round-trip RunID = %q, want %q", out.RunID, in.RunID)
	}
}

// TestLedgerEntry_RunID_OmittedWhenEmpty pins the additive-fields-only
// byte-stability contract: a single-mode entry (no run id) must marshal
// with NO run_id key at all.
func TestLedgerEntry_RunID_OmittedWhenEmpty(t *testing.T) {
	data, err := json.Marshal(LedgerEntry{TS: "2026-06-11T00:00:00Z", Cycle: 300, Role: "build", Kind: "phase"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "run_id") {
		t.Fatalf("empty RunID must be omitted (byte-stability): %s", data)
	}
}

// TestLedgerEntry_RunID_LegacyLineDecodes pins that a real pre-CA.2 ledger
// line (string-cycle variant included — the custom unmarshaler's special
// case) decodes with RunID empty and everything else intact.
func TestLedgerEntry_RunID_LegacyLineDecodes(t *testing.T) {
	lines := []string{
		`{"ts":"2026-06-07T10:00:00Z","cycle":248,"role":"ship","kind":"ship","exit_code":0,"entry_seq":5900,"prev_hash":"abc"}`,
		`{"ts":"2026-06-07T10:00:00Z","cycle":"manual-release","role":"ship","kind":"ship","exit_code":0}`,
	}
	for _, line := range lines {
		var e LedgerEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("legacy line failed to decode: %v\n%s", err, line)
		}
		if e.RunID != "" {
			t.Errorf("legacy line produced RunID %q, want empty", e.RunID)
		}
		if e.Role != "ship" {
			t.Errorf("legacy decode lost fields: %+v", e)
		}
	}
}
