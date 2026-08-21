package clihealth

import (
	"testing"
	"time"
)

// exhausted_bench_test.go — agy's quota wall never reached the bench ledger.
//
// Live, 4/4 lanes across two waves: agy's escalation reports carry
// pattern_name "exhausted" ("Individual quota reached. Please upgrade your
// subscription… Resets in 6h12m56s"), while codex's carry "rate_limit". Same
// wall, two vocabularies — and only one was admitted here, so cli-health never
// learned agy was walled and the bench-aware chain reorder (which demonstrably
// works: cycle-1525's build phase logged "cli-health bench reordered chain")
// had no row to act on. Every agy-primary phase paid a dead-CLI round-trip.

func TestBenchable_AdmitsTheExhaustedWall(t *testing.T) {
	for _, p := range []string{"rate_limit", "exhausted", BootTimeoutPattern} {
		if !Benchable(p) {
			t.Errorf("Benchable(%q) = false — a classified quota wall must be benchable, "+
				"or the family is re-dispatched into a wall we already measured", p)
		}
	}
	for _, p := range []string{"trust_prompt", "auth_recheck", ""} {
		if Benchable(p) {
			t.Errorf("Benchable(%q) = true — situational escalations must stay retryable", p)
		}
	}
}

// TestNewBenchEntry_AgyWallBenchesConservatively pins the behavior the evidence
// demands, and the mechanism that already delivers it.
//
// agy claimed "Resets in 6h12m56s" at 17:02Z and then served three successful
// dispatches at ~19:00Z — inside its own claimed window. Benching six hours on
// one refusal would sideline a working CLI, and with another family already
// benched that drops effective fleet width to 1.
//
// ParseResetHint does not recognize agy's "Resets in <dur>" format (it keys on
// "try again at H:MM" / "try again in N hours"), so the entry falls back to the
// strike-scaled cooldown: 30 minutes on the first strike, doubling only once the
// wall has actually repeated. This test exists so that a well-meaning future
// change teaching the parser agy's format has to confront the evidence first.
func TestNewBenchEntry_AgyWallBenchesConservatively(t *testing.T) {
	now := time.Date(2026, 8, 19, 17, 2, 0, 0, time.UTC)
	pane := "⚠ Individual quota reached. Please upgrade your subscription to increase your limits. Resets in 6h12m56s.\n"

	first := NewBenchEntry(Entry{}, "agy", ExhaustedPattern, pane, now)
	if want := now.Add(CooldownForStrikes(1)); !first.BenchedUntil.Equal(want) {
		t.Errorf("first strike benched until %v, want %v — one refusal must not buy a multi-hour bench",
			first.BenchedUntil, want)
	}
	if first.Strikes != 1 {
		t.Errorf("Strikes = %d, want 1", first.Strikes)
	}
	if first.Reason != ExhaustedPattern || first.Family != "agy" {
		t.Errorf("entry = %+v, want family agy reason %s", first, ExhaustedPattern)
	}
	if first.Evidence == "" {
		t.Error("the wall line must be kept as evidence so an operator can see WHAT walled the CLI")
	}

	// A REPEATED wall escalates — that is what makes a long bench earned.
	second := NewBenchEntry(first, "agy", ExhaustedPattern, pane, now)
	if !second.BenchedUntil.After(first.BenchedUntil) {
		t.Errorf("second strike (%v) must bench longer than the first (%v)", second.BenchedUntil, first.BenchedUntil)
	}
	if second.Strikes != 2 {
		t.Errorf("Strikes = %d, want 2", second.Strikes)
	}
}
