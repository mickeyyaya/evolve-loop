//go:build acs

package cycle1676

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	coherencePkg = "./internal/coherence"
	corePkg      = "./internal/core"

	evalPath = ".evolve/evals/crossartifact-invariant-stack.md"
)

func goModRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func assertBindingsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goModRoot(t), "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

func TestC1676_001_FourInvariantsReportIndependentlyWithEvidence(t *testing.T) {
	assertBindingsPass(t, coherencePkg,
		"TestCrossArtifactInvariants_CoherentWorkspaceIsAllOK",
		"TestCrossArtifactInvariants_VerdictDisagreementIsViolatedWithBothSides",
		"TestCrossArtifactInvariants_TestCountDisagreementNamesClaimedAndCounted",
		"TestCrossArtifactInvariants_TestCountTotalDisagreementIsViolated",
		"TestCrossArtifactInvariants_MissingReferencedPathIsViolatedAndNamed",
		"TestCrossArtifactInvariants_PhaseOrderViolationsAreReported",
	)
}

func TestC1676_002_AbsentOrMalformedArtifactsFailSafeAndCannotBeGreppedGreen(t *testing.T) {
	assertBindingsPass(t, coherencePkg,
		"TestCrossArtifactInvariants_ProseVerdictCannotSatisfyTheSentinelInvariant",
		"TestCrossArtifactInvariants_AbsentArtifactsAreIndeterminateNotOK",
		"TestCrossArtifactInvariants_MalformedArtifactsAreIndeterminateNotOK",
		"TestCrossArtifactInvariants_ViolationsReturnsOnlyTheViolated",
	)
}

func TestC1676_003_AggregateIsDeterministicAndLaneBound(t *testing.T) {
	assertBindingsPass(t, coherencePkg,
		"TestCrossArtifactInvariants_ReportShapeIsDeterministic",
		"TestCrossArtifactInvariants_ReferencedPathsResolveUnderWorkspaceThenWorktree",
	)
	assertBindingsPass(t, corePkg,
		"TestFinalizeCycle_CrossArtifactBindsTheLaneWorktreeNotTheProjectRoot",
	)
}

func TestC1676_004_FindingsAreRecordedAtTheRealSeamAndStayAdvisory(t *testing.T) {
	assertBindingsPass(t, corePkg,
		"TestFinalizeCycle_EmitsAdvisoryCrossArtifactInvariantsArtifact",
		"TestFinalizeCycle_CrossArtifactViolationsNeverBlockTheCycle",
	)
}

func TestC1676_005_ExistingVerdictCoherenceBehaviourIsUnchanged(t *testing.T) {
	assertBindingsPass(t, coherencePkg,
		"TestCoherence_ResultShape",
		"TestReadCycleVerdicts_Fixture",
		"TestCheckVerdictCoherence",
		"TestCheckVerdictCoherence_Reconcile",
		"TestCheckVerdictCoherence_ForgedStillHalts",
		"TestCrossArtifactInvariants_ExistingVerdictCoherenceIsUntouched",
	)
	assertBindingsPass(t, corePkg,
		"TestDetectVerdictIncoherence_ForgedVerdict_Halts",
		"TestDetectVerdictIncoherence_GenuineFail_NoHalt",
		"TestDetectVerdictIncoherence_ReconcileUsesFullVerify",
		"TestFinalizeCycle_VerdictIncoherenceFloorSurvivesTheAdvisory",
	)
}

func TestC1676_006_MaterializedEvalIsDurableAndBehavioral(t *testing.T) {
	root := acsassert.RepoRoot(t)
	abs := filepath.Join(root, evalPath)
	if !acsassert.FileExists(t, abs) {
		t.Fatalf("RED: %s missing — the cycle's permanent regression entry was never materialized", evalPath)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", evalPath); code == 0 {
		t.Errorf("RED: %s is gitignored — it will be dropped at ship and cap nothing (cycle-93)", evalPath)
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", evalPath, err)
	}
	body := string(raw)
	if !strings.HasPrefix(body, "---\nscore_cap:") {
		t.Errorf("RED: %s has no score_cap frontmatter — nothing caps a future audit score", evalPath)
	}
	if n := strings.Count(body, "- criterion:"); n < 4 {
		t.Errorf("RED: %s declares %d score_cap criteria, want one per behavioral acceptance criterion (>=4)", evalPath, n)
	}
	if n := strings.Count(body, "[code]"); n < 4 {
		t.Errorf("RED: %s carries %d [code] checks, want >=4 behavioral ones", evalPath, n)
	}
	for _, needle := range []string{"go test", "internal/coherence", "internal/core"} {
		if !strings.Contains(body, needle) {
			t.Errorf("RED: %s has no evidence command exercising %q — a non-behavioral eval caps nothing", evalPath, needle)
		}
	}
}

func TestC1676_007_EnrolledPackagePublicAPIGateStaysGreen(t *testing.T) {
	mod := goModRoot(t)
	if !acsassert.FileContains(t, filepath.Join(mod, ".apicover-enforce"), coherencePkg) {
		t.Skipf("internal/coherence is not enrolled in .apicover-enforce — nothing for this gate to enforce")
	}

	tmp := t.TempDir()
	profile := filepath.Join(tmp, "coverage.txt")
	if _, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", mod, "test", "-count=1", "-coverprofile="+profile, coherencePkg); code != 0 {
		t.Fatalf("RED: `go test -coverprofile` on %s exited %d: %v\nstderr:\n%s", coherencePkg, code, err, stderr)
	}

	funcProfile := filepath.Join(tmp, "coverage.func.txt")
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", mod, "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover exited %d: %v\nstderr:\n%s", code, err, stderr)
	}
	if werr := os.WriteFile(funcProfile, []byte(out), 0o644); werr != nil {
		t.Fatalf("write func profile: %v", werr)
	}

	pkgDir := filepath.Join(mod, "internal", "coherence")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", mod, "run", "./cmd/apicover", "-enforce", "-cover", funcProfile, pkgDir)
	if code != 0 {
		t.Errorf("RED: the public-API gate reds on %s (exit %d, %v) — an added export is uncovered or named-but-never-executed:\n%s\n%s",
			coherencePkg, code, err, stdout, stderr)
	}
}
