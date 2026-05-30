package config

import (
	"strings"
	"testing"
)

// inertEnableWarn extracts the inert-phase-enable warning from ws, or returns
// ("", false) when none is present. Helper so tests assert on the typed code
// rather than substring-matching the message text.
func inertEnableWarn(ws []Warning) (Warning, bool) {
	for _, w := range ws {
		if w.Code == "inert-phase-enable" {
			return w, true
		}
	}
	return Warning{}, false
}

// TestLoad_PlanReviewEnabled_StageOff_EmitsInertWarning is the canonical
// trigger surfaced by the cycle-120 retrospective: an operator sets
// EVOLVE_PLAN_REVIEW=1 expecting plan-review to run, but the default static
// state machine ignores it (plan-review is router-only). Pre-fix this was a
// silent no-op — the warning makes the inert flag loud.
func TestLoad_PlanReviewEnabled_StageOff_EmitsInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{"EVOLVE_PLAN_REVIEW": "1", "EVOLVE_DYNAMIC_ROUTING": "off"})
	w, ok := inertEnableWarn(ws)
	if !ok {
		t.Fatalf("expected an inert-phase-enable warning; got %+v", ws)
	}
	if !strings.Contains(w.Message, "plan-review") {
		t.Errorf("warning should name the phase; got %q", w.Message)
	}
}

// TestLoad_PlanReviewEnabled_StageAdvisory_NoInertWarning proves the warning
// is SCOPED to Stage<Advisory. At Stage>=Advisory the router consults
// PhaseEnable, so plan-review IS reachable and the enable is not inert.
func TestLoad_PlanReviewEnabled_StageAdvisory_NoInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{
		"EVOLVE_PLAN_REVIEW":     "1",
		"EVOLVE_DYNAMIC_ROUTING": "advisory",
	})
	if _, ok := inertEnableWarn(ws); ok {
		t.Errorf("inert-warning fired at Stage=Advisory; got %+v", ws)
	}
}

// TestLoad_PlanReviewEnabled_StageShadow_EmitsInertWarning pins the
// reviewer-flagged Stage==Shadow case: shadow computes+logs the would-have-
// routed plan but the STATIC state machine still drives execution, so
// non-spine phases remain unreachable. The warning must still fire to keep the
// operator-confusion failure mode plugged across the full sub-advisory range.
func TestLoad_PlanReviewEnabled_StageShadow_EmitsInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{
		"EVOLVE_PLAN_REVIEW":     "1",
		"EVOLVE_DYNAMIC_ROUTING": "shadow",
	})
	if _, ok := inertEnableWarn(ws); !ok {
		t.Errorf("expected inert-warning at Stage=Shadow (static still drives); got %+v", ws)
	}
}

// TestLoad_SpinePhaseEnabled_NoInertWarning proves the warning is also scoped
// to NON-spine phases. A force-enable of a spine phase (tdd here) is honored
// by the static state machine, so it's not inert.
func TestLoad_SpinePhaseEnabled_NoInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{"EVOLVE_TEST_PHASE_ENABLED": "1"})
	if _, ok := inertEnableWarn(ws); ok {
		t.Errorf("inert-warning fired on a spine phase (tdd); got %+v", ws)
	}
}

// TestLoad_NoEnables_NoInertWarning baseline: empty env → no warning.
func TestLoad_NoEnables_NoInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{})
	if _, ok := inertEnableWarn(ws); ok {
		t.Errorf("inert-warning fired with no enables; got %+v", ws)
	}
}

// TestLoad_OtherWarningsStillEmit ensures the new pass does not suppress the
// existing weak-spine warning when an operator both weakens the spine AND
// inert-enables a phase. Both must surface.
func TestLoad_OtherWarningsStillEmit(t *testing.T) {
	_, ws := Load("", map[string]string{
		"EVOLVE_PLAN_REVIEW":      "1",
		"EVOLVE_DYNAMIC_ROUTING":  "off",         // force StageOff so plan-review is inert
		"EVOLVE_MANDATORY_PHASES": "scout,build", // omits audit+ship → weak-spine
	})
	var sawInert, sawWeak bool
	for _, w := range ws {
		if w.Code == "inert-phase-enable" {
			sawInert = true
		}
		if w.Code == "weak-spine" {
			sawWeak = true
		}
	}
	if !sawInert {
		t.Error("expected inert-phase-enable warning")
	}
	if !sawWeak {
		t.Error("expected weak-spine warning (audit+ship dropped)")
	}
}
