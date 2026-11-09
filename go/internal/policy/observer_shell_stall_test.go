package policy

import "testing"

func TestObserverConfig_ShellStallSOverrideWinsOverTheDefault(t *testing.T) {
	s := 90
	got := Policy{Observer: &ObserverPolicy{ShellStallS: &s}}.ObserverConfig()
	if got.ShellStallS == nil || *got.ShellStallS != 90 {
		t.Fatalf("ShellStallS = %v, want 90", got.ShellStallS)
	}
}
