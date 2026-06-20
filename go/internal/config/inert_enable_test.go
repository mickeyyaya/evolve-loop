package config

import "testing"

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

// TestLoad_PlanReviewEnabled_StageAdvisory_NoInertWarning proves the inert
// warning is SCOPED to Stage<Advisory. At Stage>=Advisory the router consults
// PhaseEnable, so a non-spine phase enable is not inert.
// (The env-based EVOLVE_PLAN_REVIEW trigger was removed in cycle-39; this
// baseline checks that no spurious warning fires with empty env.)
func TestLoad_PlanReviewEnabled_StageAdvisory_NoInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{
		"EVOLVE_DYNAMIC_ROUTING": "advisory",
	})
	if _, ok := inertEnableWarn(ws); ok {
		t.Errorf("inert-warning fired with no enables at Stage=Advisory; got %+v", ws)
	}
}

// TestLoad_SpinePhaseEnabled_NoInertWarning proves the inert warning is scoped
// to NON-spine phases. With empty env the spine defaults produce no inert warning.
func TestLoad_SpinePhaseEnabled_NoInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{})
	if _, ok := inertEnableWarn(ws); ok {
		t.Errorf("inert-warning fired with spine defaults; got %+v", ws)
	}
}

// TestLoad_NoEnables_NoInertWarning baseline: empty env → no warning.
func TestLoad_NoEnables_NoInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{})
	if _, ok := inertEnableWarn(ws); ok {
		t.Errorf("inert-warning fired with no enables; got %+v", ws)
	}
}

// TestLoad_OtherWarningsStillEmit ensures the weak-spine warning surfaces
// when audit+ship are dropped from the mandatory spine.
func TestLoad_OtherWarningsStillEmit(t *testing.T) {
	_, ws := Load("", map[string]string{
		"EVOLVE_MANDATORY_PHASES": "scout,build", // omits audit+ship → weak-spine
	})
	var sawWeak bool
	for _, w := range ws {
		if w.Code == "weak-spine" {
			sawWeak = true
		}
	}
	if !sawWeak {
		t.Error("expected weak-spine warning (audit+ship dropped)")
	}
}
