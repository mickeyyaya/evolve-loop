package clihealth

import "testing"

func TestLoginCommand(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"claude": "open claude and run /login",
		"codex":  "run codex login",
		"agy":    "log in to agy",
		"":       "log in to ",
	}
	for family, want := range cases {
		if got := LoginCommand(family); got != want {
			t.Errorf("LoginCommand(%q)=%q want %q", family, got, want)
		}
	}
}

func TestOperatorAction_LoginRendersFromLoginCommand(t *testing.T) {
	t.Parallel()
	want := "operator: " + LoginCommand("claude") + "; the bench clears itself once a probe succeeds"
	if got := OperatorAction("claude", CredentialPattern); got != want {
		t.Errorf("OperatorAction=%q want %q", got, want)
	}
}
