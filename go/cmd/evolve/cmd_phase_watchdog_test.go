package main

import "testing"

// watchdogEnvConfig reads the EVOLVE_INACTIVITY_* knobs through envchain.

func TestWatchdogEnvConfig_Defaults(t *testing.T) {
	for _, k := range []string{
		"EVOLVE_PROJECT_ROOT", "EVOLVE_INACTIVITY_THRESHOLD_S", "EVOLVE_INACTIVITY_POLL_S",
		"EVOLVE_INACTIVITY_WARN_PCT", "EVOLVE_INACTIVITY_GRACE_S", "EVOLVE_INACTIVITY_DISABLE",
	} {
		t.Setenv(k, "")
	}
	c := watchdogEnvConfig()
	if c.ThresholdS != 0 || c.PollS != 0 || c.WarnPct != 0 || c.GraceS != 0 {
		t.Errorf("ints = %d/%d/%d/%d, want all 0", c.ThresholdS, c.PollS, c.WarnPct, c.GraceS)
	}
	if c.Disabled {
		t.Errorf("Disabled default = true, want false (default-off `== \"1\"` flag)")
	}
	if c.ProjectRoot != "" {
		t.Errorf("ProjectRoot default = %q, want empty", c.ProjectRoot)
	}
}

func TestWatchdogEnvConfig_Parsing(t *testing.T) {
	t.Setenv("EVOLVE_INACTIVITY_THRESHOLD_S", "600")
	t.Setenv("EVOLVE_INACTIVITY_POLL_S", "10")
	t.Setenv("EVOLVE_INACTIVITY_WARN_PCT", "80")
	t.Setenv("EVOLVE_INACTIVITY_GRACE_S", "30")
	t.Setenv("EVOLVE_INACTIVITY_DISABLE", "1")
	c := watchdogEnvConfig()
	if c.ThresholdS != 600 || c.PollS != 10 || c.WarnPct != 80 || c.GraceS != 30 {
		t.Errorf("ints = %d/%d/%d/%d, want 600/10/80/30", c.ThresholdS, c.PollS, c.WarnPct, c.GraceS)
	}
	if !c.Disabled {
		t.Errorf("Disabled(\"1\") = false, want true")
	}
}

func TestWatchdogEnvConfig_InvalidIntFallsBack(t *testing.T) {
	t.Setenv("EVOLVE_INACTIVITY_THRESHOLD_S", "not-a-number")
	if c := watchdogEnvConfig(); c.ThresholdS != 0 {
		t.Errorf("ThresholdS(garbage) = %d, want 0 (fallback)", c.ThresholdS)
	}
}
