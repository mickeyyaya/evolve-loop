package cyclestate

import "testing"

// TestPhaseConstants pins the exact wire string of every Phase constant.
// These strings are serialized into ledger/state JSON and parsed by the
// EGPS predicates — a drift here is a trust-kernel break, so the test is a
// byte-identity guard, not a smoke test.
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

// TestPhaseIsValid_Unknown ensures IsValid rejects strings outside the
// known set (guards against silent typos in routing/plan code).
func TestPhaseIsValid_Unknown(t *testing.T) {
	for _, s := range []string{"", "Scout", "buildplanner", "unknown"} {
		if Phase(s).IsValid() {
			t.Errorf("Phase(%q).IsValid() = true, want false", s)
		}
	}
}

// TestVerdictConstants pins the verdict vocabulary the EGPS gate matches on.
// Slice-of-struct (not map) so each constant IDENTIFIER is asserted against its
// expected literal — this catches a typo'd constant (a map keyed by the constant
// resolves the value first, so key==value and the check is dead).
func TestVerdictConstants(t *testing.T) {
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

// TestIsVerdict_Rejects guards the case/whitespace sensitivity contract.
func TestIsVerdict_Rejects(t *testing.T) {
	for _, s := range []string{"", "pass", " PASS", "OK"} {
		if IsVerdict(s) {
			t.Errorf("IsVerdict(%q) = true, want false", s)
		}
	}
}

// TestCycleOutcomeConstants pins the cycle-level outcome labels (slice-of-struct
// so the constant identifier is asserted against its literal — see above).
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
