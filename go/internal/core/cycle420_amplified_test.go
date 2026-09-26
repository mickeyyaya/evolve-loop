package core

import (
	"strings"
	"testing"
)

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
