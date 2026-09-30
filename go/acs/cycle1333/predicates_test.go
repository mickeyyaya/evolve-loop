//go:build acs

package cycle1333

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runtimeReferencePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
}

func changelogPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "CHANGELOG.md")
}

// acs-predicate: config-check — documentation accuracy criterion (AC1, part
func TestC1333_001_RuntimeReferenceNamesFingerprintFlagAndResetGating(t *testing.T) {
	path := runtimeReferencePath(t)
	acsassert.AllOf(t,
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, path, "--fingerprint") },
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, path, "--reset") },
		func(tb acsassert.TB) bool {
			return acsassert.FileContains(tb, path, "resolved-fingerprints.json")
		},
	)
}

// acs-predicate: config-check — documentation accuracy criterion (AC1, part
func TestC1333_002_RuntimeReferenceDescribesLedgerRecordShape(t *testing.T) {
	path := runtimeReferencePath(t)
	acsassert.AllOf(t,
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, path, "fingerprint") },
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, path, "resolved_at") },
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, path, "resolved_by") },
	)
}

// acs-predicate: config-check — documentation accuracy criterion (AC1, part
func TestC1333_003_RuntimeReferenceNamesRuleBExclusion(t *testing.T) {
	path := runtimeReferencePath(t)
	if !acsassert.FileContainsAny(path, "Rule B", "identical-fingerprint") {
		t.Errorf("runtime-reference.md must name the excluded rule (%q): expected %q or %q",
			path, "Rule B", "identical-fingerprint")
	}
}

// acs-predicate: config-check — documentation accuracy criterion (AC2); see
func TestC1333_004_ChangelogMentionsResetFingerprintFlag(t *testing.T) {
	path := changelogPath(t)
	if !acsassert.FileContains(t, path, "--reset --fingerprint") {
		t.Errorf("CHANGELOG.md must contain the literal flag pair %q (%q)",
			"--reset --fingerprint", path)
	}
}

func TestC1333_005_CoreBlockerBreakerFingerprintTestsStillPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", filepath.Join(root, "go"), "-v",
		"-run", "TestLoadResolvedFingerprints_ReadsLedgerRecords|TestLoadResolvedFingerprints_MissingFileReturnsEmptyNoError|TestEvaluateBlockerBreaker_ExcludesAckedFingerprint|TestEvaluateBlockerBreaker_UnackedIdenticalFingerprintStillHalts|TestAppendResolvedFingerprint_WritesRecord",
		"./internal/core",
	)
	if err != nil || code != 0 {
		t.Fatalf("go test ./internal/core (fingerprint subset) failed: code=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS:") {
		t.Errorf("expected at least one \"--- PASS:\" marker in go test output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "--- FAIL:") {
		t.Errorf("unexpected \"--- FAIL:\" marker in go test output:\n%s", stdout)
	}
}

func TestC1333_006_RunLoopFingerprintAckCallerProofStillPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", filepath.Join(root, "go"), "-v",
		"-run", "TestRunLoop_FingerprintAck_AppendsLedgerRecord",
		"./cmd/evolve",
	)
	if err != nil || code != 0 {
		t.Fatalf("go test ./cmd/evolve -run TestRunLoop_FingerprintAck_AppendsLedgerRecord failed: code=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: TestRunLoop_FingerprintAck_AppendsLedgerRecord") {
		t.Errorf("expected \"--- PASS: TestRunLoop_FingerprintAck_AppendsLedgerRecord\" in go test output, got:\n%s", stdout)
	}
}
