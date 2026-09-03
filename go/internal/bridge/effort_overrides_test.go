package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

// effort_overrides: when a tier escalation (model_tier_overrides) lands a
// profile on a deeper tier, the launch carries that tier's effort rung with it.
// The tier is the situation — no second plumbing path.
func TestEffortForTier_OverrideThenDefault(t *testing.T) {
	p := Profile{EffortLevel: "medium", EffortOverrides: map[string]string{"deep": "high", "top": ""}}
	cases := map[string]string{"deep": "high", "balanced": "medium", "top": "medium", "": "medium"}
	for tier, want := range cases {
		if got := p.effortForTier(tier); got != want {
			t.Errorf("effortForTier(%q) = %q, want %q", tier, got, want)
		}
	}
	if got := (Profile{EffortLevel: "low"}).effortForTier("deep"); got != "low" {
		t.Errorf("no overrides map: got %q, want the profile default", got)
	}
}

func TestLoadProfile_ReadsEffortOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "builder.json")
	body := `{"name":"builder","permission_mode":"default","effort_level":"medium","effort_overrides":{"deep":"high"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfile(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.EffortLevel != "medium" || p.EffortOverrides["deep"] != "high" || p.effortForTier("deep") != "high" {
		t.Fatalf("loaded profile = %+v", p)
	}
}
