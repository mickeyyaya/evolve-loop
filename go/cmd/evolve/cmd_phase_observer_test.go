package main

import "testing"

// observerEnvConfig reads the EVOLVE_OBSERVER_* knobs through envchain. The
// most important pin is NudgeS's default of 300 (a flip to 0 silently disables
// the pre-SIGTERM nudge) and that StallS keeps its two-key fallback
// (EVOLVE_OBSERVER_STALL_S else EVOLVE_INACTIVITY_THRESHOLD_S).

func TestObserverEnvConfig_Defaults(t *testing.T) {
	for _, k := range []string{
		"EVOLVE_OBSERVER_POLL_S", "EVOLVE_OBSERVER_STALL_S", "EVOLVE_INACTIVITY_THRESHOLD_S",
		"EVOLVE_OBSERVER_NUDGE_S", "EVOLVE_OBSERVER_NUDGE_BODY", "EVOLVE_OBSERVER_EOF_GRACE_S",
	} {
		t.Setenv(k, "")
	}
	c := observerEnvConfig()
	if c.PollS != 0 || c.StallS != 0 || c.EOFGraceS != 0 {
		t.Errorf("PollS/StallS/EOFGraceS = %d/%d/%d, want 0/0/0", c.PollS, c.StallS, c.EOFGraceS)
	}
	if c.NudgeS != 300 {
		t.Errorf("NudgeS default = %d, want 300 (a flip to 0 disables the nudge)", c.NudgeS)
	}
	if c.NudgeBody != "" {
		t.Errorf("NudgeBody default = %q, want empty", c.NudgeBody)
	}
}

func TestObserverEnvConfig_Parsing(t *testing.T) {
	t.Setenv("EVOLVE_OBSERVER_POLL_S", "5")
	t.Setenv("EVOLVE_OBSERVER_NUDGE_S", "120")
	t.Setenv("EVOLVE_OBSERVER_NUDGE_BODY", "wake up")
	t.Setenv("EVOLVE_OBSERVER_EOF_GRACE_S", "3")
	c := observerEnvConfig()
	if c.PollS != 5 || c.NudgeS != 120 || c.EOFGraceS != 3 || c.NudgeBody != "wake up" {
		t.Errorf("got PollS=%d NudgeS=%d EOFGraceS=%d NudgeBody=%q", c.PollS, c.NudgeS, c.EOFGraceS, c.NudgeBody)
	}
}

func TestObserverEnvConfig_StallSTwoKeyFallback(t *testing.T) {
	// Primary wins when set.
	t.Setenv("EVOLVE_OBSERVER_STALL_S", "10")
	t.Setenv("EVOLVE_INACTIVITY_THRESHOLD_S", "20")
	if c := observerEnvConfig(); c.StallS != 10 {
		t.Errorf("StallS(primary=10,fallback=20) = %d, want 10", c.StallS)
	}
	// Falls back to the legacy key when primary is unset.
	t.Setenv("EVOLVE_OBSERVER_STALL_S", "")
	if c := observerEnvConfig(); c.StallS != 20 {
		t.Errorf("StallS(primary unset, fallback=20) = %d, want 20", c.StallS)
	}
}
