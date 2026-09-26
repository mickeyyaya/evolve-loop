package config

import "testing"

func inertEnableWarn(ws []Warning) (Warning, bool) {
	for _, w := range ws {
		if w.Code == "inert-phase-enable" {
			return w, true
		}
	}
	return Warning{}, false
}

// Despite its name, nothing is enabled here: this is the advisory baseline with no inert warning.
func TestLoad_PlanReviewEnabled_StageAdvisory_NoInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{
		"EVOLVE_DYNAMIC_ROUTING": "advisory",
	})
	if _, ok := inertEnableWarn(ws); ok {
		t.Errorf("inert-warning fired with no enables at Stage=Advisory; got %+v", ws)
	}
}

func TestLoad_SpinePhaseEnabled_NoInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{})
	if _, ok := inertEnableWarn(ws); ok {
		t.Errorf("inert-warning fired with spine defaults; got %+v", ws)
	}
}

func TestLoad_NoEnables_NoInertWarning(t *testing.T) {
	_, ws := Load("", map[string]string{})
	if _, ok := inertEnableWarn(ws); ok {
		t.Errorf("inert-warning fired with no enables; got %+v", ws)
	}
}

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
