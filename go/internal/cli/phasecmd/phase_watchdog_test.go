package phasecmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestWatchdogEnvConfig_Defaults(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	c := watchdogEnvConfig(&bytes.Buffer{})
	if c.ThresholdS != 600 || c.PollS != 15 || c.WarnPct != 75 || c.GraceS != 10 {
		t.Errorf("ints = %d/%d/%d/%d, want 600/15/75/10", c.ThresholdS, c.PollS, c.WarnPct, c.GraceS)
	}
	if c.Disabled {
		t.Errorf("Disabled default = true, want false (default-off `== \"1\"` flag)")
	}
	if c.ProjectRoot != root {
		t.Errorf("ProjectRoot = %q, want %q", c.ProjectRoot, root)
	}
}

func TestWatchdogEnvConfig_Parsing(t *testing.T) {
	root := writeObserverPolicy(t, `{"observer":{"stall_s":900,"watchdog_poll_s":10,"watchdog_warn_pct":80,"watchdog_grace_s":30,"watchdog_disabled":true}}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	c := watchdogEnvConfig(&bytes.Buffer{})
	if c.ThresholdS != 900 || c.PollS != 10 || c.WarnPct != 80 || c.GraceS != 30 {
		t.Errorf("ints = %d/%d/%d/%d, want 900/10/80/30", c.ThresholdS, c.PollS, c.WarnPct, c.GraceS)
	}
	if !c.Disabled {
		t.Errorf("Disabled(\"1\") = false, want true")
	}
}

func TestWatchdogEnvConfig_MalformedPolicyWarnsWithCode(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", writeObserverPolicy(t, `{`))
	var stderr bytes.Buffer
	c := watchdogEnvConfig(&stderr)
	if c.ThresholdS != 600 {
		t.Errorf("ThresholdS = %d, want the compiled default 600", c.ThresholdS)
	}
	if got := stderr.String(); !strings.Contains(got, "WARN OBSERVER_POLICY_LOAD_FAILED") || !strings.Contains(got, "policy.json") {
		t.Errorf("stderr = %q, want a coded WARN naming policy.json", got)
	}
}
