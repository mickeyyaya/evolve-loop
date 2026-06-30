//go:build acs

// Package cycle429 materialises the cycle-429 acceptance criteria for two tasks
// comprising slice S1 of the signal-center consolidation campaign.
//
// Goal: Consolidate bridge liveness into ONE unified, concurrency-safe SignalCenter.
// S1 (this cycle): unify the two divergent peak-token extractors into a single
// exported panestream.ExtractResponseTokens, and migrate the stopreview callsite.
//
// Tasks:
//
//	s1-unify-token-extractor (T1 — Small, P0):
//	  Create panestream.ExtractResponseTokens (general: k-form + plain-integer);
//	  delete extractTokenCountLiveness + rxLivenessTokens; migrate ClaudeDetector.Assess.
//
//	s1-migrate-stopreview-callsite (T2 — Small, P1, dependsOn T1):
//	  Route driver_tmux_repl.go + stopreview.go through panestream.ExtractResponseTokens;
//	  delete rxTokens + extractTokenCount; reconcile tokencount_test "↓ 5200 tokens" 0→5200.
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	s1-unify-token-extractor (T1):
//	  AC1  ExtractResponseTokens exported + k-form correct          (positive)  → C429_001 RED (compile fail)
//	  AC2  plain-integer works (superset vs old k-only)             (positive)  → C429_002 RED (compile fail)
//	  AC3  peak-across-matches returns max                          (positive)  → C429_003 RED (compile fail)
//	  AC4  malformed/empty → 0                                     (negative)  → C429_004 RED (compile fail)
//	  AC5  extractTokenCountLiveness absent from panestream         (negative)  → C429_005 RED (symbol present)
//	  AC6  ClaudeDetector token layer still fires (regression)      (positive)  → C429_006 RED (compile fail)
//
//	s1-migrate-stopreview-callsite (T2):
//	  AC7  extractTokenCount absent from bridge pkg                 (negative)  → C429_007 RED (symbol present)
//	  AC8  reconciled case: ↓ 5200 tokens → 5200                   (positive)  → C429_008 RED (compile fail)
//	  AC9  token-usage.json peak invariant preserved                (positive)  → C429_009 RED (compile fail)
//	  AC10 BuildReport.TokenUsage populated from sidecar            (positive)  → C429_010 RED (compile fail)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C429_004 (malformed → 0; no-op returning constant can't pass k-form cases),
//	            C429_005 (old private symbol MUST be absent — fails if still present),
//	            C429_007 (extractTokenCount MUST be absent from bridge — fails if still present)
//	Edge/OOD:   C429_004 (empty pane → 0; missing-arrow → 0; no-digits → 0),
//	            C429_002 (plain-integer 50/200 — OOD for old k-only extractor)
//	Semantic:   C429_001 vs C429_002: k-form and plain-integer are distinct parse paths;
//	            C429_003: peak-max is a reduce behavior, not a single-match behavior
//
// 1:1 enforcement:
//
//	T1: predicate=6 (C429_001–006) → total=6 ✓
//	T2: predicate=4 (C429_007–010) → total=4 ✓
//
// RED strategy:
//
//	C429_001–C429_004, C429_006, C429_008–C429_010:
//	  Shell out to `go test ./internal/bridge/panestream/...` or `./internal/bridge/...`.
//	  liveness_test.go calls undefined ExtractResponseTokens → compile error → exit non-zero → RED.
//	C429_005: FileNotContains — extractTokenCountLiveness IS present in liveness.go → assertion fails → RED.
//	C429_007: FileNotContains — extractTokenCount IS present in stopreview.go → assertion fails → RED.
package cycle429

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ── helpers ──────────────────────────────────────────────────────────────────

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

// ── T1: s1-unify-token-extractor ─────────────────────────────────────────────

// TestC429_001_ExtractResponseTokens_KForm (positive, RED):
// panestream.ExtractResponseTokens must exist and correctly parse k-form inputs
// (e.g. "↓ 5.2k tokens" → 5200, "↓ 12k tokens" → 12000). Named so apicover
// -enforce can confirm the exported symbol is covered by a test.
func TestC429_001_ExtractResponseTokens_KForm(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestExtractResponseTokens/k-form")
	if code != 0 {
		t.Errorf("C429_001: ExtractResponseTokens k-form test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC429_002_ExtractResponseTokens_PlainInteger (positive, edge-OOD, RED):
// ExtractResponseTokens must parse plain-integer inputs (e.g. "↓ 200 tokens" → 200)
// — the superset behavior that the old k-only extractor rejected. This is the
// OOD axis: an input class the old function could not handle, now required.
func TestC429_002_ExtractResponseTokens_PlainInteger(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestExtractResponseTokens/plain-integer")
	if code != 0 {
		t.Errorf("C429_002: ExtractResponseTokens plain-integer test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC429_003_ExtractResponseTokens_PeakAcrossMatches (positive, semantic, RED):
// ExtractResponseTokens must return the MAX across all token-counter occurrences
// in the pane, not the first or last. This is a distinct reduce behavior from
// single-match extraction.
func TestC429_003_ExtractResponseTokens_PeakAcrossMatches(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestExtractResponseTokens/multiple")
	if code != 0 {
		t.Errorf("C429_003: ExtractResponseTokens peak test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC429_004_ExtractResponseTokens_MalformedAndEmpty (negative, RED):
// Malformed and empty inputs must return 0. This is the load-bearing anti-no-op
// signal: an implementation returning a constant (e.g. always 5200) fails the
// k-form tests, while one that falls back to 0 for missing markers is also
// verified here (preventing a "return 0" no-op from passing malformed but failing
// k-form). Both axes together rule out the degenerate implementations.
func TestC429_004_ExtractResponseTokens_MalformedAndEmpty(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestExtractResponseTokens/(empty|malformed|no_counter|no_arrow)")
	if code != 0 {
		t.Errorf("C429_004: ExtractResponseTokens malformed/empty test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC429_005_ExtractTokenCountLivenessAbsent (negative, config-check):
// The private extractTokenCountLiveness function and rxLivenessTokens regex
// MUST be absent from panestream/liveness.go after T1 lands. Their presence
// means Builder duplicated instead of unified (the ADR-0047 violation).
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

// TestC429_006_ClaudeDetectorRegressionStillFires (positive, regression, RED):
// ClaudeDetector's token layer must still work after the rename: increasing
// ↓ token counters must produce (LivenessConverging, conf > DefaultDetector).
// Runs the pre-existing TestClaudeDetector_IncreasingTokensConverging test — RED
// only because liveness_test.go now references the non-existent ExtractResponseTokens,
// causing a panestream compile failure until T1 is done.
func TestC429_006_ClaudeDetectorRegressionStillFires(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestClaudeDetector_IncreasingTokensConverging")
	if code != 0 {
		t.Errorf("C429_006: ClaudeDetector regression test exit=%d\nstderr=%s", code, stderr)
	}
}

// ── T2: s1-migrate-stopreview-callsite ───────────────────────────────────────

// TestC429_007_ExtractTokenCountAbsent (negative, config-check):
// The private extractTokenCount function and rxTokens regex MUST be absent from
// the bridge package (stopreview.go) after T2 lands. Their presence means the
// callsite migration is incomplete — the duplicate still lives in bridge.
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

// TestC429_008_ReconciledPlainIntegerYields5200 (positive, semantic, RED):
// The reconciled tokencount_test case "↓ 5200 tokens" must yield 5200, not 0.
// The old k-only extractor returned 0 for plain-integer inputs; the unified
// ExtractResponseTokens superset must return the correct value. Runs the updated
// TestExtractTokenCount suite which now calls panestream.ExtractResponseTokens.
func TestC429_008_ReconciledPlainIntegerYields5200(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestExtractTokenCount/unified")
	if code != 0 {
		t.Errorf("C429_008: reconciled plain-integer test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC429_009_TokenUsageSidecarPreserved (positive, RED):
// The token-usage.json sidecar must still be written with the correct peak after
// the callsite migration. Runs TestTmuxPhase_WritesTokenUsage which drives the
// real REPL engine end-to-end (not a string check on source).
func TestC429_009_TokenUsageSidecarPreserved(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestTmuxPhase_WritesTokenUsage")
	if code != 0 {
		t.Errorf("C429_009: token-usage sidecar test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC429_010_BuildReportTokenUsagePopulated (positive, RED):
// BuildReport.TokenUsage must be populated from the sidecar after migration.
// Runs TestBuildReport_TokenUsage which covers valid/missing/malformed sidecar
// scenarios — end-to-end behavioral, not a source-file check.
func TestC429_010_BuildReportTokenUsagePopulated(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestBuildReport_TokenUsage")
	if code != 0 {
		t.Errorf("C429_010: BuildReport token-usage test exit=%d\nstderr=%s", code, stderr)
	}
}
