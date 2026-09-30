//go:build acs

package cycle613

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC613_001_SkillRegistryFromFilesystem(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestSkillRegistryFromFS_EnumeratesSkillDirs|TestSkillRegistryFromFS_EmptyDirYieldsEmptyRegistry")
	if !ok {
		t.Errorf("filesystem-sourced skill registry missing or broken (SkillRegistryFromFS undefined or wrong):\n%s", out)
	}
}

func TestC613_002_ClampMatrixRegistryAndDenylist(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestClampAdvisorSkills_RejectsOutOfRegistry|TestClampAdvisorSkills_RejectsDenylisted")
	if !ok {
		t.Errorf("advisor skill clamp does not reject out-of-registry/denylisted proposals (ClampAdvisorSkills missing or wrong):\n%s", out)
	}
}

func TestC613_003_ClampMatrixMaxSkillsPerDispatch(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestClampAdvisorSkills_TruncatesOverMaxByAdvisorPriorityOrder|TestClampAdvisorSkills_OperatorConfiguredMaxIsHonored")
	if !ok {
		t.Errorf("advisor skill clamp does not enforce max_skills_per_dispatch (default 2, operator override):\n%s", out)
	}
}

func TestC613_004_PromptInjectionGuard(t *testing.T) {
	ok, out := runGoTest(t, policyPkg, "TestClampAdvisorSkills_PathSeparatorNeverResolves")
	if !ok {
		t.Errorf("advisor skill clamp does not guard against path-separator/traversal proposals:\n%s", out)
	}
}

func TestC613_005_AdditiveMergeNeverReplacesPolicy(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestResolveOverlaysWithAdvisor_AdditiveNeverReplaces|TestResolveOverlaysWithAdvisor_DedupesOverlapWithStaticRule")
	if !ok {
		t.Errorf("advisor overlay merge is not additive-only (ResolveOverlaysWithAdvisor missing or replaces static rules):\n%s", out)
	}
}
