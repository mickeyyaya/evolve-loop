package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// makeOverflowCards builds n cards that force the overflow path when n > maxEnrichedCatalogCards.
// All cards are Optional with metadata so they sort into the withMeta bucket.
func makeOverflowCards(n int) []router.PhaseCard {
	cards := make([]router.PhaseCard, n)
	for i := range cards {
		cards[i] = router.PhaseCard{
			Name:        fmt.Sprintf("phase-%02d", i),
			Role:        "evaluate",
			Optional:    true,
			Description: "a phase",
			WhenToUse:   "cycle work",
		}
	}
	return cards
}

// TestWriteCatalog_OverflowNotEnumerated asserts that when the catalog exceeds
// maxEnrichedCatalogCards, writeCatalog does NOT emit the bare "- also available
// (<comma-list of names>)" overflow enumeration.
//
// AC1 — router-catalog-dedup-overflow.
//
// RED baseline: current code emits "- also available (" for the overflow bucket.
// Builder must replace it with a pointer line (details in .evolve/phase-inventory.json).
func TestWriteCatalog_OverflowNotEnumerated(t *testing.T) {
	t.Parallel()
	cards := makeOverflowCards(maxEnrichedCatalogCards + 5)
	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()
	if strings.Contains(out, "- also available (") {
		idx := strings.Index(out, "- also available (")
		end := strings.Index(out[idx:], "\n")
		if end < 0 {
			end = len(out[idx:])
		}
		t.Errorf("RED: writeCatalog emitted a bare overflow enumeration.\n"+
			"Line: %q\n"+
			"Builder must replace '- also available (<names>)' with a one-line pointer to phase-inventory.json.", out[idx:idx+end])
	}
}

// TestWriteCatalog_PointerLinePresent asserts that when overflow exists, writeCatalog
// does NOT individually list the overflow phase names. The pointer replaces the name
// enumeration: overflow card names must NOT appear in the output.
//
// AC2 — router-catalog-dedup-overflow.
//
// RED baseline: current code individually lists overflow names in the output
// (e.g. "phase-12, phase-13, phase-14, phase-15, phase-16"). Builder must replace
// the name list with a one-line pointer. The pointer must also reference a lookup
// source (e.g. phase-inventory.json) so the router retains selectability.
func TestWriteCatalog_PointerLinePresent(t *testing.T) {
	t.Parallel()
	cards := makeOverflowCards(maxEnrichedCatalogCards + 5)
	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()
	// Overflow names must NOT be individually listed (pointer replaces name list).
	for i := maxEnrichedCatalogCards; i < len(cards); i++ {
		name := cards[i].Name
		if strings.Contains(out, name) {
			t.Errorf("RED: overflow card %q individually named in output.\n"+
				"Builder must replace the overflow name list with a one-line pointer.\n"+
				"Pointer preserves selectability without enumerating every overflow name.", name)
		}
	}
	// Pointer must reference a lookup source so router can still SELECT overflow phases.
	if !strings.Contains(out, "phase-inventory.json") {
		t.Errorf("RED (secondary): pointer line missing reference to phase-inventory.json.\n"+
			"Builder must include a lookup reference (phase-inventory.json) so router can SELECT overflow phases.\n"+
			"Current output: %q", truncate(out, 400))
	}
}

// TestWriteCatalog_PointerLineRequired_Negative is the adversarial negative test:
// even with just 1 overflow card (the minimal overflow case), the overflow card's
// name must NOT appear individually, AND the output must be longer than with zero
// overflow (proving a pointer was added, not just the enumeration removed silently).
//
// AC3 — router-catalog-dedup-overflow.
//
// RED baseline: current code emits the overflow name individually (contains "phase-12").
// Anti-gaming: catching a builder who removes the enumeration but adds nothing — the
// output would not be longer than the no-overflow case, triggering the second assertion.
func TestWriteCatalog_PointerLineRequired_Negative(t *testing.T) {
	t.Parallel()
	// Minimal overflow: exactly 1 card beyond the cap.
	cards := makeOverflowCards(maxEnrichedCatalogCards + 1)
	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()

	overflowName := cards[maxEnrichedCatalogCards].Name // "phase-12"

	// Negative: overflow card name must NOT appear individually.
	if strings.Contains(out, overflowName) {
		t.Errorf("Negative: with 1 overflow card, %q is individually listed in output.\n"+
			"Even minimal overflow must use a pointer, not an enumeration.", overflowName)
	}

	// Anti-remove-silently: with overflow, output must be LONGER than without overflow.
	// A builder who just removes the enumeration (adding no pointer) produces the same
	// output length as the no-overflow case — this assertion catches that gaming.
	noOverflowCards := makeOverflowCards(maxEnrichedCatalogCards)
	var noB strings.Builder
	writeCatalog(&noB, noOverflowCards)
	if len(out) <= len(noB.String()) {
		t.Errorf("Negative (anti-gaming): output with 1 overflow card (%d bytes) is not longer "+
			"than output with no overflow (%d bytes).\n"+
			"Builder must add a pointer line — do not just remove the enumeration silently.",
			len(out), len(noB.String()))
	}
}

// TestWriteCatalog_NoOverflow_NoPointer is the edge test: when the catalog fits
// within maxEnrichedCatalogCards (no overflow), writeCatalog must not emit either
// the enumeration or the pointer line.
//
// AC4 — router-catalog-dedup-overflow (edge / no-overflow path unchanged).
//
// Pre-existing GREEN: current code also emits nothing extra when no overflow.
// Regression guard: fix must not add a pointer unconditionally to every catalog call.
func TestWriteCatalog_NoOverflow_NoPointer(t *testing.T) {
	t.Parallel()
	// Exactly at the cap — none overflow.
	cards := makeOverflowCards(maxEnrichedCatalogCards)
	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()
	if strings.Contains(out, "also available") {
		t.Errorf("Edge: writeCatalog emitted overflow text when %d cards fit within cap %d.\n"+
			"No overflow → no enumeration and no pointer.", len(cards), maxEnrichedCatalogCards)
	}
	if strings.Contains(out, "phase-inventory.json") {
		t.Errorf("Edge: writeCatalog emitted a pointer when no overflow exists (%d cards ≤ cap %d).\n"+
			"Pointer must only appear when len(cards) > maxEnrichedCatalogCards.", len(cards), maxEnrichedCatalogCards)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
