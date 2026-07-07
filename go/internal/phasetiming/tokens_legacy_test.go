package phasetiming

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// TestTimingEntry_LegacyLogParses guards backward compatibility for the S4
// tokens field: a phase-timing.json entry written before the field existed
// must still unmarshal, leaving Tokens zero-valued (never a parse error). New
// entries carry the terminal attempt's token counts beside cost_usd.
func TestTimingEntry_LegacyLogParses(t *testing.T) {
	t.Parallel()

	legacy := `[{"phase":"build","duration_ms":700000,"verdict":"PASS","cost_usd":0.5,"attempt_count":1}]`
	var entries []Entry
	if err := json.Unmarshal([]byte(legacy), &entries); err != nil {
		t.Fatalf("legacy timing log must parse: %v", err)
	}
	if len(entries) != 1 || entries[0].Phase != "build" {
		t.Fatalf("parsed %d entries, want 1 build entry", len(entries))
	}
	if (entries[0].Tokens != cyclestate.TokenUsage{}) {
		t.Errorf("legacy entry tokens = %+v, want zero value", entries[0].Tokens)
	}

	// A new entry round-trips its tokens.
	fresh := Entry{Phase: "build", Tokens: cyclestate.TokenUsage{Input: 10, Output: 5}}
	data, err := json.Marshal(fresh)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Entry
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Tokens != fresh.Tokens {
		t.Errorf("round-trip tokens = %+v, want %+v", back.Tokens, fresh.Tokens)
	}
}
