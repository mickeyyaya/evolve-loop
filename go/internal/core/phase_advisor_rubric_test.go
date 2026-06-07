package core

// phase_advisor_rubric_test.go — failure floor Phase 4b: the advisor's
// decision rubric is a PROJECTION of the structured routing data the kernel
// already walks — insert_when triggers and conditional_mandatory rules render
// into prose by ONE renderer, so a threshold can never disagree between the
// walk and the prompt. registry routing.rubric_hint carries ONLY judgment
// guidance with no structured counterpart. The FORBIDDEN kernel rule (never
// ship without audit) stays in code — it is an invariant, not phase data.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// Insert lines derive from the SAME structured insert_when conditions the
// kernel walks — never restated as prose data.
func TestRubric_InsertLinesDerivedFromStructuredTriggers(t *testing.T) {
	t.Parallel()
	in := router.RouteInput{Cfg: config.RoutingConfig{
		Triggers: map[string]config.RoutingBlock{
			"tester": {InsertWhen: []config.Condition{
				{Field: "build.acs_red", Op: "gt", Value: 0},
				{Field: "build.severity_max", Op: "gte", Value: "HIGH"},
			}},
		},
	}}
	got := buildRoutingPrompt(in)
	want := "- build.acs_red > 0 OR build.severity_max >= HIGH → insert tester"
	if !strings.Contains(got, want) {
		t.Errorf("rubric missing derived insert line %q\n---\n%s", want, got)
	}
}

// Skip-exemption lines derive from conditional_mandatory rules (op negated:
// the rule says when the phase is PINNED; the rubric tells the advisor when
// it may be skipped).
func TestRubric_SkipExemptionDerivedFromConditional(t *testing.T) {
	t.Parallel()
	in := router.RouteInput{Cfg: config.RoutingConfig{
		Conditional: map[string]config.CondRule{
			"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"},
		},
	}}
	got := buildRoutingPrompt(in)
	want := "- cycle_size == trivial → skip tdd (conditional-mandatory exemption)"
	if !strings.Contains(got, want) {
		t.Errorf("rubric missing derived skip-exemption %q\n---\n%s", want, got)
	}
}

// Judgment-only guidance (no structured counterpart) renders verbatim from
// registry routing.rubric_hint, phases sorted (deterministic prompt ⇒
// prompt-prefix cache friendly).
func TestRubric_JudgmentHintsRenderFromRegistry(t *testing.T) {
	t.Parallel()
	archHint := "a novel/cross-cutting goal also warrants architecture-design"
	scoutHint := "scout.item_count == 0 → end cycle early (no-ship is legitimate)"
	in := router.RouteInput{Cfg: config.RoutingConfig{
		Triggers: map[string]config.RoutingBlock{
			"scout":               {RubricHint: []string{scoutHint}},
			"architecture-design": {RubricHint: []string{archHint}},
		},
	}}
	got := buildRoutingPrompt(in)
	for _, want := range []string{"- " + archHint, "- " + scoutHint} {
		if !strings.Contains(got, want) {
			t.Errorf("rubric missing registry hint %q\n---\n%s", want, got)
		}
	}
	if ai, si := strings.Index(got, archHint), strings.Index(got, scoutHint); ai > si {
		t.Errorf("rubric hints not phase-sorted: architecture-design at %d after scout at %d", ai, si)
	}
	// The legacy hardcoded lines are gone — phases absent from this cfg
	// contribute nothing.
	if strings.Contains(got, "carryover_count") {
		t.Error("rubric still carries the hardcoded scout.carryover_count line")
	}
	// The kernel invariant is NOT phase data and stays hardcoded.
	if !strings.Contains(got, "FORBIDDEN: never propose reaching ship without audit") {
		t.Error("rubric lost the hardcoded FORBIDDEN kernel rule")
	}
}

// A cfg with no rubric sources renders just the header + kernel rule — no
// stray bullets from code.
func TestRubric_EmptyWithoutRegistrySources(t *testing.T) {
	t.Parallel()
	got := buildRoutingPrompt(router.RouteInput{})
	if strings.Contains(got, "→ skip scout") || strings.Contains(got, "insert plan-review") {
		t.Errorf("source-less cfg must render no rubric bullets\n---\n%s", got)
	}
	if !strings.Contains(got, "FORBIDDEN: never propose reaching ship without audit") {
		t.Error("kernel rule must render even with no sources")
	}
}

// The two op vocabularies: Condition ops arrive word-form OR symbolic from
// JSON (opSymbol renders both); CondRule ops are symbolic from parseCondRule
// but negateOp accepts word-form too, so an in-process caller can never
// silently lose an exemption line.
func TestRubric_OpVocabularies(t *testing.T) {
	t.Parallel()
	symbol := map[string]string{"eq": "==", "ne": "!=", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}
	negation := map[string]string{"==": "!=", "!=": "==", ">": "<=", ">=": "<", "<": ">=", "<=": ">"}
	for word, sym := range symbol {
		if got := opSymbol(word); got != sym {
			t.Errorf("opSymbol(%q) = %q, want %q", word, got, sym)
		}
		if got := opSymbol(sym); got != sym {
			t.Errorf("opSymbol(%q) = %q, want identity", sym, got)
		}
		wantNeg := negation[sym]
		if got, ok := negateOp(word); !ok || got != wantNeg {
			t.Errorf("negateOp(%q) = %q,%v, want %q,true", word, got, ok, wantNeg)
		}
		if got, ok := negateOp(sym); !ok || got != wantNeg {
			t.Errorf("negateOp(%q) = %q,%v, want %q,true", sym, got, ok, wantNeg)
		}
	}
	if _, ok := negateOp("matches"); ok {
		t.Error("negateOp must refuse unknown ops (no line beats a wrong line)")
	}
}

// The failure vocabulary names the retry-path insert phases from the
// router's kernel map (the ONE source that also enforces them) — never a
// second hardcoded list.
func TestFailureVocabulary_InsertPhasesFromKernelMap(t *testing.T) {
	t.Parallel()
	got := buildRoutingPrompt(router.RouteInput{Current: "retrospective", Verdict: "FAIL"})
	for _, p := range router.FailureInsertPhases() {
		if !strings.Contains(got, p) {
			t.Errorf("failure vocabulary missing kernel insert phase %q", p)
		}
	}
}
