//go:build acs

package cycle1510

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "./internal/core"

func runCoreTest(t *testing.T, runPattern string) (string, int) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir, "test", "-count=1", "-run", runPattern, corePkg)
	if code < 0 {
		t.Fatalf("could not launch `go test -run %s %s` in %s: %v", runPattern, corePkg, goDir, err)
	}
	return out + "\n" + errOut, code
}

func workspaceDir(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	candidates := []string{filepath.Join(root, ".evolve", "runs", "cycle-1510")}
	if i := strings.Index(root, string(filepath.Separator)+filepath.Join(".evolve", "worktrees")+string(filepath.Separator)); i >= 0 {
		candidates = append(candidates, filepath.Join(root[:i], ".evolve", "runs", "cycle-1510"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

func TestC1510_001_ArtifactDeterminedClassifierAllowlistsTheSixCodes(t *testing.T) {
	out, code := runCoreTest(t, "^TestContractArtifactDetermined_")
	if code != 0 {
		t.Errorf("contractArtifactDetermined contract FAILED (exit %d).\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") || strings.Contains(out, "[no test files]") {
		t.Errorf("no TestContractArtifactDetermined_* test executed — the classifier's contract is unverified, not satisfied.\n%s", out)
	}
}

func TestC1510_002_ArtifactDeterminedFailsClosedOnStrayAndUnknown(t *testing.T) {
	out, code := runCoreTest(t,
		"^TestContractArtifactDetermined_StrayInWorktreeIsFalse$|^TestContractArtifactDetermined_Table$/^(stray_in_worktree|unknown_code|empty_string|case_variant_is_not_the_code|bracketed_rendering_is_not_a_code)$")
	if code != 0 {
		t.Errorf("fail-closed contract FAILED (exit %d) — an unchanged artifact hash must never be read as \"no new evidence\" for a location-class or unknown violation code.\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no fail-closed subtest executed — the negative axis is unverified.\n%s", out)
	}
}

func TestC1510_003_ComposeCorrectionCarriesReasonVerbatim(t *testing.T) {
	out, code := runCoreTest(t, "^TestComposeCorrection_")
	if code != 0 {
		t.Errorf("composeCorrection verbatim-fidelity lock FAILED (exit %d).\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no TestComposeCorrection_* test executed — the verbatim property is unlocked, which IS this task's deliverable.\n%s", out)
	}
}

func TestC1510_004_ComposeCorrectionProductionCodeUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"git", "-C", root, "diff", "origin/main", "--", "go/internal/core/retry_backoff.go")
	if code < 0 {
		t.Fatalf("could not launch git diff in %s: %v", root, err)
	}
	if code > 1 {
		t.Fatalf("git diff failed (exit %d): %s", code, errOut)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("go/internal/core/retry_backoff.go was MODIFIED, but this task's verbatim criterion is green-from-birth on the pre-change code — a product edit here would be a tautologically-green claim (the exact cycle-1508 M1 defect). Diff:\n%s", out)
	}
}

func TestC1510_005_BuildReportEnumeratesBlockingGateAmendments(t *testing.T) {
	report := filepath.Join(workspaceDir(t), "build-report.md")
	if !acsassert.FileExists(t, report) {
		t.Fatalf("build-report.md absent at %s", report)
	}
	if !acsassert.FileMatchesRegex(t, report, `(?i)^#+\s*Amendments`) {
		t.Errorf("build-report.md has no `## Amendments` section. Every amendment returned by a blocking gate must be enumerated adopted/rejected+reason — and if none were returned, that must be stated explicitly. A silently dropped re-scoping amendment is the cycle-1508 M1 process defect.")
	}
	if !acsassert.FileContainsAny(report, "adopted", "Adopted", "none returned", "No amendments", "no amendments") {
		t.Errorf("build-report.md's amendment accounting names no disposition — expected each amendment marked adopted/rejected with a reason, or an explicit \"no amendments returned\".")
	}
}
