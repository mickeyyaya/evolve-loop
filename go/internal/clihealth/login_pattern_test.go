package clihealth

import (
	"testing"
	"time"
)

func TestLoginPattern(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"auth_recheck":    true,
		"auth_required":   true,
		"rate_limit":      false,
		"quota_exhausted": false,
		"":                false,
	}
	for in, want := range cases {
		if got := LoginPattern(in); got != want {
			t.Errorf("LoginPattern(%q)=%v want %v", in, got, want)
		}
	}
}

func TestAuthBench_AnOllamaLoginBenchesAndIsBenchable(t *testing.T) {
	t.Parallel()
	if !AuthBench("auth_required") {
		t.Error("AuthBench(auth_required) = false")
	}
	if !Benchable("auth_required") {
		t.Error("Benchable(auth_required) = false")
	}
	if AuthBench("rate_limit") {
		t.Error("AuthBench(rate_limit) = true")
	}
}

func TestLoginPattern_AnOllamaLoginBenchEntryCarriesTheOperatorActionAndNoTimedCooldown(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	for _, pattern := range []string{CredentialPattern, OllamaLoginPattern} {
		e := NewBenchEntry(Entry{}, "ollama", pattern, "unauthorized, try again in 5 minutes", now)
		if e.OperatorAction != OperatorAction("ollama", CredentialPattern) {
			t.Errorf("%s: operator action %q", pattern, e.OperatorAction)
		}
		if e.OperatorAction == "" {
			t.Errorf("%s: empty operator action", pattern)
		}
		if want := now.Add(CooldownForStrikes(1)); !e.BenchedUntil.Equal(want) {
			t.Errorf("%s: until %v want %v", pattern, e.BenchedUntil, want)
		}
	}
}
