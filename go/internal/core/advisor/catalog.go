package advisor

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/textcap"
)

// MaxEnrichedCatalogCards bounds how many cards render with full metadata
// (categories + when-to-use hint) so a large plugin ecosystem cannot crowd the
// rubric out of the context window. Overflow phases stay SELECTable via a
// name-only line — a phase absent from the prompt cannot be selected at all.
// Core projects it for the by-name catalog tests.
const MaxEnrichedCatalogCards = 12

// maxCardHintRunes caps a single card's when-to-use hint.
const maxCardHintRunes = 140

// WriteCatalog renders the pre-defined phases the advisor may SELECT (WS3)
// with no on-demand index — WriteCatalogWithOnDemand over a nil index.
func WriteCatalog(b *strings.Builder, cards []router.PhaseCard) {
	WriteCatalogWithOnDemand(b, cards, nil)
}

// WriteCatalogWithOnDemand renders the SELECT menu, biasing toward reuse over
// minting: a selectable phase already has a tuned persona + profile, so
// minting should be the exception (YAGNI for new phases). Cards carry the
// spec's advisor-facing metadata (ADR-0038); relevance judgment is the
// advisor LLM's job — Go only bounds the token cost. When the catalog exceeds
// the enriched cap, Optional (SELECTable) phases take the enriched slots —
// spine phases run via the mandatory config regardless. Deterministic order
// (catalog order, stable partition) ⇒ prompt-prefix-cache friendly. Emits
// nothing when the catalog is empty (legacy built-in-only path). The phases
// that declined a slot are then named in a single line.
//
// The index is the difference between HIDING a phase and REMOVING it. Declining
// exists so 53 never-selected cards stop crowding out 12 enriched slots; if the
// declined set then vanished from the prompt entirely, the advisor could not
// learn those phases exist and the fix would trade one invisibility defect for
// another — the exact class this repo has spent the week removing.
func WriteCatalogWithOnDemand(b *strings.Builder, cards []router.PhaseCard, onDemand []string) {
	if len(cards) == 0 {
		return
	}
	b.WriteString("\n## Pre-defined phases you may SELECT (prefer these over minting)\n")
	b.WriteString("Each already exists with a tuned persona + profile. SELECT one by naming it in your plan ")
	b.WriteString("(no \"mint\" block). Only MINT a new phase when none of these fit the work.\n")

	enriched, overflow := cards, []router.PhaseCard(nil)
	if len(cards) > MaxEnrichedCatalogCards {
		ordered := partitionCards(cards)
		enriched, overflow = ordered[:MaxEnrichedCatalogCards], ordered[MaxEnrichedCatalogCards:]
	}
	for _, c := range enriched {
		writeCard(b, c)
	}
	if len(overflow) > 0 {
		b.WriteString("- All remaining selectable phases are in the \"## Phase Catalog — Core Values\" table (this prompt) and .evolve/phase-inventory.json (same SELECT rules; type/domain details there).\n")
	}
	if len(onDemand) > 0 {
		fmt.Fprintf(b, "\n%d further phase(s) are installed and available ON REQUEST — name one in your plan to use it: %s\n",
			len(onDemand), strings.Join(onDemand, ", "))
	}

}

// partitionCards is the stable three-bucket priority for the enriched slots:
// a SELECTable card with metadata has something to show; a metadata-less
// optional card renders the same either way; spine cards run via the
// mandatory config regardless of rendering.
func partitionCards(cards []router.PhaseCard) []router.PhaseCard {
	var withMeta, opt, rest []router.PhaseCard
	for _, c := range cards {
		switch {
		case c.Optional && (c.WhenToUse != "" || c.Description != "" || len(c.Categories) > 0):
			withMeta = append(withMeta, c)
		case c.Optional:
			opt = append(opt, c)
		default:
			rest = append(rest, c)
		}
	}
	ordered := append(withMeta, opt...)
	return append(ordered, rest...)
}

// writeCard renders one enriched catalog line:
//
//   - bug-reproduction [evaluate] (bugfix) — when: bugfix cycles, before tdd/build
//
// Metadata-less cards degrade to the legacy "- name [role]" form.
func writeCard(b *strings.Builder, c router.PhaseCard) {
	ws := ""
	if c.WritesSource {
		ws = ", writes-source"
	}
	fmt.Fprintf(b, "- %s [%s%s]", c.Name, c.Role, ws)
	if len(c.Categories) > 0 {
		fmt.Fprintf(b, " (%s)", strings.Join(c.Categories, ", "))
	}
	hint := c.WhenToUse
	if hint == "" {
		hint = c.Description
	}
	if hint = textcap.TruncateRunes(hint, maxCardHintRunes); hint != "" {
		fmt.Fprintf(b, " — when: %s", hint)
	}
	b.WriteString("\n")
	// Project this phase's own dispatch guardrails (cycle-436 MR1) so an
	// advisor proposing {cli,tier} for it has the legal bounds in hand instead
	// of guessing blind. Omitted entirely when the phase carries no per-phase
	// guardrail (the common case today).
	if len(c.AllowedCLIs) > 0 {
		fmt.Fprintf(b, "  allowed_clis: %s\n", strings.Join(c.AllowedCLIs, ", "))
	}
	if env := c.ModelTierEnvelope; env != nil {
		fmt.Fprintf(b, "  model_tier_envelope: {min: %s, default: %s, max: %s}\n", env.Min, env.Default, env.Max)
	}
}
