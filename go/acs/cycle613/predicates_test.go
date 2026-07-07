//go:build acs

// Package cycle613 materialises the cycle-613 acceptance criteria for the
// single triage-committed top_n task, advisor-skill-selection (weight 0.92,
// .evolve/inbox/2026-07-07T18-30-00Z-advisor-skill-selection.json): the
// advisor gains authority to PROPOSE per-phase skill sets — following the
// exact {cli,tier} soft-overlay precedent (runner.go:429-447,
// router.ClampPlanModelRouting) — clamped by the kernel against the
// filesystem skills registry, never trusted as free text.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…574
// precedent) — each predicate shells `go test -run` over the RED unit tests
// authored this cycle in internal/policy (advisor_skill_overlay_test.go).
// None is a source-grep; every one exercises the system under test
// (Policy.ClampAdvisorSkills, Policy.ResolveOverlaysWithAdvisor,
// policy.SkillRegistryFromFS over a real temp-dir filesystem walk) and
// asserts on its result. RED now: internal/policy does not compile
// (AdvisorOverlayPolicy / AdvisorSkillRejection / ClampAdvisorSkills /
// ResolveOverlaysWithAdvisor / SkillRegistryFromFS all undefined). GREEN
// once Builder implements the clamp/merge/registry contract.
//
// Scope: the dispatch wiring into Engine.Launch / PhaseRequest / the advisor
// prompt's registry section, the advisor-rejections.json artifact plumbing,
// and the StaticPrefix cache-contract round-trip are dispositioned
// manual+checklist in test-report.md, not predicated here — see that file's
// Coverage Map for the rationale and the Auditor checklist.
package cycle613

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the
// test cache so the predicate always exercises current source. code<0 is a
// genuine launch failure (binary missing / killed by signal), never a test
// verdict — that must fail loudly, not be misread as RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC613_001_SkillRegistryFromFilesystem — AC5: the advisor's skill
// registry is read from the filesystem (skills/<name>/SKILL.md), never a
// hand-maintained list, and excludes directories with no SKILL.md.
func TestC613_001_SkillRegistryFromFilesystem(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestSkillRegistryFromFS_EnumeratesSkillDirs|TestSkillRegistryFromFS_EmptyDirYieldsEmptyRegistry")
	if !ok {
		t.Errorf("filesystem-sourced skill registry missing or broken (SkillRegistryFromFS undefined or wrong):\n%s", out)
	}
}

// TestC613_002_ClampMatrixRegistryAndDenylist — AC2 (part 1): an
// out-of-registry proposal and a denylisted proposal are each rejected and
// logged, never silently dropped or silently allowed.
func TestC613_002_ClampMatrixRegistryAndDenylist(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestClampAdvisorSkills_RejectsOutOfRegistry|TestClampAdvisorSkills_RejectsDenylisted")
	if !ok {
		t.Errorf("advisor skill clamp does not reject out-of-registry/denylisted proposals (ClampAdvisorSkills missing or wrong):\n%s", out)
	}
}

// TestC613_003_ClampMatrixMaxSkillsPerDispatch — AC2 (part 2): proposals
// beyond max_skills_per_dispatch (compiled default 2, operator-overridable)
// truncate by advisor-priority order and log every truncated skill.
func TestC613_003_ClampMatrixMaxSkillsPerDispatch(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestClampAdvisorSkills_TruncatesOverMaxByAdvisorPriorityOrder|TestClampAdvisorSkills_OperatorConfiguredMaxIsHonored")
	if !ok {
		t.Errorf("advisor skill clamp does not enforce max_skills_per_dispatch (default 2, operator override):\n%s", out)
	}
}

// TestC613_004_PromptInjectionGuard — AC3: a proposed skill name containing
// a path separator (or any string not matching a registry entry verbatim)
// resolves to NOTHING — never a file read outside the skills registry.
func TestC613_004_PromptInjectionGuard(t *testing.T) {
	ok, out := runGoTest(t, policyPkg, "TestClampAdvisorSkills_PathSeparatorNeverResolves")
	if !ok {
		t.Errorf("advisor skill clamp does not guard against path-separator/traversal proposals:\n%s", out)
	}
}

// TestC613_005_AdditiveMergeNeverReplacesPolicy — AC1: a phase with an
// advisor skill proposal gets the union of static overlay rules + the
// clamped advisor skills (deduped); a phase with no proposal is
// byte-identical to the plain static resolver — advisory adds, never
// replaces policy.
func TestC613_005_AdditiveMergeNeverReplacesPolicy(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestResolveOverlaysWithAdvisor_AdditiveNeverReplaces|TestResolveOverlaysWithAdvisor_DedupesOverlapWithStaticRule")
	if !ok {
		t.Errorf("advisor overlay merge is not additive-only (ResolveOverlaysWithAdvisor missing or replaces static rules):\n%s", out)
	}
}
