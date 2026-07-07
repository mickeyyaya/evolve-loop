package policy_test

// advisor_skill_overlay_test.go — RED tests for the cycle-613 task
// advisor-skill-selection (.evolve/inbox/2026-07-07T18-30-00Z-advisor-skill-selection.json,
// triage top_n, weight 0.92): the advisor gains authority to PROPOSE per-phase
// skill sets, following the EXACT {cli,tier} soft-overlay precedent
// (phases/runner/runner.go:429-447, router.ClampPlanModelRouting) — "advisor
// proposes, kernel disposes". This file pins the policy-layer clamp/merge/
// registry contract; production types (AdvisorOverlayPolicy,
// AdvisorSkillRejection, Policy.ClampAdvisorSkills,
// Policy.ResolveOverlaysWithAdvisor, SkillRegistryFromFS) do not exist yet —
// RED now: internal/policy fails to compile.
//
// Scope note: this file covers the policy-layer clamp/merge/registry
// contract only (AC1 additive-merge, AC2 clamp matrix, AC3 injection guard,
// AC5 filesystem-sourced registry). The dispatch wiring into Engine.Launch /
// PhaseRequest / the advisor prompt's registry section, the
// advisor-rejections.json artifact plumbing, and the StaticPrefix
// cache-contract round-trip (AC4) are dispositioned manual+checklist in
// test-report.md — they compose EXISTING shipped mechanisms
// (RejectionsFromClamps, StaticPrefix, BaseCycleContext) verbatim once this
// clamp/merge/registry contract lands, so no new predicate is needed for
// them; Auditor verifies the wiring by inspection per the checklist.

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// writeFixtureSkills creates skillsDir/<name>/SKILL.md for each name and
// returns skillsDir, so registry tests exercise a real filesystem walk
// rather than an in-memory list (drift-proof registry, per AC5).
func writeFixtureSkills(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		skillDir := filepath.Join(dir, n)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", skillDir, err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+n+"\n"), 0o644); err != nil {
			t.Fatalf("write SKILL.md for %s: %v", n, err)
		}
	}
	return dir
}

// TestSkillRegistryFromFS_EnumeratesSkillDirs: the registry is READ from the
// filesystem (skills/<name>/SKILL.md), not hand-maintained — AC5 "advisor
// prompt lists the skill registry from the filesystem (no hand-maintained
// list; drift-proof)". A directory without a SKILL.md is not a skill and
// must not appear.
func TestSkillRegistryFromFS_EnumeratesSkillDirs(t *testing.T) {
	dir := writeFixtureSkills(t, "engineering-craft", "fable-mode", "adversarial-testing")
	if err := os.MkdirAll(filepath.Join(dir, "not-a-skill"), 0o755); err != nil {
		t.Fatalf("mkdir stray dir: %v", err)
	}

	got, err := policy.SkillRegistryFromFS(dir)
	if err != nil {
		t.Fatalf("SkillRegistryFromFS(%s) error: %v", dir, err)
	}
	sort.Strings(got)
	want := []string{"adversarial-testing", "engineering-craft", "fable-mode"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SkillRegistryFromFS(%s) = %v, want %v (dir without SKILL.md excluded)", dir, got, want)
	}
}

// TestSkillRegistryFromFS_EmptyDirYieldsEmptyRegistry: an empty skills dir is
// a valid (if degenerate) registry, not an error — every proposal clamps to
// nothing rather than the loader failing loudly on a legitimate empty state.
func TestSkillRegistryFromFS_EmptyDirYieldsEmptyRegistry(t *testing.T) {
	dir := t.TempDir()
	got, err := policy.SkillRegistryFromFS(dir)
	if err != nil {
		t.Fatalf("SkillRegistryFromFS(empty dir) error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("SkillRegistryFromFS(empty dir) = %v, want empty", got)
	}
}

// TestClampAdvisorSkills_RejectsOutOfRegistry: AC2 clamp matrix — a proposed
// skill name that is not in the filesystem registry is rejected and logged,
// never silently dropped.
func TestClampAdvisorSkills_RejectsOutOfRegistry(t *testing.T) {
	var pol policy.Policy
	registry := []string{"engineering-craft", "fable-mode"}

	accepted, rejections := pol.ClampAdvisorSkills([]string{"engineering-craft", "does-not-exist"}, registry)

	if !reflect.DeepEqual(accepted, []string{"engineering-craft"}) {
		t.Errorf("accepted = %v, want [engineering-craft]", accepted)
	}
	if len(rejections) != 1 || rejections[0].Skill != "does-not-exist" || rejections[0].Reason != "not-in-registry" {
		t.Errorf("rejections = %+v, want one entry {Skill: does-not-exist, Reason: not-in-registry}", rejections)
	}
}

// TestClampAdvisorSkills_RejectsDenylisted: AC2 — a skill on the operator's
// overlays.advisor deny_list is rejected even though it is a valid registry
// member.
func TestClampAdvisorSkills_RejectsDenylisted(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{
		Advisor: &policy.AdvisorOverlayPolicy{DenyList: []string{"fable-mode"}},
	}}
	registry := []string{"engineering-craft", "fable-mode"}

	accepted, rejections := pol.ClampAdvisorSkills([]string{"engineering-craft", "fable-mode"}, registry)

	if !reflect.DeepEqual(accepted, []string{"engineering-craft"}) {
		t.Errorf("accepted = %v, want [engineering-craft] (fable-mode denylisted)", accepted)
	}
	if len(rejections) != 1 || rejections[0].Skill != "fable-mode" || rejections[0].Reason != "denylisted" {
		t.Errorf("rejections = %+v, want one entry {Skill: fable-mode, Reason: denylisted}", rejections)
	}
}

// TestClampAdvisorSkills_TruncatesOverMaxByAdvisorPriorityOrder: AC2 — over
// max_skills_per_dispatch (default 2), the proposal is truncated by the
// advisor's own priority order (proposal order, first N kept) and every
// truncated skill is logged, never a silent drop or a re-sort.
func TestClampAdvisorSkills_TruncatesOverMaxByAdvisorPriorityOrder(t *testing.T) {
	var pol policy.Policy // no explicit block -> default max_skills_per_dispatch=2
	registry := []string{"engineering-craft", "fable-mode", "adversarial-testing"}

	accepted, rejections := pol.ClampAdvisorSkills(
		[]string{"engineering-craft", "fable-mode", "adversarial-testing"}, registry)

	if !reflect.DeepEqual(accepted, []string{"engineering-craft", "fable-mode"}) {
		t.Errorf("accepted = %v, want first 2 in proposal order [engineering-craft fable-mode]", accepted)
	}
	if len(rejections) != 1 || rejections[0].Skill != "adversarial-testing" || rejections[0].Reason != "over-max-skills-per-dispatch" {
		t.Errorf("rejections = %+v, want one entry {Skill: adversarial-testing, Reason: over-max-skills-per-dispatch}", rejections)
	}
}

// TestClampAdvisorSkills_OperatorConfiguredMaxIsHonored: overlays.advisor.max_skills_per_dispatch
// overrides the compiled default of 2.
func TestClampAdvisorSkills_OperatorConfiguredMaxIsHonored(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{
		Advisor: &policy.AdvisorOverlayPolicy{MaxSkillsPerDispatch: 1},
	}}
	registry := []string{"engineering-craft", "fable-mode"}

	accepted, rejections := pol.ClampAdvisorSkills([]string{"engineering-craft", "fable-mode"}, registry)

	if !reflect.DeepEqual(accepted, []string{"engineering-craft"}) {
		t.Errorf("accepted = %v, want [engineering-craft] (operator cap=1)", accepted)
	}
	if len(rejections) != 1 || rejections[0].Reason != "over-max-skills-per-dispatch" {
		t.Errorf("rejections = %+v, want one over-max-skills-per-dispatch entry", rejections)
	}
}

// TestClampAdvisorSkills_PathSeparatorNeverResolves: AC3 prompt-injection
// guard — a proposed name containing a path separator must NEVER resolve,
// even if an identically-suffixed real skill exists in the registry. This is
// a defense-in-depth assertion distinct from the plain registry-miss case:
// the clamp must reject on the LITERAL proposed string, never normalize/join
// it against the skills root before comparing.
func TestClampAdvisorSkills_PathSeparatorNeverResolves(t *testing.T) {
	var pol policy.Policy
	registry := []string{"engineering-craft"}

	for _, malicious := range []string{
		"../../etc/passwd",
		"engineering-craft/../../../etc/passwd",
		"foo/bar",
		"/etc/passwd",
	} {
		accepted, rejections := pol.ClampAdvisorSkills([]string{malicious}, registry)
		if len(accepted) != 0 {
			t.Errorf("ClampAdvisorSkills(%q) accepted = %v, want none (path separator must never resolve)", malicious, accepted)
		}
		if len(rejections) != 1 || rejections[0].Skill != malicious {
			t.Errorf("ClampAdvisorSkills(%q) rejections = %+v, want exactly one rejection naming the literal proposed string", malicious, rejections)
		}
	}
}

// TestResolveOverlaysWithAdvisor_AdditiveNeverReplaces: AC1 — a phase WITH an
// advisor proposal gets the UNION of the static overlay rules and the
// (already-clamped) advisor skills; a phase WITHOUT any proposal is
// byte-identical to plain ResolveOverlays. Advisory adds, never replaces
// policy — mirrors the {cli,tier} soft-overlay contract in
// phases/runner/runner.go:429-447.
func TestResolveOverlaysWithAdvisor_AdditiveNeverReplaces(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{
		Rules: []policy.OverlayRule{{Tiers: []string{"deep"}, Skills: []string{"fable-mode"}}},
	}}
	d := policy.OverlayDispatch{Phase: "build", Tier: "deep"}

	withProposal := pol.ResolveOverlaysWithAdvisor(d, []string{"engineering-craft"})
	if !reflect.DeepEqual(withProposal, []string{"fable-mode", "engineering-craft"}) {
		t.Errorf("ResolveOverlaysWithAdvisor(with proposal) = %v, want static-first union [fable-mode engineering-craft]", withProposal)
	}

	withoutProposal := pol.ResolveOverlaysWithAdvisor(d, nil)
	plain := pol.ResolveOverlays(d)
	if !reflect.DeepEqual(withoutProposal, plain) {
		t.Errorf("ResolveOverlaysWithAdvisor(no proposal) = %v, want byte-identical to ResolveOverlays = %v", withoutProposal, plain)
	}
}

// TestResolveOverlaysWithAdvisor_DedupesOverlapWithStaticRule: if the advisor
// proposes a skill the static rules already contribute, the union must dedup
// (stable, static-first order) rather than double-inject the same skill body.
func TestResolveOverlaysWithAdvisor_DedupesOverlapWithStaticRule(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{
		Rules: []policy.OverlayRule{{Tiers: []string{"deep"}, Skills: []string{"fable-mode"}}},
	}}
	d := policy.OverlayDispatch{Phase: "build", Tier: "deep"}

	got := pol.ResolveOverlaysWithAdvisor(d, []string{"fable-mode", "engineering-craft"})
	if !reflect.DeepEqual(got, []string{"fable-mode", "engineering-craft"}) {
		t.Errorf("ResolveOverlaysWithAdvisor(overlap) = %v, want deduped [fable-mode engineering-craft]", got)
	}
}
