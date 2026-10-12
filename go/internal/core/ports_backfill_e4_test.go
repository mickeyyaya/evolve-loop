package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLedgerEntrySetCycle_EachJSONShapeOfTheCycleField(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw       string
		wantCycle int
		wantLabel string
		wantErr   string
	}{
		{"", 0, "", ""},
		{"  ", 0, "", ""},
		{"null", 0, "", ""},
		{"nul", 0, "", `unsupported JSON value "nul"`},
		{"nope", 0, "", `unsupported JSON value "nope"`},
		{`"batch-7"`, 0, "batch-7", ""},
		{`"ünïcødé"`, 0, "ünïcødé", ""},
		{`"unterminated`, 0, "", "ledger cycle: "},
		{"42", 42, "kept", ""},
		{"-1", -1, "kept", ""},
		{"2147483647", 2147483647, "kept", ""},
		{"-2147483648", -2147483648, "kept", ""},
		{" 7 ", 7, "kept", ""},
		{"2147483648", 0, "", "out of range"},
		{"-2147483649", 0, "", "out of range"},
		{"1.5", 0, "", "fractional value 1.5"},
		{"1e999", 0, "", "ledger cycle: "},
		{"-", 0, "", "ledger cycle: "},
		{"0x10", 0, "", "ledger cycle: "},
		{"true", 0, "", `unsupported JSON value "true"`},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			e := LedgerEntry{Cycle: 99, CycleLabel: "kept"}
			err := e.setCycle(json.RawMessage(tc.raw))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("setCycle(%q) err = %v, want it to contain %q", tc.raw, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("setCycle(%q) = %v", tc.raw, err)
			}
			if strings.TrimSpace(tc.raw) == "" || tc.raw == "null" {
				if e.Cycle != 99 || e.CycleLabel != "kept" {
					t.Fatalf("an absent cycle must leave the entry untouched: %+v", e)
				}
				return
			}
			if e.Cycle != tc.wantCycle || e.CycleLabel != tc.wantLabel {
				t.Fatalf("setCycle(%q) = cycle %d label %q, want %d %q", tc.raw, e.Cycle, e.CycleLabel, tc.wantCycle, tc.wantLabel)
			}
		})
	}
}

func TestLedgerEntryUnmarshalJSON_MalformedJSONIsRejected(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{`, `[]`, `{"role":7}`, `{"cycle":}`, `not json`} {
		var e LedgerEntry
		if err := json.Unmarshal([]byte(raw), &e); err == nil {
			t.Errorf("Unmarshal(%q) = nil error, entry %+v", raw, e)
		}
	}
}

func FuzzLedgerEntryUnmarshalJSON_DecodesWhatItEncodes(f *testing.F) {
	f.Add(`{"ts":"2026-10-12T00:00:00Z","cycle":12,"role":"auditor","kind":"agent_subprocess","exit_code":0}`)
	f.Add(`{"cycle":"batch-1","role":"orchestrator"}`)
	f.Add(`{"cycle":null}`)
	f.Add(`{"cycle":1.5}`)
	f.Add(`{"cycle":-2147483649}`)
	f.Add(`{`)
	f.Add(`{"cycle":"ünïcødé","message":"multi\nline"}`)
	f.Fuzz(func(t *testing.T, raw string) {
		var e LedgerEntry
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			return
		}
		if e.CycleLabel != "" && e.Cycle != 0 {
			var probe struct {
				Cycle json.RawMessage `json:"cycle"`
			}
			if json.Unmarshal([]byte(raw), &probe) == nil && len(probe.Cycle) > 0 && probe.Cycle[0] == '"' {
				t.Fatalf("a string cycle left a numeric cycle %d: %+v", e.Cycle, e)
			}
		}
		encoded, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("a decoded entry must re-encode: %v", err)
		}
		var again LedgerEntry
		if err := json.Unmarshal(encoded, &again); err != nil {
			t.Fatalf("re-decode of %s: %v", encoded, err)
		}
		if again.Cycle != e.Cycle || again.Role != e.Role || again.Kind != e.Kind || again.Message != e.Message || again.RunID != e.RunID {
			t.Fatalf("round trip changed the entry: %+v -> %+v", e, again)
		}
	})
}
