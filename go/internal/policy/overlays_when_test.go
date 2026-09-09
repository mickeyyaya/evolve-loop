package policy_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// ADR-0099 slice 3 — an overlay rule may key on a cycle SIGNAL (`when`), not
// only on phase/cli/model/tier: a document cycle preloads the solution skills
// into the scout, build and audit dispatches. Absent signal ⇒ the rule does
// not match (fail-closed, the D2 discipline); the selector is deterministic
// config, the advisor may still ADD skills through its clamp.
func TestResolveOverlays_WhenSelector(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{Phases: []string{"build"}, When: []config.Condition{{Field: "deliverable_kind", Op: "eq", Value: "document"}}, Skills: []string{"solution-build"}},
	}}}
	doc := policy.OverlayDispatch{Phase: "build", Signals: map[string]string{"deliverable_kind": "document"}}
	if got := pol.ResolveOverlays(doc); !reflect.DeepEqual(got, []string{"solution-build"}) {
		t.Errorf("document build dispatch = %v, want [solution-build]", got)
	}
	code := policy.OverlayDispatch{Phase: "build", Signals: map[string]string{"deliverable_kind": "code"}}
	if got := pol.ResolveOverlays(code); len(got) != 0 {
		t.Errorf("code build dispatch = %v, want none", got)
	}
	absent := policy.OverlayDispatch{Phase: "build"}
	if got := pol.ResolveOverlays(absent); len(got) != 0 {
		t.Errorf("absent signal must not match (fail-closed); got %v", got)
	}
	other := policy.OverlayDispatch{Phase: "scout", Signals: map[string]string{"deliverable_kind": "document"}}
	if got := pol.ResolveOverlays(other); len(got) != 0 {
		t.Errorf("phase dimension still applies; got %v", got)
	}
	ne := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{When: []config.Condition{{Field: "deliverable_kind", Op: "ne", Value: "document"}}, Skills: []string{"x"}},
	}}}
	if got := ne.ResolveOverlays(policy.OverlayDispatch{}); len(got) != 0 {
		t.Errorf("ne on an absent signal must not fire (fail-closed); got %v", got)
	}
	if got := ne.ResolveOverlays(code); !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("ne on a present non-matching value fires; got %v", got)
	}
	// A non-string clause value (JSON number/bool) never matches in either
	// polarity — the signal plane is string-valued and the selector fails
	// closed instead of coercing.
	nonString := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{When: []config.Condition{{Field: config.SignalDeliverableKind, Op: "eq", Value: 123}}, Skills: []string{"x"}},
		{When: []config.Condition{{Field: config.SignalDeliverableKind, Op: "ne", Value: true}}, Skills: []string{"y"}},
	}}}
	if got := nonString.ResolveOverlays(code); len(got) != 0 {
		t.Errorf("non-string clause values must fail closed in both polarities; got %v", got)
	}
	// Any projected signal is selectable, not only the kind: a rule keyed on
	// the goal type fires on the recipe the scout declared.
	byGoal := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{When: []config.Condition{{Field: config.SignalGoalType, Op: "eq", Value: "partnership-deal"}}, Skills: []string{"deal-lens"}},
	}}}
	deal := policy.OverlayDispatch{Signals: map[string]string{config.SignalDeliverableKind: "document", config.SignalGoalType: "partnership-deal"}}
	if got := byGoal.ResolveOverlays(deal); !reflect.DeepEqual(got, []string{"deal-lens"}) {
		t.Errorf("goal_type keyed rule = %v, want [deal-lens]", got)
	}
}

// TestCompiledDefaultOverlays_SolutionSkillsOnDocumentCycles: with no operator
// overlays block, a document cycle's scout/build/audit dispatches preload the
// solution skills — and the deep-tier fable rule still applies alongside.
func TestCompiledDefaultOverlays_SolutionSkillsOnDocumentCycles(t *testing.T) {
	var pol policy.Policy
	doc := map[string]string{config.SignalDeliverableKind: "document"}
	for phase, want := range map[string]string{"scout": "solution-scout", "build": "solution-build", "audit": "solution-audit"} {
		got := pol.ResolveOverlays(policy.OverlayDispatch{Phase: phase, Tier: "deep", Signals: doc})
		if !reflect.DeepEqual(got, []string{"fable", want}) {
			t.Errorf("document %s at deep = %v, want [fable %s]", phase, got, want)
		}
		if got := pol.ResolveOverlays(policy.OverlayDispatch{Phase: phase, Tier: "balanced", Signals: map[string]string{"deliverable_kind": "code"}}); len(got) != 0 {
			t.Errorf("code %s at balanced = %v, want none (byte-identical to before)", phase, got)
		}
	}
	// Every compiled-default skill exists in the registry (no dangling name).
	registry, err := policy.SkillRegistryFromFS("../../../skills")
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, s := range registry {
		have[s] = true
	}
	for _, s := range []string{"fable", "solution-scout", "solution-build", "solution-audit"} {
		if !have[s] {
			t.Errorf("compiled default names skill %q which is not in skills/", s)
		}
	}
}

// TestCompiledDefaultOverlaySkills: the compiled-default skill set is exported
// as ONE list so the integrity guard (ProtectedSurfaceManifest) can be pinned
// to it — every persona the kernel preloads on its own authority is
// control-plane, and the next compiled skill cannot skip the manifest.
func TestCompiledDefaultOverlaySkills(t *testing.T) {
	got := policy.CompiledDefaultOverlaySkills()
	want := []string{"fable", "solution-scout", "solution-build", "solution-audit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CompiledDefaultOverlaySkills() = %v, want %v", got, want)
	}
	for _, s := range got {
		if _, err := os.Stat(filepath.Join("..", "..", "..", "skills", s, "SKILL.md")); err != nil {
			t.Errorf("compiled-default skill %q has no skills/%s/SKILL.md: %v", s, s, err)
		}
	}
}
