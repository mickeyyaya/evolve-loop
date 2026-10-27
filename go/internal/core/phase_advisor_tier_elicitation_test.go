package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestComposePlanPrompt_ElicitsATierAndNoCLI(t *testing.T) {
	t.Parallel()
	p := NewPhaseAdvisor(nil, WithPersona("PERSONA BODY"))
	got := p.composePlanPrompt(baseRouteInput(), "routing-plan.json")

	if strings.Contains(got, `"cli":`) || !strings.Contains(got, `"tier":"balanced"`) {
		t.Errorf("persona-path plan prompt must show the tier example and never a cli key; got:\n%s", got)
	}
	tierIdx := strings.Index(got, `"tier":"balanced"`)
	if tierIdx < 0 {
		t.Fatalf("no tier example found; got:\n%s", got)
	}
	objStart := strings.LastIndex(got[:tierIdx], "{")
	objEnd := strings.Index(got[tierIdx:], "}")
	if objStart < 0 || objEnd < 0 {
		t.Fatalf("could not isolate the JSON object carrying the tier example; got:\n%s", got)
	}
	obj := got[objStart : tierIdx+objEnd]
	if strings.Contains(obj, "mint") {
		t.Errorf("the tier example must attach to an EXISTING phase (no \"mint\" block); object:\n%s", obj)
	}
}

func TestComposePlanPrompt_RendersOperatorModelPolicy(t *testing.T) {
	t.Parallel()
	p := NewPhaseAdvisor(nil, WithPersona("PERSONA BODY"))
	got := p.composePlanPrompt(baseRouteInput(), "routing-plan.json")

	if !strings.Contains(got, "fast") || !strings.Contains(got, "mechanical") {
		t.Errorf("plan prompt must gate \"fast\" to mechanical-only phases; got:\n%s", got)
	}
	if !strings.Contains(got, "deep") || !strings.Contains(strings.ToLower(got), "judgment") {
		t.Errorf("plan prompt must recommend \"deep\" for judgment-heavy phases; got:\n%s", got)
	}
}

func TestPhaseCardsFromCatalog_ProjectsDispatchGuardrails(t *testing.T) {
	t.Parallel()
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{
			Name: "build", WritesSource: true,
			AllowedCLIs:       []string{"claude", "codex"},
			ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Default: "balanced", Max: "deep"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cards := phaseCardsFromCatalog(cat)
	if len(cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(cards))
	}
	c := cards[0]
	if c.ModelTierEnvelope == nil || c.ModelTierEnvelope.Min != "balanced" || c.ModelTierEnvelope.Max != "deep" {
		t.Errorf("card.ModelTierEnvelope = %+v, want {min:balanced max:deep} projected from the spec", c.ModelTierEnvelope)
	}
}

func TestComposePlanPrompt_RendersGuardrailLinesForCatalogPhase(t *testing.T) {
	t.Parallel()
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{
			Name: "security-scan", Optional: true, Role: "evaluate",
			AllowedCLIs:       []string{"claude"},
			ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	in := baseRouteInput()
	in.Catalog = phaseCardsFromCatalog(cat)
	p := NewPhaseAdvisor(nil, WithPersona("PERSONA BODY"))
	got := p.composePlanPrompt(in, "routing-plan.json")

	if strings.Contains(got, "allowed_clis") {
		t.Errorf("composed prompt shows the advisor a CLI list; the advisor proposes a tier only; got:\n%s", got)
	}
	if !strings.Contains(got, "model_tier_envelope: {min: balanced") {
		t.Errorf("composed prompt missing model_tier_envelope guardrail line; got:\n%s", got)
	}
}

func TestComposePlanPrompt_NamesBenchedCLIAsWalled(t *testing.T) {
	t.Parallel()
	in := baseRouteInput()
	in.BenchedCLIs = []router.BenchedCLI{
		{Family: "codex", Reason: "quota_exhausted", Until: time.Date(2026, 7, 2, 15, 4, 0, 0, time.UTC)},
	}
	p := NewPhaseAdvisor(nil, WithPersona("PERSONA BODY"))
	got := p.composePlanPrompt(in, "routing-plan.json")

	if !strings.Contains(got, "codex") {
		t.Fatalf("composed prompt missing the benched family name; got:\n%s", got)
	}
	lower := strings.ToLower(got)
	if !strings.Contains(lower, "walled") && !strings.Contains(lower, "unavailable") {
		t.Errorf("composed prompt must name a quota-exhausted CLI as WALLED/unavailable, not just \"benched\"; got:\n%s", got)
	}
}

func TestParsePhasePlan_AbsentCLITierFieldsStayEmpty(t *testing.T) {
	t.Parallel()
	raw := `[{"phase":"build","run":true,"justification":"legacy shape, no cli/tier"}]`
	plan, err := parsePhasePlan(raw)
	if err != nil {
		t.Fatalf("parsePhasePlan: %v", err)
	}
	if len(plan.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(plan.Entries))
	}
	if plan.Entries[0].Tier != "" {
		t.Errorf("legacy-shape entry = %+v, want an empty tier (no overlay proposed)", plan.Entries[0])
	}
}

func TestSanitizeAdvisorTier_RejectsHighAndRawModel(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"high", "claude-fable-5", "opus", "HIGH", ""} {
		if got := sanitizeAdvisorTier(bad); got != "" {
			t.Errorf("sanitizeAdvisorTier(%q) = %q, want \"\" (only fast/balanced/deep/top survive)", bad, got)
		}
	}
	for _, good := range []string{"fast", "balanced", "deep", "top"} {
		if got := sanitizeAdvisorTier(good); got != good {
			t.Errorf("sanitizeAdvisorTier(%q) = %q, want unchanged", good, got)
		}
	}
}

func realRouterPersona(t *testing.T) (frontmatter map[string]any, body string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate the test file to resolve agents/evolve-router.md")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-router.md"))
	if err != nil {
		t.Fatalf("read agents/evolve-router.md: %v", err)
	}
	fm, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter(agents/evolve-router.md): %v", err)
	}
	return fm, body
}

func topLevelJSONObjects(s string) []string {
	var objs []string
	depth, start := 0, -1
	for i, r := range s {
		switch r {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					objs = append(objs, s[start:i+1])
					start = -1
				}
			}
		}
	}
	return objs
}

func existingPhaseExamples(s string) []string {
	var out []string
	for _, o := range topLevelJSONObjects(s) {
		if strings.Contains(o, `"phase"`) && strings.Contains(o, `"run"`) && !strings.Contains(o, `"mint"`) {
			out = append(out, o)
		}
	}
	return out
}

func TestComposePlanPrompt_RealPersonaExistingExampleCarriesATierAndNoCLI(t *testing.T) {
	t.Parallel()
	_, body := realRouterPersona(t)
	p := NewPhaseAdvisor(nil, WithPersona(body))
	got := p.composePlanPrompt(baseRouteInput(), "routing-plan.json")

	examples := existingPhaseExamples(got)
	if len(examples) < 2 {
		t.Fatalf("found %d existing-phase schema example(s) in the real-persona prompt, want >=2 (the persona's own example + the Go-appended one); deleting the persona example is not a valid harmonization:\n%s", len(examples), got)
	}
	for _, ex := range examples {
		if !strings.Contains(ex, `"tier"`) || strings.Contains(ex, `"cli"`) {
			t.Errorf("existing-phase schema example must carry the tier and no cli — the advisor mimics the example:\n%s", ex)
		}
	}
}

func TestRealPersonaFrontmatterOutputFormatEnumeratesTheTierOnly(t *testing.T) {
	t.Parallel()
	fm, _ := realRouterPersona(t)
	of, ok := fm["output-format"].(string)
	if !ok {
		t.Fatalf("frontmatter output-format missing or not a string: %#v", fm["output-format"])
	}
	if strings.Contains(of, "[cli]") || !strings.Contains(of, "[tier]") {
		t.Errorf("frontmatter output-format must enumerate the tier and no cli field; got: %q", of)
	}
}
