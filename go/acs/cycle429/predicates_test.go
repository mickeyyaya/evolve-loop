//go:build acs

package cycle429

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runPanestreamTest(t *testing.T, runFilter string) (string, string, int) {
	t.Helper()
	const pkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runFilter, pkg,
	)
	return stdout, stderr, code
}

func runBridgeTest(t *testing.T, runFilter string) (string, string, int) {
	t.Helper()
	const pkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runFilter, pkg,
	)
	return stdout, stderr, code
}

func TestC429_001_ExtractResponseTokens_KForm(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestExtractResponseTokens/k-form")
	if code != 0 {
		t.Errorf("C429_001: ExtractResponseTokens k-form test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC429_002_ExtractResponseTokens_PlainInteger(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestExtractResponseTokens/plain-integer")
	if code != 0 {
		t.Errorf("C429_002: ExtractResponseTokens plain-integer test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC429_003_ExtractResponseTokens_PeakAcrossMatches(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestExtractResponseTokens/multiple")
	if code != 0 {
		t.Errorf("C429_003: ExtractResponseTokens peak test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC429_004_ExtractResponseTokens_MalformedAndEmpty(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestExtractResponseTokens/(empty|malformed|no_counter|no_arrow)")
	if code != 0 {
		t.Errorf("C429_004: ExtractResponseTokens malformed/empty test exit=%d\nstderr=%s", code, stderr)
	}
}

// acs-predicate: config-check
func TestC429_005_ExtractTokenCountLivenessAbsent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	livenessPath := filepath.Join(root, "go", "internal", "bridge", "panestream", "liveness.go")
	ok1 := acsassert.FileNotContains(t, livenessPath, "extractTokenCountLiveness")
	ok2 := acsassert.FileNotContains(t, livenessPath, "rxLivenessTokens")
	if !ok1 || !ok2 {
		t.Errorf("C429_005: old private extractor symbols must be deleted from liveness.go (still present)")
	}
}

func TestC429_006_ClaudeDetectorRegressionStillFires(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestClaudeDetector_IncreasingTokensConverging")
	if code != 0 {
		t.Errorf("C429_006: ClaudeDetector regression test exit=%d\nstderr=%s", code, stderr)
	}
}

// acs-predicate: config-check
func TestC429_007_ExtractTokenCountAbsent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stopReviewPath := filepath.Join(root, "go", "internal", "bridge", "stopreview.go")
	ok1 := acsassert.FileNotContains(t, stopReviewPath, "func extractTokenCount")
	ok2 := acsassert.FileNotContains(t, stopReviewPath, "rxTokens")
	if !ok1 || !ok2 {
		t.Errorf("C429_007: old private extractor symbols must be deleted from stopreview.go (still present)")
	}
}

func TestC429_008_ReconciledPlainIntegerYields5200(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestExtractTokenCount/unified")
	if code != 0 {
		t.Errorf("C429_008: reconciled plain-integer test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC429_009_TokenUsageSidecarPreserved(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestTmuxPhase_WritesTokenUsage")
	if code != 0 {
		t.Errorf("C429_009: token-usage sidecar test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC429_010_BuildReportTokenUsagePopulated(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestBuildReport_TokenUsage")
	if code != 0 {
		t.Errorf("C429_010: BuildReport token-usage test exit=%d\nstderr=%s", code, stderr)
	}
}
