package modelquery

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// TestBuildClassifyPrompt_CoversEveryCanonicalTier: the prompt's tier block
// and JSON template are GENERATED from modelcatalog.CanonicalTiers, so a
// canonical tier can never again be silently omitted (the original prompt
// hardcoded fast/balanced/deep and every refresh deleted tier_models.top).
// Iterates the live vocabulary rather than hardcoding it — a future fifth
// tier is covered automatically.
func TestBuildClassifyPrompt_CoversEveryCanonicalTier(t *testing.T) {
	t.Parallel()
	prompt := buildClassifyPrompt("codex", []string{"gpt-5.5", "gpt-5.5-mini"})
	for _, tier := range modelcatalog.CanonicalTiers {
		if !strings.Contains(prompt, tier+" ") && !strings.Contains(prompt, tier+"=") && !strings.Contains(prompt, tier+"\t") && !strings.Contains(prompt, tier+":") && !strings.Contains(prompt, `"`+tier+`"`) {
			t.Errorf("prompt does not mention tier %q", tier)
		}
		if !strings.Contains(prompt, `"`+tier+`":"<id>"`) {
			t.Errorf("JSON template lacks %q slot", tier)
		}
		if tierBriefs[tier] == "" {
			t.Errorf("tierBriefs lacks a brief for canonical tier %q — the generated prompt would carry an empty judgment line", tier)
		}
	}
}

// TestCompleteTiers_NearestNeighbourLadder: a tier the classifier omitted is
// filled from its nearest present neighbour in CanonicalTiers order,
// preferring the more-capable side on distance ties. Completion only reuses
// ids sanitizeTierMap already validated — it never invents one.
func TestCompleteTiers_NearestNeighbourLadder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   map[string]string
		want map[string]string
	}{
		{
			name: "missing top fills from deep",
			in:   map[string]string{"fast": "m1", "balanced": "m2", "deep": "m3"},
			want: map[string]string{"fast": "m1", "balanced": "m2", "deep": "m3", "top": "m3"},
		},
		{
			name: "missing balanced ties to the more capable neighbour (deep)",
			in:   map[string]string{"fast": "m1", "deep": "m3", "top": "m4"},
			want: map[string]string{"fast": "m1", "balanced": "m3", "deep": "m3", "top": "m4"},
		},
		{
			name: "single entry fans out everywhere",
			in:   map[string]string{"deep": "only"},
			want: map[string]string{"fast": "only", "balanced": "only", "deep": "only", "top": "only"},
		},
		{
			name: "empty input has nothing to borrow",
			in:   map[string]string{},
			want: map[string]string{},
		},
	}
	for _, c := range cases {
		got := CompleteTiers(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %#v, want %#v", c.name, got, c.want)
			continue
		}
		for tier, model := range c.want {
			if got[tier] != model {
				t.Errorf("%s: [%s] = %q, want %q", c.name, tier, got[tier], model)
			}
		}
	}
	// Purity: input map is never mutated.
	in := map[string]string{"deep": "only"}
	CompleteTiers(in)
	if len(in) != 1 {
		t.Error("CompleteTiers mutated its input")
	}
}
