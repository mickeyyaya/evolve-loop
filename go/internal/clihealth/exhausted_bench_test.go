package clihealth

import (
	"testing"
	"time"
)

func TestBenchable_AdmitsTheExhaustedWall(t *testing.T) {
	for _, p := range []string{"rate_limit", "exhausted", BootTimeoutPattern} {
		if !Benchable(p) {
			t.Errorf("Benchable(%q) = false — a classified quota wall must be benchable, "+
				"or the family is re-dispatched into a wall we already measured", p)
		}
	}
	for _, p := range []string{"trust_prompt", "model_deprecation_continue", ""} {
		if Benchable(p) {
			t.Errorf("Benchable(%q) = true — situational escalations must stay retryable", p)
		}
	}
}

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

	second := NewBenchEntry(first, "agy", ExhaustedPattern, pane, now)
	if !second.BenchedUntil.After(first.BenchedUntil) {
		t.Errorf("second strike (%v) must bench longer than the first (%v)", second.BenchedUntil, first.BenchedUntil)
	}
	if second.Strikes != 2 {
		t.Errorf("Strikes = %d, want 2", second.Strikes)
	}
}
