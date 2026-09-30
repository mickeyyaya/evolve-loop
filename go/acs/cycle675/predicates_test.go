//go:build acs

package cycle675

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	auditPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("could not launch go test %s: %v\n%s", pkg, err, out)
	}
	return code == 0, out
}

func TestC675_001_BuildGraduationCheckTable(t *testing.T) {
	if ok, out := runGoTest(t, corePkg, "^TestBuildGraduationCheck$"); !ok {
		t.Errorf("build-entry graduation check contract not green:\n%s", out)
	}
}

func TestC675_002_BuildPhaseGraduationAbortWiring(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"^(TestRecordAndBranch_BuildGraduationGuardAborts|TestRecordAndBranch_BuildGraduationGuardEnrolledProceeds)$"); !ok {
		t.Errorf("build-phase graduation abort wiring not green:\n%s", out)
	}
}

func TestC675_003_AuditGraduationGateRegistered(t *testing.T) {
	if ok, out := runGoTest(t, auditPkg, "^TestNewDefaultWithStageCompact_GraduationGateRegistered$"); !ok {
		t.Errorf("audit-side graduation gate registration regression not green:\n%s", out)
	}
}

func TestC675_004_NoFalsePositiveOnDeleteRename(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"^TestBuildGraduationCheck$/^(deleted-package-not-flagged|renamed-package-reenrolled-passes|enrolled-same-diff-passes)$"); !ok {
		t.Errorf("graduation guard false-positive arms not green:\n%s", out)
	}
}
