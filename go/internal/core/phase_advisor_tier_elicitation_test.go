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

// The PRODUCTION persona path (composePlanPrompt with a non-empty persona)
// must show the SAME optional per-phase {cli,tier} schema example that the
// legacy buildPlanPrompt fallback carries. The example must attach cli/tier
// to an EXISTING phase entry (no "mint" block on that entry), proving the
// elicitation is single-sourced across both prompt-assembly paths rather than
// re-forked.
func TestComposePlanPrompt_ElicitsTierAndCLI(t *testing.T) {
	t.Parallel()
	p := NewPhaseAdvisor(nil, WithPersona("PERSONA BODY"))
	got := p.composePlanPrompt(baseRouteInput(), "routing-plan.json")

	if !strings.Contains(got, `"cli":`) || !strings.Contains(got, `"tier":"balanced"`) {
		t.Errorf("persona-path plan prompt missing the {cli,tier} schema/example; got:\n%s", got)
	}
	// The cli/tier examples must appear on an EXISTING-phase entry, not only
	// inside a "mint" block — find the JSON object carrying "tier" and assert
	// it has no "mint" key alongside it.
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
		t.Errorf("the {cli,tier} example must attach to an EXISTING phase (no \"mint\" block); object:\n%s", obj)
	}
}

// The persona-path prompt must carry the operator's model-tier policy
// guidance — deep for judgment-heavy phases, fast confined to
// mechanical-only phases — so the advisor never proposes fast for a phase
// that writes source or renders a verdict.
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

// A phase spec declaring dispatch guardrails (allowed_clis +
// model_tier_envelope, the per-phase profile contracts phase-registry.json
// carries) must have them projected onto the advisor-facing PhaseCard so
// writeCatalog's rendering fires instead of always-nil fields.
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
	if len(c.AllowedCLIs) != 2 || c.AllowedCLIs[0] != "claude" {
		t.Errorf("card.AllowedCLIs = %v, want [claude codex] projected from the spec", c.AllowedCLIs)
	}
	if c.ModelTierEnvelope == nil || c.ModelTierEnvelope.Min != "balanced" || c.ModelTierEnvelope.Max != "deep" {
		t.Errorf("card.ModelTierEnvelope = %+v, want {min:balanced max:deep} projected from the spec", c.ModelTierEnvelope)
	}
}

// Once a catalog phase carries dispatch guardrails, the persona-path
// composed prompt must show the `allowed_clis:` and `model_tier_envelope:`
// lines writeCatalog knows how to render — closing the loop from
// phase-registry.json's contract to the rendered prompt.
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

	if !strings.Contains(got, "allowed_clis: claude") {
		t.Errorf("composed prompt missing allowed_clis guardrail line; got:\n%s", got)
	}
	if !strings.Contains(got, "model_tier_envelope: {min: balanced") {
		t.Errorf("composed prompt missing model_tier_envelope guardrail line; got:\n%s", got)
	}
}

// With an ACTIVE environmental bench for a CLI family whose reason
// indicates full quota exhaustion (not a soft rate-limit blip), the composed
// plan prompt must name it as WALLED/unavailable rather than the generic
// "benched (<reason>) until <time>" wording.
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

// An advisor response shaped like the legacy wire format —
// {phase,run,justification} only, no cli/tier keys — must parse to entries
// whose CLI/Tier are empty, so nothing downstream ever applies a dispatch
// overlay for them (llmroute.ApplySoftOverlay is gated on non-empty CLI/Tier).
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
	if plan.Entries[0].CLI != "" || plan.Entries[0].Tier != "" {
		t.Errorf("legacy-shape entry = %+v, want empty CLI/Tier (no overlay proposed)", plan.Entries[0])
	}
}

// An advisor response entry proposing "high" or a raw model name must never
// propagate past sanitizeAdvisorTier — only the canonical tier vocabulary
// survives (see TestSanitizeAdvisorTier in phase_advisor_tier_test.go for the
// accepted table).
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

// realRouterPersona reads the SHIPPED agents/evolve-router.md exactly as the
// production planner does (cmd_cycle.go: prm.Agent("evolve-router").Body —
// the frontmatter is parsed off and the body is injected as the persona) and
// returns both halves. The path is resolved off runtime.Caller so it is
// cwd-independent, and it points at the WORKTREE copy, not main's stale copy.
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

// topLevelJSONObjects returns every balanced top-level {...} substring in s. A
// nested object (e.g. a "mint" block) stays INSIDE its parent object and is never
// emitted on its own, so each returned string can be classified as a whole
// response-schema entry.
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

// existingPhaseExamples filters topLevelJSONObjects to the response-schema
// examples for an EXISTING phase: a plan entry (has "phase" and "run") that is
// NOT a mint block. These are exactly the objects whose {cli,tier} shape the
// advisor mimics; the bare persona example in agents/evolve-router.md is one.
func existingPhaseExamples(s string) []string {
	var out []string
	for _, o := range topLevelJSONObjects(s) {
		if strings.Contains(o, `"phase"`) && strings.Contains(o, `"run"`) && !strings.Contains(o, `"mint"`) {
			out = append(out, o)
		}
	}
	return out
}

// The PRODUCTION composed plan prompt, built with the REAL shipped persona,
// must show {cli,tier} on EVERY existing-phase response-schema example — no
// surviving bare example the advisor could mimic. The >=2 floor is the
// anti-delete guard: silently deleting the persona example (leaving only the
// Go-appended one) must NOT green the golden — the persona itself must teach
// the tiered schema.
func TestComposePlanPrompt_RealPersonaExistingExampleCarriesTierAndCLI(t *testing.T) {
	t.Parallel()
	_, body := realRouterPersona(t)
	p := NewPhaseAdvisor(nil, WithPersona(body))
	got := p.composePlanPrompt(baseRouteInput(), "routing-plan.json")

	examples := existingPhaseExamples(got)
	if len(examples) < 2 {
		t.Fatalf("found %d existing-phase schema example(s) in the real-persona prompt, want >=2 (the persona's own example + the Go-appended one); deleting the persona example is not a valid harmonization:\n%s", len(examples), got)
	}
	for _, ex := range examples {
		if !strings.Contains(ex, `"tier"`) || !strings.Contains(ex, `"cli"`) {
			t.Errorf("existing-phase schema example lacks {cli,tier} — a competing bare example the advisor will mimic:\n%s", ex)
		}
	}
}

// The persona frontmatter's output-format contract string must enumerate
// cli/tier alongside phase/run/justification, so the one-line schema summary
// agrees with the body example and the Go schema — a fix to the body example
// alone must not silently satisfy this frontmatter requirement too.
func TestRealPersonaFrontmatterOutputFormatEnumeratesTierCLI(t *testing.T) {
	t.Parallel()
	fm, _ := realRouterPersona(t)
	of, ok := fm["output-format"].(string)
	if !ok {
		t.Fatalf("frontmatter output-format missing or not a string: %#v", fm["output-format"])
	}
	if !strings.Contains(of, "cli") || !strings.Contains(of, "tier") {
		t.Errorf("frontmatter output-format must enumerate cli/tier; got: %q", of)
	}
}
