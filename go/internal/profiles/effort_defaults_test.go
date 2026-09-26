package profiles

import (
	"path/filepath"
	"runtime"
	"testing"
)

func effortProfilesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".evolve", "profiles")
}

func TestEffortDefaults_Matrix(t *testing.T) {
	loader := NewFromDir(effortProfilesDir(t))
	// The claude-routed graders run xhigh, above codex's deep/top rung, by design.
	want := map[string]string{
		"scout":              "low",
		"triage":             "low",
		"tdd-engineer":       "medium",
		"builder":            "medium",
		"auditor":            "xhigh",
		"adversarial-review": "xhigh",
		"retrospective":      codexDeepTopRung,
		"premise-challenge":  codexDeepTopRung,
		"intent":             codexDeepTopRung,
	}
	for profile, effort := range want {
		p, err := loader.Get(profile)
		if err != nil {
			t.Fatalf("Get(%s): %v", profile, err)
		}
		if p.EffortLevel != effort {
			t.Errorf("profile %s: effort_level = %q, want %q (committed per-phase effort matrix)", profile, p.EffortLevel, effort)
		}
	}
}

// codexDeepTopRung is the effort level every codex deep/top profile must declare.
const codexDeepTopRung = "high"

// maxEffortRung is the costliest effort level, reserved for deep/top profiles.
const maxEffortRung = "max"

// The guard reads the declared tier; a dispatch-time escalation changes effort
// only through the profile's effort_overrides.
func TestCodexDeepTierProfilesAllRunAtDirectedRung(t *testing.T) {
	loader, names := RealTreeProfiles(t)
	checked := 0
	for _, name := range names {
		p, err := loader.Get(name)
		if err != nil {
			// Report, don't skip: Get also expands policies, so a skip would
			// silently shrink the checked set.
			t.Errorf("profile %s: Get failed (%v) — it cannot be checked, so it cannot be trusted", name, err)
			continue
		}
		if p.CLI != "codex-tmux" || (p.ModelTierDefault != "deep" && p.ModelTierDefault != "top") {
			continue
		}
		checked++
		if p.EffortLevel != codexDeepTopRung {
			t.Errorf("profile %s: codex %s-tier effort_level = %q, want %q (2026-09-01 operator directive). A codex deep/top phase off the directed rung runs differently than its siblings with nothing reporting it.",
				name, p.ModelTierDefault, p.EffortLevel, codexDeepTopRung)
		}
	}
	if checked == 0 {
		t.Fatal("matched NO tracked codex deep/top profiles — the selector is broken and this guard is vacuous")
	}
}

// The rule holds for every CLI family: max is the costliest level wherever it
// exists, and a fast or balanced phase was costed at a lower one.
func TestMaxEffortOnlyOnDeepOrTopProfiles(t *testing.T) {
	loader, names := RealTreeProfiles(t)
	checked := 0
	for _, name := range names {
		p, err := loader.Get(name)
		if err != nil {
			// Report, don't skip: Get also expands policies, so a skip would
			// silently shrink the checked set.
			t.Errorf("profile %s: Get failed (%v) — it cannot be checked, so it cannot be trusted", name, err)
			continue
		}
		if p.EffortLevel != maxEffortRung {
			continue
		}
		checked++
		// An unset model_tier_default resolves to balanced at dispatch.
		tier := p.ModelTierDefault
		if tier == "" {
			tier = "balanced"
		}
		if tier != "deep" && tier != "top" {
			t.Errorf("profile %s (cli %s): effort_level %q on a %q-tier phase — max is reserved for deep/top models (2026-08-29 directive). It is the most expensive rung; a fast/balanced phase was costed that way deliberately.",
				name, p.CLI, p.EffortLevel, tier)
		}
	}
	if checked == 0 {
		// Zero matches is expected; the sibling class guard catches
		// EffortLevel decode regressions.
		t.Logf("no tracked profile at effort %q — expected since the 2026-09-01 directive; placement law armed for future adoption", maxEffortRung)
	}
}

func TestEffortOverrides_PinnedToDirectiveRungs(t *testing.T) {
	loader, _ := RealTreeProfiles(t)
	want := map[string]map[string]string{
		"builder":      {"deep": codexDeepTopRung}, // codex-routed
		"tdd-engineer": {"deep": "xhigh"},          // claude-routed, like the graders
	}
	for profile, overrides := range want {
		p, err := loader.Get(profile)
		if err != nil {
			t.Fatalf("Get(%s): %v", profile, err)
		}
		for tier, rung := range overrides {
			if got := p.EffortOverrides[tier]; got != rung {
				t.Errorf("profile %s: effort_overrides[%s] = %q, want %q (committed directive rung)", profile, tier, got, rung)
			}
		}
	}
}
