package core

// cycle420_amplified_test.go — Adversarial amplification for cycle-420 task T1.
//
// Probes gaps NOT covered by phase_advisor_catalog_test.go (AC1–AC4):
//
//   - Single-line pointer: overflow pointer must be exactly one line containing
//     "phase-inventory.json", not multiple lines.
//   - O(1) pointer size: a large overflow (50 extra cards) must not produce a longer
//     output than a minimal overflow (1 extra card) — pointer is a fixed string.
//   - One-below-cap boundary: maxEnrichedCatalogCards-1 optional cards must not
//     trigger the pointer (tighter negative guard than the at-cap AC4 test).

import (
	"strings"
	"testing"
)

// TestWriteCatalog_OverflowPointerIsSingleLine asserts that when the catalog overflows
// the enriched cap, the pointer referencing phase-inventory.json occupies exactly
// one output line (no embedded newlines, no multi-line enumeration).
//
// Amplification angle: AC2 verifies the pointer IS present; this test verifies it is
// a single line, upholding the "one-line pointer" contract that gives the token-savings
// benefit. A two-line pointer would be a partial regression back toward verbose output.
func TestWriteCatalog_OverflowPointerIsSingleLine(t *testing.T) {
	t.Parallel()
	cards := makeOverflowCards(maxEnrichedCatalogCards + 5)
	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()

	var pointerLines []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "phase-inventory.json") {
			pointerLines = append(pointerLines, line)
		}
	}
	if len(pointerLines) == 0 {
		t.Fatal("pointer to phase-inventory.json absent from overflow output (AC2 prerequisite)")
	}
	if len(pointerLines) > 1 {
		t.Errorf("overflow pointer must be exactly one line; found %d lines containing 'phase-inventory.json':\n%v\n"+
			"Multi-line pointer undermines the token-savings goal of router-catalog-dedup-overflow.",
			len(pointerLines), pointerLines)
	}
}

// TestWriteCatalog_LargeOverflow_PointerConstantLength asserts that a large overflow
// (maxEnrichedCatalogCards+50 cards) produces output no longer than a minimal overflow
// (maxEnrichedCatalogCards+1 cards). The pointer must be O(1) text — a fixed one-line
// string regardless of how many cards overflow the cap.
//
// Amplification angle: AC3 tests 1 overflow card; ACS tests 5 overflow cards. Neither
// tests the large-overflow (50 cards) case. If the implementation accidentally formats
// "N phases omitted" or enumerates names in a new format, output would scale with count.
func TestWriteCatalog_LargeOverflow_PointerConstantLength(t *testing.T) {
	t.Parallel()

	minCards := makeOverflowCards(maxEnrichedCatalogCards + 1)
	var minB strings.Builder
	writeCatalog(&minB, minCards)
	minOut := minB.String()

	largeCards := makeOverflowCards(maxEnrichedCatalogCards + 50)
	var largeB strings.Builder
	writeCatalog(&largeB, largeCards)
	largeOut := largeB.String()

	if len(largeOut) > len(minOut) {
		t.Errorf("overflow pointer scaled with overflow count: "+
			"minimal-overflow output=%d bytes, large-overflow output=%d bytes.\n"+
			"Pointer must be a fixed one-line string, not proportional to the number of overflow cards.",
			len(minOut), len(largeOut))
	}
}

// TestWriteCatalog_OneBelowCap_NoPointer asserts that with maxEnrichedCatalogCards-1
// optional cards (one below the enriched cap), no overflow pointer or enumeration is emitted.
//
// Amplification angle: AC4 tests with exactly maxEnrichedCatalogCards cards (at the boundary).
// This test tightens the negative guard to one below the boundary, catching an off-by-one
// implementation error where the pointer triggers at cap-1 rather than strictly > cap.
func TestWriteCatalog_OneBelowCap_NoPointer(t *testing.T) {
	t.Parallel()
	cards := makeOverflowCards(maxEnrichedCatalogCards - 1)
	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()

	if strings.Contains(out, "phase-inventory.json") {
		t.Errorf("Off-by-one: pointer emitted with %d optional cards (cap=%d, one below cap).\n"+
			"Pointer must only appear when len(optional cards) > maxEnrichedCatalogCards.",
			len(cards), maxEnrichedCatalogCards)
	}
	if strings.Contains(out, "also available") {
		t.Errorf("Off-by-one: overflow enumeration emitted with %d optional cards (cap=%d).\n"+
			"No overflow text must appear when card count is below the cap.",
			len(cards), maxEnrichedCatalogCards)
	}
}
