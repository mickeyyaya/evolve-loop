package policy_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

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

func TestClampAdvisorSkills_TruncatesOverMaxByAdvisorPriorityOrder(t *testing.T) {
	var pol policy.Policy
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
