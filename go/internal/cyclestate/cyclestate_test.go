package cyclestate

import "testing"

func TestPhaseConstants(t *testing.T) {
	cases := []struct {
		p    Phase
		want string
	}{
		{PhaseStart, "start"},
		{PhaseIntent, "intent"},
		{PhaseScout, "scout"},
		{PhaseTriage, "triage"},
		{PhaseTDD, "tdd"},
		{PhaseBuildPlanner, "build-planner"},
		{PhaseSwarmPlan, "swarm-plan"},
		{PhaseBuild, "build"},
		{PhaseAudit, "audit"},
		{PhaseShip, "ship"},
		{PhaseRetro, "retro"},
		{PhaseDebugger, "debugger"},
		{PhaseEnd, "end"},
	}
	for _, c := range cases {
		if got := c.p.String(); got != c.want {
			t.Errorf("Phase(%q).String() = %q, want %q", string(c.p), got, c.want)
		}
		if !c.p.IsValid() {
			t.Errorf("Phase(%q).IsValid() = false, want true", string(c.p))
		}
	}
}

func TestPhaseIsValid_Unknown(t *testing.T) {
	for _, s := range []string{"", "Scout", "buildplanner", "unknown"} {
		if Phase(s).IsValid() {
			t.Errorf("Phase(%q).IsValid() = true, want false", s)
		}
	}
}

func TestVerdictConstants(t *testing.T) {
	// A slice, not a map keyed by the constant: such a map compares each value with itself.
	cases := []struct {
		got  string
		want string
	}{
		{VerdictPASS, "PASS"},
		{VerdictFAIL, "FAIL"},
		{VerdictWARN, "WARN"},
		{VerdictSKIPPED, "SKIPPED"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("verdict constant = %q, want %q", c.got, c.want)
		}
		if !IsVerdict(c.got) {
			t.Errorf("IsVerdict(%q) = false, want true", c.got)
		}
	}
}

func TestCycleTerminationTriageNoWork(t *testing.T) {
	const want = "triage-empty-commitment"
	if CycleTerminationTriageNoWork != want {
		t.Errorf("CycleTerminationTriageNoWork = %q, want %q", CycleTerminationTriageNoWork, want)
	}
}

func TestIsVerdict_Rejects(t *testing.T) {
	for _, s := range []string{"", "pass", " PASS", "OK"} {
		if IsVerdict(s) {
			t.Errorf("IsVerdict(%q) = true, want false", s)
		}
	}
}

func TestCycleOutcomeConstants(t *testing.T) {
	cases := []struct {
		got  string
		want string
	}{
		{CycleOutcomeShippedViaBuild, "SHIPPED_VIA_BUILD"},
		{CycleOutcomeSkippedAuditAdvisory, "SKIPPED_AUDIT_ADVISORY"},
		{CycleOutcomeSkippedUnknown, "SKIPPED_UNKNOWN"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("cycle-outcome constant = %q, want %q", c.got, c.want)
		}
	}
}
