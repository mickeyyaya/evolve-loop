package profiles

import "testing"

func TestEffortDefaults_Matrix(t *testing.T) {
	loader := NewFromDir(RealProfilesDir(t))
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
		"router":             "medium",
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

var premiumEffortRungs = map[string]bool{"max": true, "ultra": true}

func violatesPremiumPlacement(p Profile) bool {
	if !premiumEffortRungs[p.EffortLevel] {
		return false
	}
	return p.ModelTierDefault != "deep" && p.ModelTierDefault != "top"
}

func TestViolatesPremiumPlacement(t *testing.T) {
	cases := []struct {
		cli, tier, effort string
		want              bool
	}{
		{"codex-tmux", "balanced", "ultra", true},
		{"codex-tmux", "", "ultra", true},
		{"codex-tmux", "fast", "ultra", true},
		{"codex-tmux", "balanced", "max", true},
		{"codex-tmux", "fast", "max", true},
		{"claude-tmux", "", "max", true},
		{"codex-tmux", "deep", "ultra", false},
		{"codex-tmux", "top", "ultra", false},
		{"codex-tmux", "top", "max", false},
		{"claude-tmux", "deep", "max", false},
		{"codex-tmux", "balanced", "xhigh", false},
		{"codex-tmux", "fast", "low", false},
		{"codex-tmux", "balanced", "", false},
		{"codex-tmux", "deep", "high", false},
	}
	for _, c := range cases {
		p := Profile{Name: "table", CLI: c.cli, ModelTierDefault: c.tier, EffortLevel: c.effort}
		if got := violatesPremiumPlacement(p); got != c.want {
			t.Errorf("violatesPremiumPlacement(cli=%s tier=%q effort=%q) = %v, want %v", c.cli, c.tier, c.effort, got, c.want)
		}
	}
}

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

func TestPremiumEffortOnlyOnDeepOrTopProfiles(t *testing.T) {
	loader, names := RealTreeProfiles(t)
	for _, name := range names {
		p, err := loader.Get(name)
		if err != nil {
			t.Errorf("profile %s: Get failed (%v) — it cannot be checked, so it cannot be trusted", name, err)
			continue
		}
		if violatesPremiumPlacement(p) {
			t.Errorf("profile %s (cli %s): effort_level %q on tier %q (unset resolves to balanced) — the premium rungs max and ultra are reserved for deep/top models. They are the most expensive rungs; a fast/balanced phase was costed lower deliberately.",
				name, p.CLI, p.EffortLevel, p.ModelTierDefault)
		}
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
