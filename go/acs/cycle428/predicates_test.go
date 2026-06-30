//go:build acs

// Package cycle428 materialises the cycle-428 acceptance criteria for task
// `unify-token-extractor` (S1 of the SignalCenter campaign).
//
// Goal: collapse rxLivenessTokens/extractTokenCountLiveness (panestream/liveness.go)
// and rxTokens/extractTokenCount (stopreview.go) into ONE exported
// panestream.ExtractTokenCount with a single k-optional regex (superset).
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n task only):
//
//	AC1 unified parser: k-scale, plain-int, peak, empty, malformed     (positive+edge+negative) → C428_001 RED
//	AC2 duplicate symbols absent (single definition, ADR-0047)         (structural)             → C428_002 RED
//	AC3 plain-integer form counts via bridge consumer path              (cross-package positive) → C428_003 RED
//	AC4 full bridge + panestream suite green under -race, no regression (regression)             → C428_004 RED
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  TestExtractTokenCount_NegativeMalformed in token_test.go (malformed → 0); C428_002 absence-grep
//	Edge/OOD:  plain sub-1k ("↓ 847 tokens"), empty pane, unit count ("↓ 1 tokens")
//	Semantic:  peak-is-max invariant; mixed plain+k peak; cross-package consumer path (AC3)
//
// 1:1 enforcement:
//
//	predicate=4 (C428_001–004) → total=4 ✓ (zero manual, zero unverifiable)
//
// RED strategy:
//
//	C428_001 and C428_003: invoke `go test ./internal/bridge/panestream/...`
//	and `go test ./internal/bridge/...` via subprocess. Both test files reference
//	panestream.ExtractTokenCount which does not exist → compile error → exit non-zero
//	→ predicate fails (RED). After Builder introduces ExtractTokenCount, both
//	suites compile and pass → GREEN.
//
//	C428_002: grep for the four duplicate symbol names in go/internal/bridge/.
//	Currently they exist → grep exits 0 with output → predicate fails (RED).
//	After Builder removes them → grep exits 1 (no matches) → GREEN.
//
//	C428_004: run the full -race suite across bridge + panestream. Currently
//	compile-fails (ExtractTokenCount absent, tokencount_test.go updated contract
//	pending) → exit non-zero → RED. After Builder → all tests pass → GREEN.
package cycle428

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	panestreamPkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	bridgePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// TestC428_001_UnifiedParserBehavior (positive + edge + negative, RED):
// panestream.ExtractTokenCount handles k-scale, plain-integer, peak-tracking,
// empty pane, and malformed inputs. RED because ExtractTokenCount does not
// exist yet → subprocess compile error.
func TestC428_001_UnifiedParserBehavior(t *testing.T) {
	_, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1",
		"-run", "TestExtractTokenCount_Unified|TestExtractTokenCount_NegativeMalformed|TestExtractTokenCount_PeakIsMonotonic",
		panestreamPkg,
	)
	if code != 0 {
		t.Errorf("C428_001: unified parser behavioral tests exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC428_002_DuplicateSymbolsAbsent (structural, RED):
// All four duplicate token-extractor symbols must be absent after Builder
// collapses them into panestream.ExtractTokenCount (ADR-0047 single-source).
// RED because the symbols currently exist in the source.
func TestC428_002_DuplicateSymbolsAbsent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bridgeDir := filepath.Join(root, "go", "internal", "bridge")
	// Each pattern targets one of the four deprecated symbols.
	// grep exits 0 = found (RED), 1 = absent (GREEN).
	duplicates := []string{
		"rxLivenessTokens",
		"extractTokenCountLiveness",
		"var rxTokens",
		"func extractTokenCount(",
	}
	failed := false
	for _, sym := range duplicates {
		stdout, _, code, _ := acsassert.SubprocessOutput(
			"grep", "-rn", "--include=*.go", sym, bridgeDir,
		)
		if code == 0 {
			t.Errorf("C428_002: duplicate symbol %q still present:\n%s", sym, stdout)
			failed = true
		}
	}
	if !failed {
		// All four absent — confirm single definition exists.
		stdout, _, code, _ := acsassert.SubprocessOutput(
			"grep", "-rn", "--include=*.go", "ExtractTokenCount", bridgeDir,
		)
		if code != 0 || !strings.Contains(stdout, "ExtractTokenCount") {
			t.Errorf("C428_002: panestream.ExtractTokenCount not found after duplicate removal:\n%s", stdout)
		}
	}
}

// TestC428_003_PlainIntegerViaConsumerPath (cross-package positive, RED):
// The bridge package consumes panestream.ExtractTokenCount and accepts plain
// "↓ N tokens" (no k suffix) — the under-counting bug this task closes.
// RED because ExtractTokenCount does not exist yet → compile error.
func TestC428_003_PlainIntegerViaConsumerPath(t *testing.T) {
	_, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1",
		"-run", "TestUnifiedTokenExtractor",
		bridgePkg,
	)
	if code != 0 {
		t.Errorf("C428_003: cross-package consumer-path tests exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC428_004_FullSuiteRaceNoRegression (regression, RED):
// The complete bridge + panestream test suite must pass under -race with no
// regression. RED because the token_test.go compile-fails until ExtractTokenCount
// exists and the strict "↓ 5200 tokens"→0 case is updated to the unified contract.
func TestC428_004_FullSuiteRaceNoRegression(t *testing.T) {
	_, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1",
		fmt.Sprintf("%s/...", bridgePkg),
	)
	if code != 0 {
		t.Errorf("C428_004: full bridge+panestream -race suite exit=%d\nstderr=%s", code, stderr)
	}
}
