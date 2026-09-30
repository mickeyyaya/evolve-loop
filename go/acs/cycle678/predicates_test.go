//go:build acs

package cycle678

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg           = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	auditPkg          = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	completenessPkg   = "github.com/mickeyyaya/evolve-loop/go/acs/regression/apicover"
	auditGateTestGlob = "github.com/mickeyyaya/evolve-loop/go/acs/regression/..."
)

func runGo(t *testing.T, args ...string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("could not launch go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return code == 0, out
}

func runGoTest(t *testing.T, pkg, pattern string, extraArgs ...string) (ok bool, out string) {
	t.Helper()
	args := append([]string{"test", "-run", pattern, "-count=1"}, extraArgs...)
	return runGo(t, append(args, pkg)...)
}

func TestC678_001_BuildGraduationCheckTable(t *testing.T) {
	if ok, out := runGoTest(t, corePkg, "^TestBuildGraduationCheck$"); !ok {
		t.Errorf("build-entry graduation check contract not green:\n%s", out)
	}
}

func TestC678_002_BuildPhaseGraduationAbortWiring(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"^(TestRecordAndBranch_BuildGraduationGuardAborts|TestRecordAndBranch_BuildGraduationGuardEnrolledProceeds)$"); !ok {
		t.Errorf("build-phase graduation abort wiring not green:\n%s", out)
	}
}

func TestC678_003_AuditGraduationGateRegistered(t *testing.T) {
	if ok, out := runGoTest(t, auditPkg, "^TestNewDefaultWithStageCompact_GraduationGateRegistered$"); !ok {
		t.Errorf("audit-side graduation gate registration regression not green:\n%s", out)
	}
}

func TestC678_004_NoFalsePositiveOnDeleteRename(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"^TestBuildGraduationCheck$/^(deleted-package-not-flagged|renamed-package-reenrolled-passes|enrolled-same-diff-passes)$"); !ok {
		t.Errorf("graduation guard false-positive arms not green:\n%s", out)
	}
}

func TestC678_005_RepoWideCompletenessPredicateInAuditGateSet(t *testing.T) {
	ok, out := runGo(t, "list", "-tags", "acs", auditGateTestGlob)
	if !ok {
		t.Fatalf("go list over the acs-durable gate glob failed:\n%s", out)
	}
	if !strings.Contains(out, completenessPkg) {
		t.Errorf("completeness predicate package %s is NOT inside the acs-durable audit gate glob %s — the audit's repo-wide gate set no longer runs TestApicoverEnforce_CoversEveryInternalPackage.\ngo list output:\n%s",
			completenessPkg, auditGateTestGlob, out)
	}
	if ok, tout := runGoTest(t, completenessPkg,
		"^TestApicoverEnforce_CoversEveryInternalPackage$", "-tags", "acs"); !ok {
		t.Errorf("repo-wide apicover completeness predicate not green at HEAD:\n%s", tout)
	}
}
