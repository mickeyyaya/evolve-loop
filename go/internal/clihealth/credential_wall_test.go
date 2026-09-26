package clihealth

import (
	"strings"
	"testing"
	"time"
)

// A credential wall is worth benching for the same reason a quota wall is: re-dispatching into it is
// guaranteed waste. Wave 14 re-dispatched claude fourteen times into "Please log in".
func TestBenchable_ACredentialWallBenchesTheFamily(t *testing.T) {
	if !Benchable(CredentialPattern) {
		t.Fatalf("%q must bench: every re-dispatch into a login prompt is waste", CredentialPattern)
	}
	if Benchable("trust_prompt") {
		t.Fatal("a situational escalation must not bench")
	}
}

func TestOperatorAction_NamesTheFixForACredentialWallOnly(t *testing.T) {
	got := OperatorAction("claude", CredentialPattern)
	for _, want := range []string{"claude", "logged in", "operator"} {
		if !strings.Contains(got, want) {
			t.Fatalf("OperatorAction(claude, %s) = %q, want it to name the family, the login and the operator", CredentialPattern, got)
		}
	}
	if strings.Contains(got, "/login") || strings.Contains(got, " login") {
		t.Fatalf("OperatorAction must not name a login command; the families differ (agy trusts a directory, ollama signs in): %q", got)
	}
	for _, pattern := range []string{"rate_limit", ExhaustedPattern, BootTimeoutPattern, ""} {
		if got := OperatorAction("codex", pattern); got != "" {
			t.Fatalf("OperatorAction(codex, %q) = %q, want nothing: a wall the canary can clear needs no operator", pattern, got)
		}
	}
}

// The fix is durable on the bench entry, so cli-health.json and every reader of it carry it, not only the
// log line that happened to print when the family was benched.
func TestNewBenchEntry_CarriesTheOperatorActionForACredentialWall(t *testing.T) {
	now := time.Date(2026, 9, 26, 22, 40, 0, 0, time.UTC)
	e := NewBenchEntry(Entry{}, "claude", CredentialPattern, "Please log in to continue", now)
	if e.OperatorAction != OperatorAction("claude", CredentialPattern) || e.OperatorAction == "" {
		t.Fatalf("OperatorAction = %q, want the credential wall's fix on the entry", e.OperatorAction)
	}
	if q := NewBenchEntry(Entry{}, "codex", "rate_limit", "usage limit reached", now); q.OperatorAction != "" {
		t.Fatalf("a quota wall carries no operator action; got %q", q.OperatorAction)
	}
}

// A login pane carries no reset time of its own; a stale hint from an earlier wall in the same scrollback
// must not set the bench.
func TestNewBenchEntry_ACredentialWallIgnoresAStaleResetHint(t *testing.T) {
	now := time.Date(2026, 9, 26, 22, 40, 0, 0, time.UTC)
	pane := "You've hit your usage limit · try again at 11:30 PM\nPlease log in to continue"
	e := NewBenchEntry(Entry{}, "claude", CredentialPattern, pane, now)
	if got := e.BenchedUntil.Sub(now); got != CooldownForStrikes(1) {
		t.Fatalf("credential bench = %v after now, want the strike cooldown %v, never a hint from the pane", got, CooldownForStrikes(1))
	}
	if q := NewBenchEntry(Entry{}, "codex", "rate_limit", pane, now); q.BenchedUntil.Sub(now) == CooldownForStrikes(1) {
		t.Fatalf("a quota wall still honours the pane's hint; got the plain cooldown")
	}
}

// Time says nothing about whether the operator logged in: a credential bench stays active for routing
// after its cooldown and is due for a probe, until a succeeding probe clears it.
func TestStore_ACredentialBenchHoldsUntilAProbeClearsIt(t *testing.T) {
	now := time.Date(2026, 9, 26, 22, 40, 0, 0, time.UTC)
	clock := now
	store := NewStore(t.TempDir(), func() time.Time { return clock })
	if _, err := store.BenchWall("claude", CredentialPattern, "Please log in"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BenchWall("codex", "rate_limit", "usage limit reached"); err != nil {
		t.Fatal(err)
	}
	clock = now.Add(5 * time.Hour)
	active, expired := store.Active(), store.Expired()
	if _, ok := active["claude"]; !ok {
		t.Fatalf("the credential bench lapsed by time alone; active=%v", active)
	}
	if _, ok := active["codex"]; ok {
		t.Fatalf("a quota bench still lapses by time; active=%v", active)
	}
	if _, ok := expired["claude"]; !ok {
		t.Fatalf("the credential bench must be due for a probe after its cooldown; expired=%v", expired)
	}
	if err := store.Clear("claude"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Active()["claude"]; ok {
		t.Fatal("a succeeding probe's Clear must release the credential bench")
	}
}
