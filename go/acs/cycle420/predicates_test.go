//go:build acs

// Package cycle420 materializes the cycle-420 acceptance criteria for the two
// committed top_n tasks:
//
//   - router-catalog-dedup-overflow (T1) — replace the bare "- also available (<names>)"
//     overflow enumeration in writeCatalog with a one-line pointer to phase-inventory.json,
//     removing ~1KB/cycle of duplicate context from the largest per-cycle prompt.
//
//   - router-persona-tsc-compress (T2) — apply TSC to the prose sections of
//     agents/evolve-router.md (## Your job, ## Output contract, ## Goal-Type Recipes prose),
//     adding the "<!-- TSC applied" marker and achieving ≥15% byte reduction on those
//     sections while keeping the Phase Catalog table byte-identical.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	router-catalog-dedup-overflow (T1):
//	  AC1  writeCatalog overflow → no "- also available (" enumeration   → C420_001 (RED)
//	  AC2  writeCatalog overflow → pointer to phase-inventory.json        → C420_002 (RED)
//	  AC3  negative: even 1-card overflow must have pointer, no enum      → C420_003 (RED)
//	  AC4  edge: no-overflow path unchanged (no pointer, no enum)         → C420_004 (pre-existing GREEN)
//	  AC5  regression: TestRouterCompaction still passes                  → C420_005 (pre-existing GREEN)
//
//	router-persona-tsc-compress (T2):
//	  AC1  TSC marker present in agents/evolve-router.md                  → C420_006 (RED)
//	  AC2  prose region < 5243 bytes (≥15% below 6169-byte baseline)      → C420_007 (RED)
//	  AC3  negative: catalog section byte-identical (7988 bytes)          → C420_008 (pre-existing GREEN)
//	  AC4  edge: domain vocab/code tokens preserved verbatim              → C420_009 (pre-existing GREEN)
//	  AC5  regression: loader + router compaction suite still passes      → C420_010 (pre-existing GREEN)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: C420_003 (pointer required even with 1 overflow — catches remove-only gaming);
//	          C420_008 (catalog unchanged — catches TSC gaming by trimming the protected table).
//	Edge/OOD: C420_004 (no overflow → no pointer/enum — boundary at maxEnrichedCatalogCards);
//	          C420_007 (boundary: 5243 bytes exact — 5243 ≥ 5243 → still fails).
//	Semantic:  10 distinct dimensions across 2 tasks: enum-absent / pointer-present /
//	           anti-gaming / no-overflow-clean / compaction-regression / tsc-marker /
//	           prose-bytes / catalog-bytes / vocab-tokens / parse-green.
//
// 1:1 enforcement:
//
//	T1: predicate=5 (C420_001–C420_005), manual+checklist=0, unverifiable-remove=0 → total AC=5 ✓
//	T2: predicate=5 (C420_006–C420_010), manual+checklist=0, unverifiable-remove=0 → total AC=5 ✓
package cycle420

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the <repoRoot>/go module directory for subprocess test invocations.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// ===================== T1 — router-catalog-dedup-overflow =====================

// TestC420_001_WriteCatalogNoEnumeration asserts that the unit test
// TestWriteCatalog_OverflowNotEnumerated passes — writeCatalog does NOT emit
// "- also available (<names>)" when overflow exists.
// BEHAVIORAL: runs real go test against the core package.
//
// RED: unit test currently fails because current code emits the enumeration.
func TestC420_001_WriteCatalogNoEnumeration(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestWriteCatalog_OverflowNotEnumerated",
		"./internal/core/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestWriteCatalog_OverflowNotEnumerated failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// TestC420_002_WriteCatalogPointerPresent asserts that the unit test
// TestWriteCatalog_PointerLinePresent passes — writeCatalog emits a pointer
// to phase-inventory.json when overflow exists.
// BEHAVIORAL: runs real go test against the core package.
//
// RED: unit test currently fails because current code emits enumeration, not pointer.
func TestC420_002_WriteCatalogPointerPresent(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestWriteCatalog_PointerLinePresent",
		"./internal/core/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestWriteCatalog_PointerLinePresent failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// TestC420_003_WriteCatalogPointerRequired_Negative asserts that the adversarial
// negative test TestWriteCatalog_PointerLineRequired_Negative passes — even with
// 1 overflow card, the enumeration is absent AND the pointer is present.
// BEHAVIORAL: runs real go test against the core package.
//
// RED: unit test currently fails (current code enumerates instead of pointing).
func TestC420_003_WriteCatalogPointerRequired_Negative(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestWriteCatalog_PointerLineRequired_Negative",
		"./internal/core/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestWriteCatalog_PointerLineRequired_Negative failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// TestC420_004_WriteCatalogNoOverflowNoPointer asserts that the edge test
// TestWriteCatalog_NoOverflow_NoPointer passes — when the catalog fits within
// maxEnrichedCatalogCards, no pointer or enumeration is emitted.
// BEHAVIORAL: runs real go test against the core package.
//
// Pre-existing GREEN: no-overflow path unchanged before and after the fix.
// Regression guard: fix must not unconditionally add a pointer to every call.
func TestC420_004_WriteCatalogNoOverflowNoPointer(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestWriteCatalog_NoOverflow_NoPointer",
		"./internal/core/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestWriteCatalog_NoOverflow_NoPointer failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// TestC420_005_RouterCompactionRegression asserts that the existing
// TestRouterCompaction tests still pass after the writeCatalog change.
// BEHAVIORAL: runs real go test against the prompts package.
//
// Pre-existing GREEN: router compaction guards already pass.
// Regression guard: change to phase_advisor.go must not break prompt loading.
func TestC420_005_RouterCompactionRegression(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterCompaction",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestRouterCompaction failed after writeCatalog change (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// ===================== T2 — router-persona-tsc-compress =====================

// TestC420_006_RouterPersonaTSCMarker asserts that the unit test
// TestRouterPersona_TSCMarkerPresent passes — agents/evolve-router.md carries
// the "<!-- TSC applied" marker matching scout/builder/auditor.
// BEHAVIORAL: runs real go test against the prompts package.
//
// RED: evolve-router.md has no TSC marker before Builder runs.
func TestC420_006_RouterPersonaTSCMarker(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterPersona_TSCMarkerPresent",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestRouterPersona_TSCMarkerPresent failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// TestC420_007_RouterPersonaProseReduction asserts that the unit test
// TestRouterPersona_ProseRegionByteReduction passes — prose region is <5243 bytes
// (≥15% below the 6169-byte baseline).
// BEHAVIORAL: runs real go test against the prompts package.
//
// RED: prose region is 6169 bytes before Builder applies TSC; 6169 ≥ 5243 → fails.
func TestC420_007_RouterPersonaProseReduction(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterPersona_ProseRegionByteReduction",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestRouterPersona_ProseRegionByteReduction failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// TestC420_008_RouterPersonaCatalogIdentical_Negative asserts that the negative
// test TestRouterPersona_CatalogByteIdentical_Negative passes — the catalog
// section is exactly 7988 bytes (unchanged from the baseline).
// BEHAVIORAL: runs real go test against the prompts package.
//
// Pre-existing GREEN: catalog is unchanged before Builder runs.
// Anti-gaming sentinel: catches a builder who trims the catalog to reduce prose bytes.
func TestC420_008_RouterPersonaCatalogIdentical_Negative(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterPersona_CatalogByteIdentical_Negative",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestRouterPersona_CatalogByteIdentical_Negative failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// TestC420_009_RouterPersonaDomainVocabPreserved asserts that the edge test
// TestRouterPersona_DomainVocabPreserved passes — key domain vocabulary tokens
// (routing-plan.json, fast|balanced|deep, writes_source, ClampPlanToFloor) are
// present verbatim after TSC.
// BEHAVIORAL: runs real go test against the prompts package.
//
// Pre-existing GREEN: all vocab tokens present before Builder runs.
// Regression guard: TSC must not paraphrase or abbreviate domain vocabulary.
func TestC420_009_RouterPersonaDomainVocabPreserved(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterPersona_DomainVocabPreserved",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestRouterPersona_DomainVocabPreserved failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// TestC420_010_RouterPersonaLoaderAndRenderGreen asserts that the regression test
// TestRouterPersona_LoaderAndRenderParseGreen passes — ParseFrontmatter still
// parses evolve-router.md correctly and TestRouterCompaction still passes.
// BEHAVIORAL: runs real go test against the prompts package (which calls SubprocessOutput
// internally to run TestRouterCompaction).
//
// Pre-existing GREEN: file parses cleanly before Builder runs.
// Regression guard: TSC must not corrupt YAML frontmatter or break loader.
func TestC420_010_RouterPersonaLoaderAndRenderGreen(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterPersona_LoaderAndRenderParseGreen",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestRouterPersona_LoaderAndRenderParseGreen failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}
