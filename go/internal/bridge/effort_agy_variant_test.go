package bridge

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestAgyClaudeEffortSelectsTheModelVariant(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	m, err := LoadManifest("agy-claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	cases := []struct {
		tier, effort, wantModel, wantCapped string
	}{
		{"fast", "low", "Claude Sonnet 5.5 (Low)", ""},
		{"balanced", "medium", "Claude Sonnet 5.5 (Medium)", ""},
		{"balanced", "high", "Claude Sonnet 5.5 (High)", ""},
		{"deep", "low", "Claude Opus 5.5 (Low)", ""},
		{"deep", "medium", "Claude Opus 5.5 (Medium)", ""},
		{"deep", "high", "Claude Opus 5.5 (High)", ""},
		{"top", "xhigh", "Claude Opus 5.5 (High)", "xhigh"},
		{"top", "max", "Claude Opus 5.5 (High)", "max"},
		{"deep", "", "Claude Opus 5.5 (High)", ""},
	}
	for _, tc := range cases {
		t.Run(tc.tier+"/"+tc.effort, func(t *testing.T) {
			r := Realize(m, LaunchIntent{ModelTier: tc.tier, Effort: tc.effort})
			i := slices.Index(r.LaunchFlags, "--model")
			if i < 0 || i+1 >= len(r.LaunchFlags) {
				t.Fatalf("no --model in %v", r.LaunchFlags)
			}
			if got := r.LaunchFlags[i+1]; got != tc.wantModel {
				t.Errorf("--model = %q, want %q", got, tc.wantModel)
			}
			if r.EffortCapped != tc.wantCapped {
				t.Errorf("EffortCapped = %q, want %q", r.EffortCapped, tc.wantCapped)
			}
		})
	}
}

func TestModelVariant_LeavesANameWithoutAVariantAlone(t *testing.T) {
	m := Manifest{
		CLI:          "variant-fixture",
		ModelTierMap: map[string]string{"deep": "claude-opus-5-5"},
		Params: map[string]ParamSpec{
			"model_tier": {Channel: "flag", Flag: "--model", From: "model_tier_map"},
			"effort":     {Channel: effortChannelModelVariant, Values: map[string][]string{"medium": {"Medium"}}},
		},
	}
	r := Realize(m, LaunchIntent{ModelTier: "deep", Effort: "medium"})
	if !slices.Equal(r.LaunchFlags, []string{"--model", "claude-opus-5-5"}) || r.EffortCapped != "" {
		t.Fatalf("flags = %v capped = %q, want the name unchanged and no cap", r.LaunchFlags, r.EffortCapped)
	}
}

func TestClaudeTmuxRealizationNeverCapsEffort(t *testing.T) {
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if r := Realize(m, LaunchIntent{ModelTier: "top", Effort: "max"}); r.EffortCapped != "" {
		t.Fatalf("claude-tmux EffortCapped = %q, want none", r.EffortCapped)
	}
}

func TestAgyClaudeLaunchLogsOneLineWhenTheEffortIsCapped(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	cases := []struct {
		effort    string
		wantLines int
	}{
		{"xhigh", 1},
		{"max", 1},
		{"high", 0},
		{"medium", 0},
	}
	for _, tc := range cases {
		t.Run(tc.effort, func(t *testing.T) {
			var stderr bytes.Buffer
			bootAgyClaudeLogged(t, LaunchIntent{ModelTier: "top", Permission: "bypass", Effort: tc.effort}, &stderr)
			want := "effort=" + tc.effort + " is capped to the (High) model variant"
			if got := strings.Count(stderr.String(), want); got != tc.wantLines {
				t.Errorf("stderr has %d cap lines %q, want %d; stderr:\n%s", got, want, tc.wantLines, stderr.String())
			}
		})
	}
}

func TestAgyClaudeTierMapSuffixIsTheProjectionOfTheCompiledTierEffort(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	m, err := LoadManifest("agy-claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	efforts := policy.BridgePolicy{}.TierEfforts()
	for _, tier := range policy.TierNames() {
		want := modelVariantSuffix(m.Params["effort"].Values[efforts[tier]])
		if got := m.ModelTierMap[tier]; want == "" || !strings.HasSuffix(got, want) {
			t.Errorf("model_tier_map[%s] = %q, want the suffix %q that the compiled effort %q selects", tier, got, want, efforts[tier])
		}
	}
}
