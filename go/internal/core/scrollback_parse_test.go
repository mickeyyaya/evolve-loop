package core

import "testing"

// scrollback_parse_test.go — the ADR-0027 stdout contract reads the advisor's
// JSON from the captured REPL scrollback, which echoes the PROMPT (and the
// prompt carries a JSON example of the very shape we parse). These tests pin
// that the parser takes the agent's REPLY (last balanced span), not the
// prompt's example — and that a clean single body (mock-bridge path) is
// unaffected.

func TestLastBalancedSpan(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		s                  string
		open, close        byte
		wantStart, wantEnd int
		wantOK             bool
	}{
		{"single array", "[1,2]", '[', ']', 0, 4, true},
		{"single object", `{"a":1}`, '{', '}', 0, 6, true},
		{"leading prose", "answer: [1]", '[', ']', 8, 10, true},
		{"two arrays takes last", "[1] then [2,3]", '[', ']', 9, 13, true},
		{"nested takes outermost-of-last", "x [ [1] ] y [ [2] ]", '[', ']', 12, 18, true},
		{"no close", "[1,2", '[', ']', 0, 0, false},
		{"no open match", "abc]", '[', ']', 0, 0, false},
		// String-literal awareness: a delimiter inside a quoted value must not
		// be counted (the go-reviewer MEDIUM — brackets in a justification).
		{"close byte inside string", `{"k":"a } b"}`, '{', '}', 0, 12, true},
		{"open+close inside string", `{"k":"[x] {y}"}`, '{', '}', 0, 14, true},
		{"escaped quote in string", `{"k":"she said \"hi\" }"}`, '{', '}', 0, 24, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start, end, ok := lastBalancedSpan(c.s, c.open, c.close)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if ok && (start != c.wantStart || end != c.wantEnd) {
				t.Fatalf("span = [%d,%d] (%q), want [%d,%d]", start, end, c.s[start:end+1], c.wantStart, c.wantEnd)
			}
		})
	}
}

func TestParsePhasePlan_IgnoresPromptEchoExample(t *testing.T) {
	t.Parallel()
	// The scrollback: prompt's example array FIRST, then the agent's reply.
	scrollback := `## Respond with STRICT JSON only (a bare array):
[{"phase":"<phase>","run":true,"justification":"<one sentence>"},...]

⏺ [
  {"phase":"scout","run":true,"justification":"start of cycle"},
  {"phase":"build","run":true,"justification":"mandatory"},
  {"phase":"audit","run":true,"justification":"floor requires it"},
  {"phase":"ship","run":true,"justification":"completes spine"}
]

✻ Crunched for 8s
❯`
	plan, err := parsePhasePlan(scrollback)
	if err != nil {
		t.Fatalf("parsePhasePlan: %v", err)
	}
	if len(plan.Entries) != 4 {
		t.Fatalf("entries = %d (%+v), want 4 (the reply, not the 1-entry prompt example)", len(plan.Entries), plan.Entries)
	}
	if plan.Entries[0].Phase != "scout" {
		t.Errorf("first entry phase = %q, want scout", plan.Entries[0].Phase)
	}
}

func TestParsePhasePlan_CleanBodyUnchanged(t *testing.T) {
	t.Parallel()
	// Mock-bridge path: a clean single array must still parse (parity).
	plan, err := parsePhasePlan(`[{"phase":"scout","run":true,"justification":"x"}]`)
	if err != nil || len(plan.Entries) != 1 || plan.Entries[0].Phase != "scout" {
		t.Fatalf("clean body: plan=%+v err=%v", plan, err)
	}
}

func TestParseProposal_IgnoresPromptEchoExample(t *testing.T) {
	t.Parallel()
	scrollback := `Respond with STRICT JSON:
{"next_phase":"<phase>","insert_phases":["<phase>"],"justification":"<one sentence>"}

⏺ {"next_phase":"build","insert_phases":[],"justification":"scout done"}
❯`
	prop, err := parseProposal(scrollback)
	if err != nil {
		t.Fatalf("parseProposal: %v", err)
	}
	if prop.NextPhase != "build" {
		t.Fatalf("next_phase = %q, want build (the reply, not the prompt's <phase> example)", prop.NextPhase)
	}
}
